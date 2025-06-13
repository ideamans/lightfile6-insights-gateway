package s3

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/ideamans/lightfile6-insights-gateway/internal/cache"
	"github.com/rs/zerolog/log"
)

// Aggregator handles file aggregation and compression
type Aggregator struct {
	cacheManager *cache.Manager
	s3Client     *Client
}

// NewAggregator creates a new aggregator
func NewAggregator(cacheManager *cache.Manager, s3Client *Client) *Aggregator {
	return &Aggregator{
		cacheManager: cacheManager,
		s3Client:     s3Client,
	}
}

// AggregateAndUpload aggregates files and uploads to S3
func (a *Aggregator) AggregateAndUpload(dataType string) error {
	log.Info().Str("dataType", dataType).Msg("Starting aggregation")

	// Get files to aggregate
	files, err := a.getFilesToAggregate(dataType)
	if err != nil {
		return fmt.Errorf("failed to get files: %w", err)
	}

	if len(files) == 0 {
		log.Debug().Str("dataType", dataType).Msg("No files to aggregate")
		return nil
	}

	log.Info().Str("dataType", dataType).Int("count", len(files)).Msg("Files to aggregate")

	// Move files to aggregation directory
	if err := a.cacheManager.MoveToAggregation(files, dataType); err != nil {
		return fmt.Errorf("failed to move files to aggregation: %w", err)
	}

	// Get files from aggregation directory
	aggregationFiles, err := a.cacheManager.GetAggregationFiles(dataType)
	if err != nil {
		return fmt.Errorf("failed to get aggregation files: %w", err)
	}

	// Sort files by name (timestamp-based)
	sort.Strings(aggregationFiles)

	// Create temporary file for aggregated data
	tempFile, err := a.createTempFile(dataType)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile)

	// Aggregate files
	if err := a.aggregateFiles(aggregationFiles, tempFile); err != nil {
		return fmt.Errorf("failed to aggregate files: %w", err)
	}

	// Move to uploading directory
	uploadingPath, err := a.cacheManager.MoveToUploading(tempFile, dataType)
	if err != nil {
		return fmt.Errorf("failed to move to uploading: %w", err)
	}

	// Upload to S3
	if err := a.s3Client.UploadAggregatedFile(uploadingPath, dataType); err != nil {
		return fmt.Errorf("failed to upload to S3: %w", err)
	}

	// Remove uploaded file
	if err := a.cacheManager.RemoveFile(uploadingPath); err != nil {
		log.Warn().Err(err).Str("file", uploadingPath).Msg("Failed to remove uploaded file")
	}

	// Remove aggregated files
	for _, file := range aggregationFiles {
		if err := a.cacheManager.RemoveFile(file); err != nil {
			log.Warn().Err(err).Str("file", file).Msg("Failed to remove aggregated file")
		}
	}

	log.Info().
		Str("dataType", dataType).
		Int("filesAggregated", len(aggregationFiles)).
		Msg("Aggregation completed")

	return nil
}

// ProcessRemaining processes any remaining files in aggregation/uploading directories
func (a *Aggregator) ProcessRemaining() error {
	dataTypes := []string{"usage", "error"}
	
	for _, dataType := range dataTypes {
		// Process uploading files first
		uploadingFiles, err := a.cacheManager.GetUploadingFiles(dataType)
		if err != nil {
			log.Error().Err(err).Str("dataType", dataType).Msg("Failed to get uploading files")
			continue
		}

		for _, file := range uploadingFiles {
			if err := a.s3Client.UploadAggregatedFile(file, dataType); err != nil {
				log.Error().Err(err).Str("file", file).Msg("Failed to upload remaining file")
				continue
			}
			if err := a.cacheManager.RemoveFile(file); err != nil {
				log.Warn().Err(err).Str("file", file).Msg("Failed to remove uploaded file")
			}
		}

		// Process aggregation files
		if err := a.AggregateAndUpload(dataType); err != nil {
			log.Error().Err(err).Str("dataType", dataType).Msg("Failed to process remaining aggregation")
		}
	}

	// Process remaining specimen files
	specimenFiles, err := a.cacheManager.GetSpecimenFiles()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get specimen files")
		return err
	}

	for _, file := range specimenFiles {
		uri, _, err := a.cacheManager.GetSpecimenInfo(file)
		if err != nil {
			log.Warn().Err(err).Str("file", file).Msg("Failed to parse specimen info")
			continue
		}
		
		// TODO: Need user info for specimen upload
		// For now, use "unknown" as user
		if err := a.s3Client.UploadSpecimen("unknown", uri); err != nil {
			log.Error().Err(err).Str("file", file).Msg("Failed to upload specimen")
		}
	}

	return nil
}

// getFilesToAggregate returns files ready for aggregation
func (a *Aggregator) getFilesToAggregate(dataType string) ([]string, error) {
	switch dataType {
	case "usage":
		return a.cacheManager.GetUsageFiles()
	case "error":
		return a.cacheManager.GetErrorFiles()
	default:
		return nil, fmt.Errorf("unknown data type: %s", dataType)
	}
}

// createTempFile creates a temporary file for aggregation
func (a *Aggregator) createTempFile(dataType string) (string, error) {
	tempDir := filepath.Join(a.cacheManager.BaseDir, dataType, "aggregation")
	tempFile := filepath.Join(tempDir, fmt.Sprintf("aggregate_%d.gz", time.Now().UnixNano()))
	
	file, err := os.Create(tempFile)
	if err != nil {
		return "", err
	}
	file.Close()
	
	return tempFile, nil
}

// aggregateFiles aggregates multiple files into a single gzipped file
func (a *Aggregator) aggregateFiles(files []string, outputPath string) error {
	// Create output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	// Create gzip writer
	gzWriter := gzip.NewWriter(outFile)
	defer gzWriter.Close()

	// Process each file
	for _, filePath := range files {
		if err := a.appendFile(gzWriter, filePath); err != nil {
			return fmt.Errorf("failed to append file %s: %w", filePath, err)
		}
	}

	return nil
}

// appendFile appends a file's content to the gzip writer
func (a *Aggregator) appendFile(gzWriter *gzip.Writer, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Copy file content
	if _, err := io.Copy(gzWriter, file); err != nil {
		return err
	}

	// Add newline if the file doesn't end with one
	if _, err := gzWriter.Write([]byte("\n")); err != nil {
		return err
	}

	return nil
}
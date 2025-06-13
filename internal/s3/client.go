package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/ideamans/lightfile6-insights-gateway/internal/cache"
	"github.com/ideamans/lightfile6-insights-gateway/internal/config"
	"github.com/rs/zerolog/log"
)

// Client handles S3 operations
type Client struct {
	client       *s3.Client
	config       *config.Config
	cacheManager *cache.Manager
}

// NewClient creates a new S3 client
func NewClient(cfg *config.Config) (*Client, error) {
	// Create AWS config
	var awsCfg aws.Config
	var err error

	if cfg.AWS.AccessKeyID != "" && cfg.AWS.SecretAccessKey != "" {
		// Use provided credentials
		creds := credentials.NewStaticCredentialsProvider(
			cfg.AWS.AccessKeyID,
			cfg.AWS.SecretAccessKey,
			"",
		)
		awsCfg, err = awsconfig.LoadDefaultConfig(context.TODO(),
			awsconfig.WithRegion(cfg.AWS.Region),
			awsconfig.WithCredentialsProvider(creds),
		)
	} else {
		// Use default credentials chain
		awsCfg, err = awsconfig.LoadDefaultConfig(context.TODO(),
			awsconfig.WithRegion(cfg.AWS.Region),
		)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client options
	s3Options := func(o *s3.Options) {
		if cfg.AWS.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.AWS.Endpoint)
			o.UsePathStyle = true
		}
	}

	// Create S3 client
	s3Client := s3.NewFromConfig(awsCfg, s3Options)

	return &Client{
		client: s3Client,
		config: cfg,
	}, nil
}

// SetCacheManager sets the cache manager
func (c *Client) SetCacheManager(cm *cache.Manager) {
	c.cacheManager = cm
}

// UploadSpecimen uploads a specimen file immediately
func (c *Client) UploadSpecimen(user, uri string) error {
	if c.cacheManager == nil {
		return fmt.Errorf("cache manager not set")
	}

	// Get specimen files
	files, err := c.cacheManager.GetSpecimenFiles()
	if err != nil {
		return fmt.Errorf("failed to get specimen files: %w", err)
	}

	// Find the file for this URI
	var targetFile string
	for _, file := range files {
		fileURI, _, err := c.cacheManager.GetSpecimenInfo(file)
		if err != nil {
			log.Warn().Err(err).Str("file", file).Msg("Failed to parse specimen info")
			continue
		}
		if fileURI == uri {
			targetFile = file
			break
		}
	}

	if targetFile == "" {
		return fmt.Errorf("specimen file not found for URI: %s", uri)
	}

	// Move to uploading directory
	uploadingPath, err := c.cacheManager.MoveToUploading(targetFile, "specimen")
	if err != nil {
		return fmt.Errorf("failed to move file to uploading: %w", err)
	}

	// Upload file
	if err := c.uploadSpecimenFile(uploadingPath, user, uri); err != nil {
		// Move back to original directory on failure
		// TODO: Implement retry logic
		return err
	}

	// Remove uploaded file
	if err := c.cacheManager.RemoveFile(uploadingPath); err != nil {
		log.Warn().Err(err).Str("file", uploadingPath).Msg("Failed to remove uploaded file")
	}

	return nil
}

// UploadAggregatedFile uploads an aggregated file to S3
func (c *Client) UploadAggregatedFile(filePath string, dataType string) error {
	// Read file
	data, err := c.cacheManager.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Determine bucket and prefix
	var bucket, prefix string
	switch dataType {
	case "usage":
		bucket = c.config.S3.UsageBucket
		prefix = c.config.S3.UsagePrefix
	case "error":
		bucket = c.config.S3.ErrorBucket
		prefix = c.config.S3.ErrorPrefix
	default:
		return fmt.Errorf("unknown data type: %s", dataType)
	}

	// Generate S3 key
	now := time.Now().UTC()
	hostname := getHostname()
	key := fmt.Sprintf("%s%s/%s/%s/%s/%s%s%s%s.%s.jsonl.gz",
		prefix,
		now.Format("2006"),
		now.Format("01"),
		now.Format("02"),
		now.Format("15"),
		now.Format("2006"),
		now.Format("01"),
		now.Format("02"),
		now.Format("15"),
		hostname,
	)

	// Upload to S3
	ctx := context.TODO()
	_, err = c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String("application/gzip"),
	})

	if err != nil {
		return fmt.Errorf("failed to upload to S3: %w", err)
	}

	log.Info().
		Str("bucket", bucket).
		Str("key", key).
		Int("size", len(data)).
		Msg("Uploaded aggregated file to S3")

	return nil
}

// uploadSpecimenFile uploads a single specimen file
func (c *Client) uploadSpecimenFile(filePath, user, uri string) error {
	// Open file
	file, err := c.cacheManager.OpenFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Read file data
	data, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Extract file extension from URI
	ext := extractExtension(uri)
	
	// Get timestamp from filename
	_, timestamp, err := c.cacheManager.GetSpecimenInfo(filePath)
	if err != nil {
		return fmt.Errorf("failed to get specimen info: %w", err)
	}

	// Generate S3 key
	utcTime := timestamp.UTC()
	cleanURI := cleanURIForFilename(uri)
	key := fmt.Sprintf("%s%s/%s/%s/%s/%s.%d%s",
		c.config.S3.SpecimenPrefix,
		user,
		utcTime.Format("2006"),
		utcTime.Format("01"),
		utcTime.Format("02"),
		cleanURI,
		timestamp.UnixNano(),
		ext,
	)

	// Upload to S3 with metadata
	ctx := context.TODO()
	_, err = c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(c.config.S3.SpecimenBucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
		Metadata: map[string]string{
			"uri": uri,
		},
		ContentType: aws.String(detectContentType(ext)),
	})

	if err != nil {
		return fmt.Errorf("failed to upload specimen to S3: %w", err)
	}

	log.Info().
		Str("bucket", c.config.S3.SpecimenBucket).
		Str("key", key).
		Str("user", user).
		Str("uri", uri).
		Int("size", len(data)).
		Msg("Uploaded specimen file to S3")

	return nil
}

// extractExtension extracts file extension from URI
func extractExtension(uri string) string {
	// Parse the URI path
	parts := strings.Split(uri, "/")
	if len(parts) == 0 {
		return ""
	}
	
	filename := parts[len(parts)-1]
	ext := filepath.Ext(filename)
	
	// If no extension, try to guess from common patterns
	if ext == "" {
		if strings.Contains(strings.ToLower(uri), "screenshot") {
			return ".png"
		}
	}
	
	return ext
}

// cleanURIForFilename cleans URI for use in filename
func cleanURIForFilename(uri string) string {
	// Extract filename from URI
	parts := strings.Split(uri, "/")
	filename := parts[len(parts)-1]
	
	// Remove extension
	ext := filepath.Ext(filename)
	if ext != "" {
		filename = strings.TrimSuffix(filename, ext)
	}
	
	// Replace problematic characters
	replacer := strings.NewReplacer(
		" ", "_",
		":", "_",
		"?", "_",
		"&", "_",
		"=", "_",
		"%", "_",
		"#", "_",
	)
	
	return replacer.Replace(filename)
}

// detectContentType detects content type from extension
func detectContentType(ext string) string {
	switch strings.ToLower(ext) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".json":
		return "application/json"
	case ".log", ".txt":
		return "text/plain"
	case ".html":
		return "text/html"
	case ".xml":
		return "application/xml"
	default:
		return "application/octet-stream"
	}
}

// getHostname returns the hostname for S3 key generation
func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	// Clean hostname for S3 key
	return strings.ReplaceAll(hostname, ".", "-")
}

// CheckBuckets verifies that all required buckets exist
func (c *Client) CheckBuckets() error {
	ctx := context.TODO()
	buckets := []string{
		c.config.S3.UsageBucket,
		c.config.S3.ErrorBucket,
		c.config.S3.SpecimenBucket,
	}

	for _, bucket := range buckets {
		_, err := c.client.HeadBucket(ctx, &s3.HeadBucketInput{
			Bucket: aws.String(bucket),
		})
		if err != nil {
			return fmt.Errorf("bucket %s not accessible: %w", bucket, err)
		}
	}

	return nil
}
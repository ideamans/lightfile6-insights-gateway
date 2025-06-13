package cache

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Manager handles cache file operations
type Manager struct {
	BaseDir string
	mu      sync.RWMutex
}

// NewManager creates a new cache manager
func NewManager(baseDir string) *Manager {
	return &Manager{
		BaseDir: baseDir,
	}
}

// Init initializes the cache directory structure
func (m *Manager) Init() error {
	dirs := []string{
		filepath.Join(m.BaseDir, "usage"),
		filepath.Join(m.BaseDir, "usage", "aggregation"),
		filepath.Join(m.BaseDir, "usage", "uploading"),
		filepath.Join(m.BaseDir, "error"),
		filepath.Join(m.BaseDir, "error", "aggregation"),
		filepath.Join(m.BaseDir, "error", "uploading"),
		filepath.Join(m.BaseDir, "specimen"),
		filepath.Join(m.BaseDir, "specimen", "uploading"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

// SaveUsage saves usage data to cache
func (m *Manager) SaveUsage(user string, data []byte) error {
	filename := m.generateFilename(user)
	path := filepath.Join(m.BaseDir, "usage", filename)
	return m.saveFile(path, data)
}

// SaveError saves error data to cache
func (m *Manager) SaveError(user string, data []byte) error {
	filename := m.generateFilename(user)
	path := filepath.Join(m.BaseDir, "error", filename)
	return m.saveFile(path, data)
}

// SaveSpecimen saves specimen data to cache
func (m *Manager) SaveSpecimen(uri string, data []byte) error {
	filename := m.generateSpecimenFilename(uri)
	path := filepath.Join(m.BaseDir, "specimen", filename)
	return m.saveFile(path, data)
}

// GetUsageFiles returns all usage files ready for aggregation
func (m *Manager) GetUsageFiles() ([]string, error) {
	return m.getFiles(filepath.Join(m.BaseDir, "usage"))
}

// GetErrorFiles returns all error files ready for aggregation
func (m *Manager) GetErrorFiles() ([]string, error) {
	return m.getFiles(filepath.Join(m.BaseDir, "error"))
}

// GetSpecimenFiles returns all specimen files ready for upload
func (m *Manager) GetSpecimenFiles() ([]string, error) {
	return m.getFiles(filepath.Join(m.BaseDir, "specimen"))
}

// MoveToAggregation moves files to aggregation directory
func (m *Manager) MoveToAggregation(files []string, dataType string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	aggregationDir := filepath.Join(m.BaseDir, dataType, "aggregation")
	for _, file := range files {
		filename := filepath.Base(file)
		dest := filepath.Join(aggregationDir, filename)
		if err := os.Rename(file, dest); err != nil {
			return fmt.Errorf("failed to move file %s: %w", file, err)
		}
	}
	return nil
}

// MoveToUploading moves files to uploading directory
func (m *Manager) MoveToUploading(file string, dataType string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	uploadingDir := filepath.Join(m.BaseDir, dataType, "uploading")
	filename := filepath.Base(file)
	dest := filepath.Join(uploadingDir, filename)
	if err := os.Rename(file, dest); err != nil {
		return "", fmt.Errorf("failed to move file %s: %w", file, err)
	}
	return dest, nil
}

// RemoveFile removes a file from cache
func (m *Manager) RemoveFile(path string) error {
	return os.Remove(path)
}

// GetAggregationFiles returns files in aggregation directory
func (m *Manager) GetAggregationFiles(dataType string) ([]string, error) {
	return m.getFiles(filepath.Join(m.BaseDir, dataType, "aggregation"))
}

// GetUploadingFiles returns files in uploading directory
func (m *Manager) GetUploadingFiles(dataType string) ([]string, error) {
	return m.getFiles(filepath.Join(m.BaseDir, dataType, "uploading"))
}

// ReadFile reads a cache file
func (m *Manager) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// GetSpecimenInfo extracts user and URI from specimen filename
func (m *Manager) GetSpecimenInfo(filename string) (uri string, timestamp time.Time, err error) {
	// Extract timestamp and encoded URI from filename
	// Format: timestamp.microseconds.pid.encodedURI
	base := filepath.Base(filename)
	
	// Find the first three dots to separate timestamp.microseconds.pid from encodedURI
	dots := []int{}
	for i, ch := range base {
		if ch == '.' {
			dots = append(dots, i)
			if len(dots) == 3 {
				break
			}
		}
	}
	
	if len(dots) < 3 {
		return "", time.Time{}, fmt.Errorf("invalid specimen filename format: %s", base)
	}
	
	timestampStr := base[:dots[1]] // timestamp.microseconds
	encodedURI := base[dots[2]+1:]  // everything after third dot
	
	if len(timestampStr) == 0 || len(encodedURI) == 0 {
		return "", time.Time{}, fmt.Errorf("invalid specimen filename format: %s", base)
	}
	
	// Parse timestamp with microseconds
	timestamp, err = time.Parse("20060102150405.999999", timestampStr)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to parse timestamp: %w", err)
	}
	
	// Decode URI
	uri, err = url.QueryUnescape(encodedURI)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to decode URI: %w", err)
	}
	
	return uri, timestamp, nil
}

// generateFilename generates a filename for usage/error data
func (m *Manager) generateFilename(user string) string {
	timestamp := time.Now().UnixNano()
	pid := os.Getpid()
	return fmt.Sprintf("%d.%d.%s", timestamp, pid, user)
}

// generateSpecimenFilename generates a filename for specimen data
func (m *Manager) generateSpecimenFilename(uri string) string {
	timestamp := time.Now().UTC().Format("20060102150405.999999")
	encodedURI := url.QueryEscape(uri)
	return fmt.Sprintf("%s.%d.%s", timestamp, os.Getpid(), encodedURI)
}

// saveFile saves data to a file
func (m *Manager) saveFile(path string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// getFiles returns all files in a directory
func (m *Manager) getFiles(dir string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}

	return files, nil
}

// OpenFile opens a file for reading
func (m *Manager) OpenFile(path string) (io.ReadCloser, error) {
	return os.Open(path)
}
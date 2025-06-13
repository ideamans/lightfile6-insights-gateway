package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager_Init(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager(tempDir)

	err := manager.Init()
	require.NoError(t, err)

	// Check all directories are created
	expectedDirs := []string{
		"usage",
		"usage/aggregation",
		"usage/uploading",
		"error",
		"error/aggregation",
		"error/uploading",
		"specimen",
		"specimen/uploading",
	}

	for _, dir := range expectedDirs {
		path := filepath.Join(tempDir, dir)
		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	}
}

func TestManager_SaveAndGetFiles(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager(tempDir)
	require.NoError(t, manager.Init())

	t.Run("SaveUsage", func(t *testing.T) {
		data := []byte(`{"event": "test"}`)
		err := manager.SaveUsage("testuser", data)
		require.NoError(t, err)

		files, err := manager.GetUsageFiles()
		require.NoError(t, err)
		assert.Len(t, files, 1)

		// Read the file back
		content, err := manager.ReadFile(files[0])
		require.NoError(t, err)
		assert.Equal(t, data, content)
	})

	t.Run("SaveError", func(t *testing.T) {
		data := []byte(`{"error": "test error"}`)
		err := manager.SaveError("testuser", data)
		require.NoError(t, err)

		files, err := manager.GetErrorFiles()
		require.NoError(t, err)
		assert.Len(t, files, 1)
	})

	t.Run("SaveSpecimen", func(t *testing.T) {
		data := []byte("specimen data")
		uri := "http://example.com/test.png"
		err := manager.SaveSpecimen(uri, data)
		require.NoError(t, err)

		files, err := manager.GetSpecimenFiles()
		require.NoError(t, err)
		assert.Len(t, files, 1)

		// Test GetSpecimenInfo
		parsedURI, timestamp, err := manager.GetSpecimenInfo(files[0])
		require.NoError(t, err)
		assert.Equal(t, uri, parsedURI)
		assert.WithinDuration(t, time.Now().UTC(), timestamp, 10*time.Second)
	})
}

func TestManager_MoveOperations(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager(tempDir)
	require.NoError(t, manager.Init())

	// Create test file
	data := []byte(`{"event": "test"}`)
	err := manager.SaveUsage("testuser", data)
	require.NoError(t, err)

	files, err := manager.GetUsageFiles()
	require.NoError(t, err)
	require.Len(t, files, 1)

	t.Run("MoveToAggregation", func(t *testing.T) {
		err := manager.MoveToAggregation(files, "usage")
		require.NoError(t, err)

		// Check file moved
		usageFiles, err := manager.GetUsageFiles()
		require.NoError(t, err)
		assert.Len(t, usageFiles, 0)

		aggregationFiles, err := manager.GetAggregationFiles("usage")
		require.NoError(t, err)
		assert.Len(t, aggregationFiles, 1)
	})

	t.Run("MoveToUploading", func(t *testing.T) {
		aggregationFiles, err := manager.GetAggregationFiles("usage")
		require.NoError(t, err)
		require.Len(t, aggregationFiles, 1)

		newPath, err := manager.MoveToUploading(aggregationFiles[0], "usage")
		require.NoError(t, err)
		assert.Contains(t, newPath, "uploading")

		uploadingFiles, err := manager.GetUploadingFiles("usage")
		require.NoError(t, err)
		assert.Len(t, uploadingFiles, 1)
	})
}

func TestManager_generateFilename(t *testing.T) {
	manager := NewManager("/tmp")

	t.Run("generateFilename", func(t *testing.T) {
		filename := manager.generateFilename("testuser")
		assert.Contains(t, filename, "testuser")
		assert.Regexp(t, `^\d+\.\d+\.testuser$`, filename)
	})

	t.Run("generateSpecimenFilename", func(t *testing.T) {
		uri := "http://example.com/test file.png"
		filename := manager.generateSpecimenFilename(uri)
		assert.Contains(t, filename, "http%3A%2F%2Fexample.com%2Ftest+file.png")
		assert.Regexp(t, `^\d{14}\.\d{6}\.\d+\..*$`, filename)
	})
}

func TestManager_GetSpecimenInfo(t *testing.T) {
	manager := NewManager("/tmp")

	tests := []struct {
		name          string
		filename      string
		expectedURI   string
		expectError   bool
	}{
		{
			name:        "valid filename",
			filename:    "20240101123045.123456.12345.http%3A%2F%2Fexample.com%2Ftest.png",
			expectedURI: "http://example.com/test.png",
			expectError: false,
		},
		{
			name:        "filename with spaces",
			filename:    "20240101123045.123456.12345.http%3A%2F%2Fexample.com%2Ftest+file.png",
			expectedURI: "http://example.com/test file.png",
			expectError: false,
		},
		{
			name:        "invalid format",
			filename:    "invalid_filename",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uri, timestamp, err := manager.GetSpecimenInfo(tt.filename)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedURI, uri)
				assert.NotZero(t, timestamp)
			}
		})
	}
}
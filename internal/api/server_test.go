package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ideamans/lightfile6-insights-gateway/internal/cache"
	"github.com/ideamans/lightfile6-insights-gateway/internal/config"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockS3Client is a mock implementation of S3 client
type MockS3Client struct {
	uploadedSpecimens []struct {
		User string
		URI  string
	}
}

func (m *MockS3Client) UploadSpecimen(user, uri string) error {
	m.uploadedSpecimens = append(m.uploadedSpecimens, struct {
		User string
		URI  string
	}{User: user, URI: uri})
	return nil
}

func (m *MockS3Client) UploadAggregatedFile(filePath string, dataType string) error {
	return nil
}

func (m *MockS3Client) CheckBuckets() error {
	return nil
}

func TestServer_Health(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	cacheManager := cache.NewManager(tempDir)
	require.NoError(t, cacheManager.Init())

	cfg := &config.Config{
		S3: config.S3Config{
			UsageBucket:    "test-usage",
			ErrorBucket:    "test-error",
			SpecimenBucket: "test-specimen",
		},
	}

	server := NewServer(8080, cacheManager, nil, cfg)

	// Test
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	server.echo.ServeHTTP(rec, req)

	// Assert
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"healthy"`)
}

func TestServer_AuthMiddleware(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	cacheManager := cache.NewManager(tempDir)
	require.NoError(t, cacheManager.Init())

	cfg := &config.Config{
		S3: config.S3Config{
			UsageBucket:    "test-usage",
			ErrorBucket:    "test-error",
			SpecimenBucket: "test-specimen",
		},
	}

	server := NewServer(8080, cacheManager, nil, cfg)

	tests := []struct {
		name       string
		token      string
		wantStatus int
	}{
		{
			name:       "with token",
			token:      "testuser",
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "without token",
			token:      "",
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/usage", bytes.NewReader([]byte(`{"test": "data"}`)))
			if tt.token != "" {
				req.Header.Set("USER_TOKEN", tt.token)
			}
			rec := httptest.NewRecorder()
			server.echo.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}

func TestServer_HandleUsage(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	cacheManager := cache.NewManager(tempDir)
	require.NoError(t, cacheManager.Init())

	cfg := &config.Config{
		S3: config.S3Config{
			UsageBucket:    "test-usage",
			ErrorBucket:    "test-error",
			SpecimenBucket: "test-specimen",
		},
	}

	server := NewServer(8080, cacheManager, nil, cfg)

	// Test
	data := []byte(`{"event": "test", "timestamp": "2024-01-01T00:00:00Z"}`)
	req := httptest.NewRequest(http.MethodPut, "/usage", bytes.NewReader(data))
	req.Header.Set("USER_TOKEN", "testuser")
	rec := httptest.NewRecorder()
	server.echo.ServeHTTP(rec, req)

	// Assert
	assert.Equal(t, http.StatusNoContent, rec.Code)

	// Verify data was saved
	files, err := cacheManager.GetUsageFiles()
	require.NoError(t, err)
	assert.Len(t, files, 1)

	content, err := cacheManager.ReadFile(files[0])
	require.NoError(t, err)
	assert.Equal(t, data, content)
}

func TestServer_HandleError(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	cacheManager := cache.NewManager(tempDir)
	require.NoError(t, cacheManager.Init())

	cfg := &config.Config{
		S3: config.S3Config{
			UsageBucket:    "test-usage",
			ErrorBucket:    "test-error",
			SpecimenBucket: "test-specimen",
		},
	}

	server := NewServer(8080, cacheManager, nil, cfg)

	// Test
	data := []byte(`{"error": "test error", "timestamp": "2024-01-01T00:00:00Z"}`)
	req := httptest.NewRequest(http.MethodPut, "/error", bytes.NewReader(data))
	req.Header.Set("USER_TOKEN", "testuser")
	rec := httptest.NewRecorder()
	server.echo.ServeHTTP(rec, req)

	// Assert
	assert.Equal(t, http.StatusNoContent, rec.Code)

	// Verify data was saved
	files, err := cacheManager.GetErrorFiles()
	require.NoError(t, err)
	assert.Len(t, files, 1)
}

func TestServer_HandleSpecimen(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	cacheManager := cache.NewManager(tempDir)
	require.NoError(t, cacheManager.Init())

	cfg := &config.Config{
		S3: config.S3Config{
			UsageBucket:    "test-usage",
			ErrorBucket:    "test-error",
			SpecimenBucket: "test-specimen",
		},
	}

	// Note: For testing, we're not actually using the mock S3 client
	// The actual upload happens asynchronously, so we'll just verify the file is saved
	server := NewServer(8080, cacheManager, nil, cfg)

	tests := []struct {
		name       string
		uri        string
		wantStatus int
	}{
		{
			name:       "with uri",
			uri:        "http://example.com/test.png",
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "without uri",
			uri:        "",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := []byte("specimen data")
			path := "/specimen"
			if tt.uri != "" {
				path += "?uri=" + tt.uri
			}
			req := httptest.NewRequest(http.MethodPut, path, bytes.NewReader(data))
			req.Header.Set("USER_TOKEN", "testuser")
			rec := httptest.NewRecorder()
			server.echo.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)

			if tt.wantStatus == http.StatusNoContent {
				// Verify data was saved
				files, err := cacheManager.GetSpecimenFiles()
				require.NoError(t, err)
				assert.Greater(t, len(files), 0)
			}
		})
	}
}

func TestReadRequestBody(t *testing.T) {
	data := []byte("test data")
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(data))
	
	e := echo.New()
	c := e.NewContext(req, httptest.NewRecorder())
	
	result, err := readRequestBody(c)
	require.NoError(t, err)
	assert.Equal(t, data, result)
}

// Helper function to create a test server
func setupTestServer(t *testing.T) (*Server, *cache.Manager, *MockS3Client) {
	tempDir := t.TempDir()
	cacheManager := cache.NewManager(tempDir)
	require.NoError(t, cacheManager.Init())

	cfg := &config.Config{
		S3: config.S3Config{
			UsageBucket:    "test-usage",
			ErrorBucket:    "test-error",
			SpecimenBucket: "test-specimen",
		},
	}

	mockS3 := &MockS3Client{}
	// Note: In the actual implementation, we'd need to modify the server
	// to accept an interface instead of the concrete s3.Client type
	// For now, we'll test without the S3 client

	server := NewServer(8080, cacheManager, nil, cfg)
	return server, cacheManager, mockS3
}
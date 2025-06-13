//go:build integration
// +build integration

package integration

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	minioEndpoint   string
	minioAccessKey  = "minioadmin"
	minioSecretKey  = "minioadmin"
	gatewayEndpoint string
	gatewayCmd      *exec.Cmd
	s3Client        *s3.Client
)

func TestMain(m *testing.M) {
	// Setup
	pool, err := dockertest.NewPool("")
	if err != nil {
		panic(fmt.Sprintf("Could not connect to docker: %s", err))
	}

	// Start MinIO
	minioResource, err := startMinIO(pool)
	if err != nil {
		panic(fmt.Sprintf("Could not start MinIO: %s", err))
	}

	// Build gateway binary
	if err := buildGateway(); err != nil {
		panic(fmt.Sprintf("Could not build gateway: %s", err))
	}

	// Create S3 client
	s3Client, err = createS3Client()
	if err != nil {
		panic(fmt.Sprintf("Could not create S3 client: %s", err))
	}

	// Create buckets
	if err := createBuckets(); err != nil {
		panic(fmt.Sprintf("Could not create buckets: %s", err))
	}

	// Write config file
	configPath, err := writeConfig()
	if err != nil {
		panic(fmt.Sprintf("Could not write config: %s", err))
	}

	// Start gateway
	if err := startGateway(configPath); err != nil {
		panic(fmt.Sprintf("Could not start gateway: %s", err))
	}

	// Wait for gateway to be ready
	if err := waitForGateway(); err != nil {
		panic(fmt.Sprintf("Gateway not ready: %s", err))
	}

	// Run tests
	code := m.Run()

	// Cleanup
	if gatewayCmd != nil && gatewayCmd.Process != nil {
		gatewayCmd.Process.Kill()
		gatewayCmd.Wait()
	}
	
	if minioResource != nil {
		if err := pool.Purge(minioResource); err != nil {
			fmt.Printf("Could not purge MinIO: %s\n", err)
		}
	}

	os.Exit(code)
}

func startMinIO(pool *dockertest.Pool) (*dockertest.Resource, error) {
	// Check if running in Docker environment
	if endpoint := os.Getenv("MINIO_ENDPOINT"); endpoint != "" {
		minioEndpoint = endpoint
		minioAccessKey = os.Getenv("MINIO_ACCESS_KEY")
		minioSecretKey = os.Getenv("MINIO_SECRET_KEY")
		return nil, nil // Skip local MinIO startup
	}

	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "minio/minio",
		Tag:        "latest",
		Env: []string{
			fmt.Sprintf("MINIO_ROOT_USER=%s", minioAccessKey),
			fmt.Sprintf("MINIO_ROOT_PASSWORD=%s", minioSecretKey),
		},
		Cmd: []string{"server", "/data"},
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		return nil, err
	}

	minioEndpoint = fmt.Sprintf("http://localhost:%s", resource.GetPort("9000/tcp"))

	// Wait for MinIO to be ready
	pool.MaxWait = 60 * time.Second
	if err := pool.Retry(func() error {
		resp, err := http.Get(minioEndpoint + "/minio/health/live")
		if err != nil {
			return err
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("MinIO not ready, status: %d", resp.StatusCode)
		}
		return nil
	}); err != nil {
		return resource, err
	}

	return resource, nil
}

func buildGateway() error {
	cmd := exec.Command("go", "build", "-o", "../../gateway", "../../cmd/gateway")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func createS3Client() (*s3.Client, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			minioAccessKey,
			minioSecretKey,
			"",
		)),
	)
	if err != nil {
		return nil, err
	}

	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(minioEndpoint)
		o.UsePathStyle = true
	}), nil
}

func createBuckets() error {
	buckets := []string{"test-usage", "test-error", "test-specimen"}
	
	for _, bucket := range buckets {
		_, err := s3Client.CreateBucket(context.TODO(), &s3.CreateBucketInput{
			Bucket: aws.String(bucket),
		})
		if err != nil {
			return fmt.Errorf("failed to create bucket %s: %w", bucket, err)
		}
	}
	
	return nil
}

func writeConfig() (string, error) {
	configDir := filepath.Join(os.TempDir(), "lightfile6-test")
	os.MkdirAll(configDir, 0755)
	
	configPath := filepath.Join(configDir, "config.yml")
	config := fmt.Sprintf(`
cache_dir: %s/cache

aws:
  region: us-east-1
  access_key_id: %s
  secret_access_key: %s
  endpoint: %s

s3:
  usage_bucket: test-usage
  error_bucket: test-error
  specimen_bucket: test-specimen

aggregation:
  usage_interval: 2s
  error_interval: 2s
`, configDir, minioAccessKey, minioSecretKey, minioEndpoint)

	return configPath, os.WriteFile(configPath, []byte(config), 0644)
}

func startGateway(configPath string) error {
	gatewayCmd = exec.Command("../../gateway", "-p", "8888", "-c", configPath)
	gatewayCmd.Stdout = os.Stdout
	gatewayCmd.Stderr = os.Stderr
	
	if err := gatewayCmd.Start(); err != nil {
		return err
	}
	
	gatewayEndpoint = "http://localhost:8888"
	return nil
}

func waitForGateway() error {
	for i := 0; i < 30; i++ {
		resp, err := http.Get(gatewayEndpoint + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("gateway did not become ready")
}

func TestHealthEndpoint(t *testing.T) {
	resp, err := http.Get(gatewayEndpoint + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()
	
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	
	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "healthy", result["status"])
}

func TestUsageEndpoint(t *testing.T) {
	// Send usage data
	data := map[string]interface{}{
		"event":     "test_event",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	
	body, _ := json.Marshal(data)
	req, err := http.NewRequest("PUT", gatewayEndpoint+"/usage", bytes.NewReader(body))
	require.NoError(t, err)
	
	req.Header.Set("USER_TOKEN", "testuser")
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	
	// Wait for aggregation
	time.Sleep(3 * time.Second)
	
	// Check S3 for aggregated file
	listResp, err := s3Client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket: aws.String("test-usage"),
	})
	require.NoError(t, err)
	
	assert.Greater(t, len(listResp.Contents), 0, "Expected at least one file in S3")
	
	// Download and verify content
	for _, obj := range listResp.Contents {
		getResp, err := s3Client.GetObject(context.TODO(), &s3.GetObjectInput{
			Bucket: aws.String("test-usage"),
			Key:    obj.Key,
		})
		require.NoError(t, err)
		
		// Decompress and verify
		gzReader, err := gzip.NewReader(getResp.Body)
		require.NoError(t, err)
		
		content, err := io.ReadAll(gzReader)
		require.NoError(t, err)
		
		gzReader.Close()
		getResp.Body.Close()
		
		assert.Contains(t, string(content), "test_event")
	}
}

func TestErrorEndpoint(t *testing.T) {
	// Send error data
	data := map[string]interface{}{
		"error":     "test_error",
		"message":   "This is a test error",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	
	body, _ := json.Marshal(data)
	req, err := http.NewRequest("PUT", gatewayEndpoint+"/error", bytes.NewReader(body))
	require.NoError(t, err)
	
	req.Header.Set("USER_TOKEN", "testuser")
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestSpecimenEndpoint(t *testing.T) {
	// Send specimen data
	data := []byte("This is a test specimen file content")
	uri := "http://example.com/test.txt"
	
	req, err := http.NewRequest("PUT", gatewayEndpoint+"/specimen?uri="+uri, bytes.NewReader(data))
	require.NoError(t, err)
	
	req.Header.Set("USER_TOKEN", "testuser")
	req.Header.Set("Content-Type", "text/plain")
	
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	
	// Wait a bit for upload
	time.Sleep(2 * time.Second)
	
	// Check S3 for specimen file
	listResp, err := s3Client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket: aws.String("test-specimen"),
		Prefix: aws.String("testuser/"),
	})
	require.NoError(t, err)
	
	assert.Greater(t, len(listResp.Contents), 0, "Expected at least one specimen file in S3")
	
	// Verify content
	for _, obj := range listResp.Contents {
		getResp, err := s3Client.GetObject(context.TODO(), &s3.GetObjectInput{
			Bucket: aws.String("test-specimen"),
			Key:    obj.Key,
		})
		require.NoError(t, err)
		
		content, err := io.ReadAll(getResp.Body)
		require.NoError(t, err)
		getResp.Body.Close()
		
		assert.Equal(t, data, content)
	}
}

func TestAuthenticationRequired(t *testing.T) {
	endpoints := []string{"/usage", "/error", "/specimen?uri=test"}
	
	for _, endpoint := range endpoints {
		req, err := http.NewRequest("PUT", gatewayEndpoint+endpoint, strings.NewReader("test"))
		require.NoError(t, err)
		
		// No USER_TOKEN header
		
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	}
}

func TestGracefulShutdown(t *testing.T) {
	// Skip if running in CI without proper process management
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping graceful shutdown test in CI")
	}
	
	// Send some data that won't be aggregated immediately
	for i := 0; i < 5; i++ {
		data := map[string]interface{}{
			"event":     fmt.Sprintf("shutdown_test_%d", i),
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		
		body, _ := json.Marshal(data)
		req, err := http.NewRequest("PUT", gatewayEndpoint+"/usage", bytes.NewReader(body))
		require.NoError(t, err)
		
		req.Header.Set("USER_TOKEN", "shutdownuser")
		req.Header.Set("Content-Type", "application/json")
		
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		resp.Body.Close()
	}
	
	// Send shutdown signal
	if gatewayCmd != nil && gatewayCmd.Process != nil {
		gatewayCmd.Process.Signal(os.Interrupt)
		
		// Wait for process to exit
		done := make(chan error, 1)
		go func() {
			done <- gatewayCmd.Wait()
		}()
		
		select {
		case <-done:
			// Process exited
		case <-time.After(10 * time.Second):
			t.Fatal("Gateway did not shut down gracefully")
		}
		
		// Check that data was uploaded
		time.Sleep(2 * time.Second)
		
		listResp, err := s3Client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
			Bucket: aws.String("test-usage"),
		})
		require.NoError(t, err)
		
		// Should have files from shutdown processing
		assert.Greater(t, len(listResp.Contents), 0, "Expected files to be uploaded during shutdown")
	}
}
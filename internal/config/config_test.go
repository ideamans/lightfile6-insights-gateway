package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConfig_SetDefaults(t *testing.T) {
	tests := []struct {
		name     string
		input    Config
		expected Config
	}{
		{
			name:  "empty config",
			input: Config{},
			expected: Config{
				CacheDir: "/var/lib/lightfile6-insights-gateway",
				AWS: AWSConfig{
					Region: "ap-northeast-1",
				},
				Aggregation: AggregationConfig{
					UsageInterval: 10 * time.Minute,
					ErrorInterval: 10 * time.Minute,
				},
			},
		},
		{
			name: "partial config",
			input: Config{
				CacheDir: "/custom/path",
				AWS: AWSConfig{
					Region: "us-west-2",
				},
			},
			expected: Config{
				CacheDir: "/custom/path",
				AWS: AWSConfig{
					Region: "us-west-2",
				},
				Aggregation: AggregationConfig{
					UsageInterval: 10 * time.Minute,
					ErrorInterval: 10 * time.Minute,
				},
			},
		},
		{
			name: "custom intervals",
			input: Config{
				Aggregation: AggregationConfig{
					UsageInterval: 5 * time.Minute,
				},
			},
			expected: Config{
				CacheDir: "/var/lib/lightfile6-insights-gateway",
				AWS: AWSConfig{
					Region: "ap-northeast-1",
				},
				Aggregation: AggregationConfig{
					UsageInterval: 5 * time.Minute,
					ErrorInterval: 10 * time.Minute,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.input
			cfg.SetDefaults()
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr error
	}{
		{
			name: "valid config",
			config: Config{
				S3: S3Config{
					UsageBucket:    "usage-bucket",
					ErrorBucket:    "error-bucket",
					SpecimenBucket: "specimen-bucket",
				},
			},
			wantErr: nil,
		},
		{
			name: "missing usage bucket",
			config: Config{
				S3: S3Config{
					ErrorBucket:    "error-bucket",
					SpecimenBucket: "specimen-bucket",
				},
			},
			wantErr: ErrUsageBucketRequired,
		},
		{
			name: "missing error bucket",
			config: Config{
				S3: S3Config{
					UsageBucket:    "usage-bucket",
					SpecimenBucket: "specimen-bucket",
				},
			},
			wantErr: ErrErrorBucketRequired,
		},
		{
			name: "missing specimen bucket",
			config: Config{
				S3: S3Config{
					UsageBucket: "usage-bucket",
					ErrorBucket: "error-bucket",
				},
			},
			wantErr: ErrSpecimenBucketRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr != nil {
				assert.Equal(t, tt.wantErr, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
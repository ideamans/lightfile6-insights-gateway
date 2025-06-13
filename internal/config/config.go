package config

import (
	"time"
)

// Config holds the application configuration
type Config struct {
	// Cache directory
	CacheDir string `mapstructure:"cache_dir"`

	// AWS configuration
	AWS AWSConfig `mapstructure:"aws"`

	// S3 bucket configuration
	S3 S3Config `mapstructure:"s3"`

	// Aggregation intervals
	Aggregation AggregationConfig `mapstructure:"aggregation"`
}

// AWSConfig holds AWS specific configuration
type AWSConfig struct {
	Region          string `mapstructure:"region"`
	AccessKeyID     string `mapstructure:"access_key_id"`
	SecretAccessKey string `mapstructure:"secret_access_key"`
	Endpoint        string `mapstructure:"endpoint"`
}

// S3Config holds S3 bucket configuration
type S3Config struct {
	UsageBucket      string `mapstructure:"usage_bucket"`
	UsagePrefix      string `mapstructure:"usage_prefix"`
	ErrorBucket      string `mapstructure:"error_bucket"`
	ErrorPrefix      string `mapstructure:"error_prefix"`
	SpecimenBucket   string `mapstructure:"specimen_bucket"`
	SpecimenPrefix   string `mapstructure:"specimen_prefix"`
}

// AggregationConfig holds aggregation intervals
type AggregationConfig struct {
	UsageInterval time.Duration `mapstructure:"usage_interval"`
	ErrorInterval time.Duration `mapstructure:"error_interval"`
}

// SetDefaults sets default values for configuration
func (c *Config) SetDefaults() {
	if c.CacheDir == "" {
		c.CacheDir = "/var/lib/lightfile6-insights-gateway"
	}
	if c.AWS.Region == "" {
		c.AWS.Region = "ap-northeast-1"
	}
	if c.Aggregation.UsageInterval == 0 {
		c.Aggregation.UsageInterval = 10 * time.Minute
	}
	if c.Aggregation.ErrorInterval == 0 {
		c.Aggregation.ErrorInterval = 10 * time.Minute
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.S3.UsageBucket == "" {
		return ErrUsageBucketRequired
	}
	if c.S3.ErrorBucket == "" {
		return ErrErrorBucketRequired
	}
	if c.S3.SpecimenBucket == "" {
		return ErrSpecimenBucketRequired
	}
	return nil
}
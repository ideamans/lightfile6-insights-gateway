package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Load loads configuration from file
func Load(path string) (*Config, error) {
	v := viper.New()
	
	// Set config file
	v.SetConfigFile(path)
	
	// Set defaults
	v.SetDefault("cache_dir", "/var/lib/lightfile6-insights-gateway")
	v.SetDefault("aws.region", "ap-northeast-1")
	v.SetDefault("aggregation.usage_interval", "10m")
	v.SetDefault("aggregation.error_interval", "10m")
	
	// Enable environment variable support
	v.SetEnvPrefix("LIGHTFILE6")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	
	// Read config file
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	
	// Unmarshal to struct
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	
	// Parse duration strings
	if intervalStr := v.GetString("aggregation.usage_interval"); intervalStr != "" {
		duration, err := time.ParseDuration(intervalStr)
		if err != nil {
			return nil, fmt.Errorf("invalid usage_interval: %w", err)
		}
		cfg.Aggregation.UsageInterval = duration
	}
	
	if intervalStr := v.GetString("aggregation.error_interval"); intervalStr != "" {
		duration, err := time.ParseDuration(intervalStr)
		if err != nil {
			return nil, fmt.Errorf("invalid error_interval: %w", err)
		}
		cfg.Aggregation.ErrorInterval = duration
	}
	
	// Set defaults
	cfg.SetDefaults()
	
	// Validate
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	
	return &cfg, nil
}
package config

import "errors"

// Configuration errors
var (
	ErrUsageBucketRequired    = errors.New("usage bucket is required")
	ErrErrorBucketRequired    = errors.New("error bucket is required")
	ErrSpecimenBucketRequired = errors.New("specimen bucket is required")
)
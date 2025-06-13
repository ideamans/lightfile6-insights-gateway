//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// waitForS3Object waits for an object to appear in S3
func waitForS3Object(bucket, prefix string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for S3 object in bucket %s with prefix %s", bucket, prefix)
		case <-ticker.C:
			listResp, err := s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
				Bucket: aws.String(bucket),
				Prefix: aws.String(prefix),
			})
			if err != nil {
				continue
			}
			
			if len(listResp.Contents) > 0 {
				return nil
			}
		}
	}
}

// cleanupBucket removes all objects from a bucket
func cleanupBucket(bucket string) error {
	ctx := context.Background()
	
	// List all objects
	listResp, err := s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		return err
	}
	
	// Delete each object
	for _, obj := range listResp.Contents {
		_, err := s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(bucket),
			Key:    obj.Key,
		})
		if err != nil {
			return err
		}
	}
	
	return nil
}
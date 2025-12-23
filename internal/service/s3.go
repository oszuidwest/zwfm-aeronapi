// Package service provides business logic for the Aeron Toolbox.
package service

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/oszuidwest/zwfm-aerontoolbox/internal/config"
	"github.com/oszuidwest/zwfm-aerontoolbox/internal/types"
)

// s3Service handles S3 synchronization for backup files.
// This is an internal storage backend for BackupService.
type s3Service struct {
	uploader *manager.Uploader
	client   *s3.Client
	bucket   string
	prefix   string
}

// newS3Service creates a new s3Service if S3 is enabled in config.
// Returns nil if S3 is disabled.
func newS3Service(cfg *config.S3Config) (*s3Service, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	client := s3.New(s3.Options{
		Region:       cfg.Region,
		BaseEndpoint: ptrOrNil(cfg.Endpoint),
		UsePathStyle: cfg.ForcePathStyle,
		Credentials: credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			"",
		),
	})

	slog.Info("S3 synchronisatie ingeschakeld",
		"bucket", cfg.Bucket,
		"region", cfg.Region,
		"endpoint", cfg.Endpoint,
		"prefix", cfg.GetPathPrefix())

	return &s3Service{
		uploader: manager.NewUploader(client),
		client:   client,
		bucket:   cfg.Bucket,
		prefix:   cfg.GetPathPrefix(),
	}, nil
}

func ptrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return aws.String(s)
}

// upload uploads a local file to S3.
func (s *s3Service) upload(ctx context.Context, filename, localPath string) (err error) {
	file, err := os.Open(localPath)
	if err != nil {
		return types.NewOperationError("S3 upload", fmt.Errorf("bestand openen: %w", err))
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = types.NewOperationError("S3 upload", fmt.Errorf("bestand sluiten: %w", closeErr))
		}
	}()

	key := s.prefix + filename
	start := time.Now()

	_, err = s.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   file,
	})
	if err != nil {
		return types.NewOperationError("S3 upload", err)
	}

	slog.Info("Backup naar S3 geüpload",
		"key", key,
		"duration", time.Since(start).Round(time.Millisecond))

	return nil
}

// delete removes a file from S3.
func (s *s3Service) delete(ctx context.Context, filename string) error {
	key := s.prefix + filename

	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return types.NewOperationError("S3 delete", err)
	}

	slog.Info("Backup van S3 verwijderd", "key", key)
	return nil
}

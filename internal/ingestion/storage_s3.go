package ingestion

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Config holds configuration for the S3 storage backend.
type S3Config struct {
	Bucket    string
	Region    string
	Endpoint  string
	AccessKey string
	SecretKey string
}

// S3Storage implements StorageClient using AWS S3 (or S3-compatible stores like MinIO).
type S3Storage struct {
	client *s3.Client
	bucket string
}

// NewS3Storage creates an S3-backed StorageClient.
func NewS3Storage(ctx context.Context, cfg S3Config) (*S3Storage, error) {
	var opts []func(*awsconfig.LoadOptions) error
	if cfg.Region != "" {
		opts = append(opts, awsconfig.WithRegion(cfg.Region))
	}
	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	var s3Opts []func(*s3.Options)
	if cfg.Endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true
		})
	}

	client := s3.NewFromConfig(awsCfg, s3Opts...)
	return &S3Storage{client: client, bucket: cfg.Bucket}, nil
}

func (s *S3Storage) key(tenantID, kind, id string) string {
	return tenantID + "/" + kind + "/" + id + ".json"
}

func (s *S3Storage) put(ctx context.Context, key string, data []byte) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String("application/json"),
	})
	if err != nil {
		return fmt.Errorf("s3 put %s: %w", key, err)
	}
	return nil
}

func (s *S3Storage) get(ctx context.Context, key string) ([]byte, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("s3 get %s: %w", key, err)
	}
	defer out.Body.Close()
	return io.ReadAll(out.Body)
}

func (s *S3Storage) PutSnapshot(ctx context.Context, tenantID, snapshotID string, data []byte) error {
	return s.put(ctx, s.key(tenantID, "snapshots", snapshotID), data)
}

func (s *S3Storage) GetSnapshot(ctx context.Context, tenantID, snapshotID string) ([]byte, error) {
	return s.get(ctx, s.key(tenantID, "snapshots", snapshotID))
}

func (s *S3Storage) PutDelta(ctx context.Context, tenantID, deltaID string, data []byte) error {
	return s.put(ctx, s.key(tenantID, "deltas", deltaID), data)
}

func (s *S3Storage) GetDelta(ctx context.Context, tenantID, deltaID string) ([]byte, error) {
	return s.get(ctx, s.key(tenantID, "deltas", deltaID))
}

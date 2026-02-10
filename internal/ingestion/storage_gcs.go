package ingestion

import (
	"context"
	"fmt"
	"io"

	gcs "cloud.google.com/go/storage"
)

// GCSStorage implements StorageClient using Google Cloud Storage.
type GCSStorage struct {
	client *gcs.Client
	bucket string
}

// NewGCSStorage creates a GCS-backed StorageClient.
// It uses Application Default Credentials (works with Workload Identity, SA keys, gcloud auth).
func NewGCSStorage(ctx context.Context, bucket string) (*GCSStorage, error) {
	client, err := gcs.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("create gcs client: %w", err)
	}
	return &GCSStorage{client: client, bucket: bucket}, nil
}

func (s *GCSStorage) key(tenantID, kind, id string) string {
	return tenantID + "/" + kind + "/" + id + ".json"
}

func (s *GCSStorage) put(ctx context.Context, key string, data []byte) error {
	w := s.client.Bucket(s.bucket).Object(key).NewWriter(ctx)
	w.ContentType = "application/json"
	if _, err := w.Write(data); err != nil {
		w.Close()
		return fmt.Errorf("gcs write %s: %w", key, err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("gcs close %s: %w", key, err)
	}
	return nil
}

func (s *GCSStorage) get(ctx context.Context, key string) ([]byte, error) {
	r, err := s.client.Bucket(s.bucket).Object(key).NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("gcs read %s: %w", key, err)
	}
	defer r.Close()
	return io.ReadAll(r)
}

func (s *GCSStorage) PutSnapshot(ctx context.Context, tenantID, snapshotID string, data []byte) error {
	return s.put(ctx, s.key(tenantID, "snapshots", snapshotID), data)
}

func (s *GCSStorage) GetSnapshot(ctx context.Context, tenantID, snapshotID string) ([]byte, error) {
	return s.get(ctx, s.key(tenantID, "snapshots", snapshotID))
}

func (s *GCSStorage) PutDelta(ctx context.Context, tenantID, deltaID string, data []byte) error {
	return s.put(ctx, s.key(tenantID, "deltas", deltaID), data)
}

func (s *GCSStorage) GetDelta(ctx context.Context, tenantID, deltaID string) ([]byte, error) {
	return s.get(ctx, s.key(tenantID, "deltas", deltaID))
}

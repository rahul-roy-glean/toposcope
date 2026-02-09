// Package ingestion orchestrates the Toposcope pipeline: extraction, delta
// computation, scoring, and result storage.
package ingestion

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// StorageClient abstracts blob storage for snapshots and deltas.
type StorageClient interface {
	PutSnapshot(ctx context.Context, tenantID, snapshotID string, data []byte) error
	GetSnapshot(ctx context.Context, tenantID, snapshotID string) ([]byte, error)
	PutDelta(ctx context.Context, tenantID, deltaID string, data []byte) error
	GetDelta(ctx context.Context, tenantID, deltaID string) ([]byte, error)
}

// LocalStorage implements StorageClient using the local filesystem.
// Useful for development and testing.
type LocalStorage struct {
	BaseDir string
}

// NewLocalStorage creates a LocalStorage rooted at the given directory.
func NewLocalStorage(baseDir string) *LocalStorage {
	return &LocalStorage{BaseDir: baseDir}
}

func (s *LocalStorage) path(tenantID, kind, id string) string {
	return filepath.Join(s.BaseDir, tenantID, kind, id+".json")
}

func (s *LocalStorage) put(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

// PutSnapshot stores a snapshot blob.
func (s *LocalStorage) PutSnapshot(ctx context.Context, tenantID, snapshotID string, data []byte) error {
	return s.put(s.path(tenantID, "snapshots", snapshotID), data)
}

// GetSnapshot retrieves a snapshot blob.
func (s *LocalStorage) GetSnapshot(ctx context.Context, tenantID, snapshotID string) ([]byte, error) {
	return os.ReadFile(s.path(tenantID, "snapshots", snapshotID))
}

// PutDelta stores a delta blob.
func (s *LocalStorage) PutDelta(ctx context.Context, tenantID, deltaID string, data []byte) error {
	return s.put(s.path(tenantID, "deltas", deltaID), data)
}

// GetDelta retrieves a delta blob.
func (s *LocalStorage) GetDelta(ctx context.Context, tenantID, deltaID string) ([]byte, error) {
	return os.ReadFile(s.path(tenantID, "deltas", deltaID))
}

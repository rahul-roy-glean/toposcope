package ingestion

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalStoragePutGetSnapshot(t *testing.T) {
	dir := t.TempDir()
	s := NewLocalStorage(dir)
	ctx := context.Background()

	data := []byte(`{"nodes":{}}`)
	if err := s.PutSnapshot(ctx, "tenant1", "snap1", data); err != nil {
		t.Fatalf("PutSnapshot: %v", err)
	}

	got, err := s.GetSnapshot(ctx, "tenant1", "snap1")
	if err != nil {
		t.Fatalf("GetSnapshot: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("GetSnapshot = %q, want %q", got, data)
	}

	// Verify file path layout
	expectedPath := filepath.Join(dir, "tenant1", "snapshots", "snap1.json")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Errorf("expected file at %s: %v", expectedPath, err)
	}
}

func TestLocalStoragePutGetDelta(t *testing.T) {
	dir := t.TempDir()
	s := NewLocalStorage(dir)
	ctx := context.Background()

	data := []byte(`{"added_nodes":[]}`)
	if err := s.PutDelta(ctx, "tenant1", "delta1", data); err != nil {
		t.Fatalf("PutDelta: %v", err)
	}

	got, err := s.GetDelta(ctx, "tenant1", "delta1")
	if err != nil {
		t.Fatalf("GetDelta: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("GetDelta = %q, want %q", got, data)
	}

	expectedPath := filepath.Join(dir, "tenant1", "deltas", "delta1.json")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Errorf("expected file at %s: %v", expectedPath, err)
	}
}

func TestLocalStorageGetNotFound(t *testing.T) {
	dir := t.TempDir()
	s := NewLocalStorage(dir)
	ctx := context.Background()

	_, err := s.GetSnapshot(ctx, "tenant1", "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent snapshot")
	}
}

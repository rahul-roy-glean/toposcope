package graph

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// SaveSnapshot writes a snapshot to disk as JSON.
func SaveSnapshot(path string, snap *Snapshot) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating directory for snapshot: %w", err)
	}

	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling snapshot: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing snapshot: %w", err)
	}

	return nil
}

// LoadSnapshot reads a snapshot from disk.
func LoadSnapshot(path string) (*Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading snapshot: %w", err)
	}

	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("unmarshaling snapshot: %w", err)
	}

	return &snap, nil
}

// SaveDelta writes a delta to disk as JSON.
func SaveDelta(path string, delta *Delta) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating directory for delta: %w", err)
	}

	data, err := json.MarshalIndent(delta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling delta: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing delta: %w", err)
	}

	return nil
}

// LoadDelta reads a delta from disk.
func LoadDelta(path string) (*Delta, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading delta: %w", err)
	}

	var delta Delta
	if err := json.Unmarshal(data, &delta); err != nil {
		return nil, fmt.Errorf("unmarshaling delta: %w", err)
	}

	return &delta, nil
}

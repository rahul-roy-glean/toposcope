package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Extraction.Timeout != 600 {
		t.Errorf("expected default timeout 600, got %d", cfg.Extraction.Timeout)
	}
	if cfg.Extraction.BazelPath != "bazelisk" {
		t.Errorf("expected default BazelPath 'bazelisk', got %q", cfg.Extraction.BazelPath)
	}
	if len(cfg.Scoring.Boundaries) != 4 {
		t.Errorf("expected 4 default boundaries, got %d", len(cfg.Scoring.Boundaries))
	}
	if cfg.Scoring.Weights == nil {
		t.Error("expected Weights map to be initialized, got nil")
	}
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name      string
		yaml      string
		wantErr   bool
		check     func(t *testing.T, cfg *Config)
	}{
		{
			name: "non-existent file returns defaults",
			yaml: "", // signal: don't create a file
			check: func(t *testing.T, cfg *Config) {
				if cfg.Extraction.Timeout != 600 {
					t.Errorf("expected default timeout 600, got %d", cfg.Extraction.Timeout)
				}
				if cfg.Extraction.BazelPath != "bazelisk" {
					t.Errorf("expected default BazelPath, got %q", cfg.Extraction.BazelPath)
				}
			},
		},
		{
			name: "valid YAML overrides defaults",
			yaml: `
extraction:
  timeout: 120
  bazel_path: "/usr/bin/bazel"
  use_cquery: true
scoring:
  boundaries:
    - svc
    - lib
  weights:
    coupling: 0.5
    cohesion: 0.3
`,
			check: func(t *testing.T, cfg *Config) {
				if cfg.Extraction.Timeout != 120 {
					t.Errorf("expected timeout 120, got %d", cfg.Extraction.Timeout)
				}
				if cfg.Extraction.BazelPath != "/usr/bin/bazel" {
					t.Errorf("expected BazelPath '/usr/bin/bazel', got %q", cfg.Extraction.BazelPath)
				}
				if !cfg.Extraction.UseCQuery {
					t.Error("expected UseCQuery true")
				}
				if len(cfg.Scoring.Boundaries) != 2 {
					t.Errorf("expected 2 boundaries, got %d", len(cfg.Scoring.Boundaries))
				}
				if cfg.Scoring.Weights["coupling"] != 0.5 {
					t.Errorf("expected coupling weight 0.5, got %f", cfg.Scoring.Weights["coupling"])
				}
			},
		},
		{
			name:    "invalid YAML returns error",
			yaml:    "{{invalid yaml",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.yaml")

			if tc.yaml == "" && tc.name == "non-existent file returns defaults" {
				// Don't create file - test loading non-existent path
				cfg, err := Load(path)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				tc.check(t, cfg)
				return
			}

			if err := os.WriteFile(path, []byte(tc.yaml), 0o644); err != nil {
				t.Fatalf("write test config: %v", err)
			}

			cfg, err := Load(path)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.check != nil {
				tc.check(t, cfg)
			}
		})
	}
}

func TestDirectoryFunctions(t *testing.T) {
	// repoSlug is unexported, but we can test it indirectly via the
	// public Dir functions which all use CacheDir -> repoSlug.
	workspace := "/home/alice/repos/myproject"

	snap := SnapshotDir(workspace)
	score := ScoreDir(workspace)
	hash := HashCacheDir(workspace)

	// All should contain the slug "repos_myproject"
	slug := "repos_myproject"

	if !strings.Contains(snap, slug) {
		t.Errorf("SnapshotDir should contain slug %q, got %q", slug, snap)
	}
	if !strings.Contains(score, slug) {
		t.Errorf("ScoreDir should contain slug %q, got %q", slug, score)
	}
	if !strings.Contains(hash, slug) {
		t.Errorf("HashCacheDir should contain slug %q, got %q", slug, hash)
	}

	// Verify subdirectory names
	if !strings.HasSuffix(snap, filepath.Join(slug, "snapshots")) {
		t.Errorf("SnapshotDir should end with %q, got %q", filepath.Join(slug, "snapshots"), snap)
	}
	if !strings.HasSuffix(score, filepath.Join(slug, "scores")) {
		t.Errorf("ScoreDir should end with %q, got %q", filepath.Join(slug, "scores"), score)
	}
	if !strings.HasSuffix(hash, filepath.Join(slug, "hashes")) {
		t.Errorf("HashCacheDir should end with %q, got %q", filepath.Join(slug, "hashes"), hash)
	}
}

func TestRepoSlug(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "normal path",
			path: "/home/user/workspace/myrepo",
			want: "workspace_myrepo",
		},
		{
			name: "short path",
			path: "/myrepo",
			want: "/_myrepo", // filepath.Base of "/" depends on OS, test via Dir funcs
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := repoSlug(tc.path)
			if got != tc.want {
				t.Errorf("repoSlug(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestFindWorkspaceRoot(t *testing.T) {
	tests := []struct {
		name    string
		marker  string
		wantErr bool
	}{
		{name: "MODULE.bazel", marker: "MODULE.bazel"},
		{name: "WORKSPACE", marker: "WORKSPACE"},
		{name: "WORKSPACE.bazel", marker: "WORKSPACE.bazel"},
		{name: "no marker", marker: "", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()

			if tc.marker != "" {
				markerPath := filepath.Join(root, tc.marker)
				if err := os.WriteFile(markerPath, nil, 0o644); err != nil {
					t.Fatalf("create marker: %v", err)
				}
			}

			// Create a subdirectory and search from there
			sub := filepath.Join(root, "src", "pkg")
			if err := os.MkdirAll(sub, 0o755); err != nil {
				t.Fatalf("create subdirectory: %v", err)
			}

			got, err := FindWorkspaceRoot(sub)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != root {
				t.Errorf("FindWorkspaceRoot = %q, want %q", got, root)
			}
		})
	}
}

func TestFindConfigFile(t *testing.T) {
	t.Run("found in current directory", func(t *testing.T) {
		root := t.TempDir()
		configDir := filepath.Join(root, ".toposcope")
		if err := os.MkdirAll(configDir, 0o755); err != nil {
			t.Fatalf("create config dir: %v", err)
		}
		configPath := filepath.Join(configDir, "config.yaml")
		if err := os.WriteFile(configPath, []byte("{}"), 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}

		got := FindConfigFile(root)
		if got != configPath {
			t.Errorf("FindConfigFile = %q, want %q", got, configPath)
		}
	})

	t.Run("found in parent directory", func(t *testing.T) {
		root := t.TempDir()
		configDir := filepath.Join(root, ".toposcope")
		if err := os.MkdirAll(configDir, 0o755); err != nil {
			t.Fatalf("create config dir: %v", err)
		}
		configPath := filepath.Join(configDir, "config.yaml")
		if err := os.WriteFile(configPath, []byte("{}"), 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}

		sub := filepath.Join(root, "a", "b", "c")
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatalf("create sub: %v", err)
		}

		got := FindConfigFile(sub)
		if got != configPath {
			t.Errorf("FindConfigFile = %q, want %q", got, configPath)
		}
	})

	t.Run("not found", func(t *testing.T) {
		root := t.TempDir()
		got := FindConfigFile(root)
		if got != "" {
			t.Errorf("FindConfigFile = %q, want empty", got)
		}
	})
}

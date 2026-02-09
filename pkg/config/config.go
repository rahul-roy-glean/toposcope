// Package config handles loading and managing Toposcope configuration.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration for Toposcope.
type Config struct {
	Scoring    ScoringConfig    `yaml:"scoring"`
	Extraction ExtractionConfig `yaml:"extraction"`
}

// ScoringConfig controls scoring behavior.
type ScoringConfig struct {
	Boundaries []string           `yaml:"boundaries"`
	Weights    map[string]float64 `yaml:"weights"`
}

// ExtractionConfig controls extraction behavior.
type ExtractionConfig struct {
	Timeout      int    `yaml:"timeout"` // seconds
	BazelPath    string `yaml:"bazel_path"`
	BazelRC      string `yaml:"bazelrc"`
	UseCQuery    bool   `yaml:"use_cquery"`
	BazelDiffJar string `yaml:"bazel_diff_jar"` // path to bazel-diff.jar
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Scoring: ScoringConfig{
			Boundaries: []string{"app", "lib", "platform", "proto"},
			Weights:    map[string]float64{},
		},
		Extraction: ExtractionConfig{
			Timeout:   600,
			BazelPath: "bazelisk",
		},
	}
}

// Load reads a config file from the given path.
// If the file does not exist, it returns the default config.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	return cfg, nil
}

// FindConfigFile looks for .toposcope/config.yaml in the given directory
// and its parents, returning the path if found, or "" if not.
func FindConfigFile(dir string) string {
	for {
		candidate := filepath.Join(dir, ".toposcope", "config.yaml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// FindBazelDiffJar looks for bazel-diff.jar in common locations.
func FindBazelDiffJar() string {
	candidates := []string{
		"bazel-diff.jar", // current dir
		filepath.Join(os.Getenv("HOME"), "bazel-diff.jar"),        // home dir
		filepath.Join(os.Getenv("HOME"), "bazel-diff_deploy.jar"), // alternate name
		filepath.Join(os.Getenv("HOME"), "bin", "bazel-diff.jar"), // ~/bin
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	return ""
}

// CacheDir returns the cache directory for a given workspace path.
// Uses ~/.cache/toposcope/<repo-slug>/ to avoid polluting the repo.
func CacheDir(workspacePath string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback to temp dir if HOME isn't available
		home = os.TempDir()
	}
	slug := repoSlug(workspacePath)
	return filepath.Join(home, ".cache", "toposcope", slug)
}

// SnapshotDir returns the snapshot storage directory for a workspace.
func SnapshotDir(workspacePath string) string {
	return filepath.Join(CacheDir(workspacePath), "snapshots")
}

// HashCacheDir returns the bazel-diff hash cache directory for a workspace.
func HashCacheDir(workspacePath string) string {
	return filepath.Join(CacheDir(workspacePath), "hashes")
}

// ScoreDir returns the score result storage directory for a workspace.
func ScoreDir(workspacePath string) string {
	return filepath.Join(CacheDir(workspacePath), "scores")
}

// repoSlug creates a filesystem-safe identifier from a workspace path.
// Uses the last two path components (e.g., "user/myrepo" from "/home/user/workspace/myrepo").
func repoSlug(workspacePath string) string {
	abs, err := filepath.Abs(workspacePath)
	if err != nil {
		abs = workspacePath
	}
	// Use last two path components for readability
	dir := filepath.Base(filepath.Dir(abs))
	base := filepath.Base(abs)
	return dir + "_" + base
}

// FindWorkspaceRoot walks up from dir looking for MODULE.bazel or WORKSPACE files.
func FindWorkspaceRoot(dir string) (string, error) {
	for {
		for _, marker := range []string{"MODULE.bazel", "WORKSPACE", "WORKSPACE.bazel"} {
			candidate := filepath.Join(dir, marker)
			if _, err := os.Stat(candidate); err == nil {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("no Bazel workspace found (looked for MODULE.bazel or WORKSPACE)")
}

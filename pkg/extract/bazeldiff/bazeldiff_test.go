package bazeldiff

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/toposcope/toposcope/pkg/extract"
)

func TestNormalizeLabel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"//app/foo:lib", "//app/foo:lib"},
		{"//app/foo:foo", "//app/foo"},
		{"@//app/foo:lib", "//app/foo:lib"},
		{"@//app/foo:foo", "//app/foo"},
		{"//app/foo", "//app/foo"},
		{"//lib/bar:bar", "//lib/bar"},
		{"  //app/foo:lib  ", "//app/foo:lib"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeLabel(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeLabel(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseTargetList(t *testing.T) {
	output := "//app/foo:lib\n//lib/bar:bar\n\n//app/baz:test\n"
	targets := parseTargetList(output)
	if len(targets) != 3 {
		t.Fatalf("got %d targets, want 3", len(targets))
	}
	if targets[0] != "//app/foo:lib" {
		t.Errorf("targets[0] = %q, want %q", targets[0], "//app/foo:lib")
	}
}

func TestFilterTargets(t *testing.T) {
	targets := []string{
		"//app/foo:lib",
		"@pip//some/dep:dep",
		"@maven//com/example:lib",
		"@com_google_protobuf//:protobuf",
		".hidden_target",
		"//lib/bar:bar",
	}
	filtered := filterTargets(targets)
	if len(filtered) != 2 {
		t.Fatalf("got %d filtered targets, want 2: %v", len(filtered), filtered)
	}
	if filtered[0] != "//app/foo:lib" {
		t.Errorf("filtered[0] = %q, want %q", filtered[0], "//app/foo:lib")
	}
	if filtered[1] != "//lib/bar:bar" {
		t.Errorf("filtered[1] = %q, want %q", filtered[1], "//lib/bar:bar")
	}
}

func TestShouldKeep(t *testing.T) {
	tests := []struct {
		target string
		want   bool
	}{
		{"//app/foo:lib", true},
		{"@pip//dep:dep", false},
		{"@maven//dep:dep", false},
		{"@com_google_protobuf//:protobuf", false},
		{".hidden", false},
		{"//lib/bar:bar", true},
	}

	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			got := shouldKeep(tt.target)
			if got != tt.want {
				t.Errorf("shouldKeep(%q) = %v, want %v", tt.target, got, tt.want)
			}
		})
	}
}

func TestGenerateHashesCaching(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Pre-create a cached hash file
	cachedFile := filepath.Join(cacheDir, "abc123.json")
	if err := os.WriteFile(cachedFile, []byte(`{"test": true}`), 0o644); err != nil {
		t.Fatal(err)
	}

	runner := &Runner{
		WorkspacePath: dir,
		CacheDir:      cacheDir,
	}

	// Should return cached file without running bazel-diff
	result, err := runner.GenerateHashes(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("GenerateHashes with cache: %v", err)
	}
	if result != cachedFile {
		t.Errorf("got %q, want %q", result, cachedFile)
	}
}

func TestBuildCommandJar(t *testing.T) {
	runner := &Runner{
		BazelDiffJarPath: "/path/to/bazel-diff.jar",
		WorkspacePath:    "/workspace",
		BazelPath:        "bazelisk",
	}

	cmd := runner.buildCommand(context.Background(), "generate-hashes", []string{"-w", "/workspace"})
	args := cmd.Args
	if args[0] != "java" {
		t.Errorf("expected java, got %s", args[0])
	}
	if args[1] != "-jar" {
		t.Errorf("expected -jar, got %s", args[1])
	}
	if args[2] != "/path/to/bazel-diff.jar" {
		t.Errorf("expected jar path, got %s", args[2])
	}
	if args[3] != "generate-hashes" {
		t.Errorf("expected generate-hashes, got %s", args[3])
	}
}

func TestBuildCommandBazelRun(t *testing.T) {
	runner := &Runner{
		WorkspacePath: "/workspace",
		BazelPath:     "bazel",
	}

	cmd := runner.buildCommand(context.Background(), "get-impacted-targets", []string{"-sh", "base.json"})
	args := cmd.Args
	if args[0] != "bazel" {
		t.Errorf("expected bazel, got %s", args[0])
	}
	if args[1] != "run" {
		t.Errorf("expected run, got %s", args[1])
	}
}

func TestBuildGenerateHashesArgs(t *testing.T) {
	runner := &Runner{
		WorkspacePath: "/workspace",
		BazelPath:     "bazelisk",
		BazelRC:       "/workspace/.bazelrc",
		UseCQuery:     true,
	}

	args := runner.buildGenerateHashesArgs("abc123", "/cache/abc123.json")

	assertContains := func(flag, value string) {
		for i, a := range args {
			if a == flag && i+1 < len(args) && args[i+1] == value {
				return
			}
		}
		t.Errorf("args %v missing %s %s", args, flag, value)
	}

	assertContains("-w", "/workspace")
	assertContains("-o", "/cache/abc123.json")
	assertContains("-b", "bazelisk")

	hasUseCquery := false
	for _, a := range args {
		if a == "--useCquery" {
			hasUseCquery = true
		}
	}
	if !hasUseCquery {
		t.Error("expected --useCquery flag")
	}
}

// Verify Runner satisfies ChangeDetector interface
var _ extract.ChangeDetector = (*Runner)(nil)

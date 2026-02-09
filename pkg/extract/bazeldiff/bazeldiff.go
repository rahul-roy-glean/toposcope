// Package bazeldiff wraps the bazel-diff Java tool for change detection.
package bazeldiff

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/toposcope/toposcope/pkg/extract"
)

// Runner wraps the bazel-diff tool to detect impacted targets between commits.
type Runner struct {
	BazelDiffJarPath string // path to bazel-diff.jar (or "" to use `bazel run @bazel-diff`)
	WorkspacePath    string
	BazelPath        string // bazelisk or bazel
	BazelRC          string // .bazelrc file to use
	UseCQuery        bool
	CacheDir         string // where to store hash files
}

// externalTargetPrefixes lists target prefixes to filter out from impacted targets.
var externalTargetPrefixes = []string{"@pip", "@maven", "@com_", "."}

// GenerateHashes runs bazel-diff generate-hashes for the given commit.
// If a cached hash file exists, it returns immediately.
func (r *Runner) GenerateHashes(ctx context.Context, commitSHA string) (string, error) {
	hashFile := filepath.Join(r.CacheDir, commitSHA+".json")

	if _, err := os.Stat(hashFile); err == nil {
		return hashFile, nil
	}

	if err := os.MkdirAll(r.CacheDir, 0o755); err != nil {
		return "", fmt.Errorf("creating cache dir: %w", err)
	}

	args := r.buildGenerateHashesArgs(commitSHA, hashFile)
	cmd := r.buildCommand(ctx, "generate-hashes", args)
	cmd.Dir = r.WorkspacePath

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("generate-hashes for %s failed: %w\nstderr: %s", commitSHA, err, stderr.String())
	}

	return hashFile, nil
}

// GetImpactedTargets runs bazel-diff get-impacted-targets to find changed targets.
func (r *Runner) GetImpactedTargets(ctx context.Context, baseHashFile, headHashFile string) ([]string, error) {
	args := []string{
		"-sh", baseHashFile,
		"-fh", headHashFile,
	}

	cmd := r.buildCommand(ctx, "get-impacted-targets", args)
	cmd.Dir = r.WorkspacePath

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("get-impacted-targets failed: %w\nstderr: %s", err, stderr.String())
	}

	return filterTargets(parseTargetList(stdout.String())), nil
}

// DetectChanges implements extract.ChangeDetector.
func (r *Runner) DetectChanges(ctx context.Context, req extract.ChangeDetectionRequest) (*extract.ChangeDetectionResult, error) {
	start := time.Now()

	runner := r
	if req.BazelPath != "" {
		runner = r.withBazelPath(req.BazelPath)
	}
	if req.CacheDir != "" {
		runner.CacheDir = req.CacheDir
	}

	baseHash, err := runner.GenerateHashes(ctx, req.BaseSHA)
	if err != nil {
		return nil, fmt.Errorf("generating base hashes: %w", err)
	}

	headHash, err := runner.GenerateHashes(ctx, req.HeadSHA)
	if err != nil {
		return nil, fmt.Errorf("generating head hashes: %w", err)
	}

	targets, err := runner.GetImpactedTargets(ctx, baseHash, headHash)
	if err != nil {
		return nil, fmt.Errorf("getting impacted targets: %w", err)
	}

	return &extract.ChangeDetectionResult{
		ImpactedTargets: targets,
		BaseHashFile:    baseHash,
		HeadHashFile:    headHash,
		Duration:        time.Since(start),
	}, nil
}

func (r *Runner) buildCommand(ctx context.Context, subcommand string, extraArgs []string) *exec.Cmd {
	if r.BazelDiffJarPath != "" {
		args := []string{"-jar", r.BazelDiffJarPath, subcommand}
		args = append(args, extraArgs...)
		return exec.CommandContext(ctx, "java", args...)
	}

	bazel := r.BazelPath
	if bazel == "" {
		bazel = "bazelisk"
	}
	args := []string{"run", "@bazel_diff//:bazel-diff", "--", subcommand}
	args = append(args, extraArgs...)
	return exec.CommandContext(ctx, bazel, args...)
}

func (r *Runner) buildGenerateHashesArgs(commitSHA, outputFile string) []string {
	args := []string{
		"-w", r.WorkspacePath,
		"--excludeExternalTargets",
	}

	bazel := r.BazelPath
	if bazel == "" {
		bazel = "bazelisk"
	}
	args = append(args, "-b", bazel)

	if r.BazelRC != "" {
		args = append(args, "-so", "--nohome_rc", "-so", "--bazelrc="+r.BazelRC)
	} else {
		args = append(args, "-so", "--nohome_rc")
	}

	if r.UseCQuery {
		args = append(args, "--useCquery")
	}

	args = append(args, "-o", outputFile)

	return args
}

func (r *Runner) withBazelPath(bp string) *Runner {
	copy := *r
	copy.BazelPath = bp
	return &copy
}

// parseTargetList splits newline-separated target output into a string slice.
func parseTargetList(output string) []string {
	var targets []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			targets = append(targets, line)
		}
	}
	return targets
}

// filterTargets removes external/irrelevant targets.
func filterTargets(targets []string) []string {
	var filtered []string
	for _, t := range targets {
		if shouldKeep(t) {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

func shouldKeep(target string) bool {
	for _, prefix := range externalTargetPrefixes {
		if strings.HasPrefix(target, prefix) {
			return false
		}
	}
	return true
}

// NormalizeLabel normalizes a Bazel label to a canonical form.
// Strips @// prefix to //, handles //pkg:pkg -> //pkg shorthand.
func NormalizeLabel(label string) string {
	label = strings.TrimSpace(label)

	// Strip @// to //
	if strings.HasPrefix(label, "@//") {
		label = label[1:]
	}

	// Handle //pkg:pkg -> //pkg shorthand
	if idx := strings.LastIndex(label, ":"); idx > 0 {
		pkg := label[:idx]
		target := label[idx+1:]
		pkgBase := filepath.Base(pkg)
		if target == pkgBase {
			return pkg
		}
	}

	return label
}

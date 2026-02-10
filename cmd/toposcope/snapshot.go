package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/toposcope/toposcope/pkg/config"
	"github.com/toposcope/toposcope/pkg/extract"
	"github.com/toposcope/toposcope/pkg/extract/subgraph"
	"github.com/toposcope/toposcope/pkg/graph"
)

func newSnapshotCmd() *cobra.Command {
	var (
		repoPath  string
		scope     string
		output    string
		bazelPath string
		bazelRC   string
		useCQuery bool
	)

	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Extract a graph snapshot from a Bazel workspace",
		Long:  `Runs bazel query to extract the build dependency graph and saves a snapshot.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSnapshot(cmd.Context(), snapshotOpts{
				repoPath:  repoPath,
				scope:     scope,
				output:    output,
				bazelPath: bazelPath,
				bazelRC:   bazelRC,
				useCQuery: useCQuery,
			})
		},
	}

	cmd.Flags().StringVar(&repoPath, "repo-path", "", "Path to repository root (default: detect workspace)")
	cmd.Flags().StringVar(&scope, "scope", "FULL", "Extraction scope: FULL or SCOPED")
	cmd.Flags().StringVar(&output, "output", "", "Output path (default: ~/.cache/toposcope/<repo>/snapshots/<sha>.json)")
	cmd.Flags().StringVar(&bazelPath, "bazel-path", "", "Path to bazel/bazelisk binary")
	cmd.Flags().StringVar(&bazelRC, "bazelrc", "", "Path to .bazelrc file")
	cmd.Flags().BoolVar(&useCQuery, "cquery", false, "Use cquery instead of query")

	return cmd
}

type snapshotOpts struct {
	repoPath  string
	scope     string
	output    string
	bazelPath string
	bazelRC   string
	useCQuery bool
}

func runSnapshot(ctx context.Context, opts snapshotOpts) error {
	// Resolve workspace root
	wsRoot, err := resolveWorkspace(opts.repoPath)
	if err != nil {
		return err
	}

	// Load config
	cfg := loadConfig(wsRoot)
	bazelPath := firstNonEmpty(opts.bazelPath, cfg.Extraction.BazelPath, "bazelisk")
	bazelRC := firstNonEmpty(opts.bazelRC, cfg.Extraction.BazelRC)

	// Get current commit SHA
	commitSHA, err := gitRevParse(ctx, wsRoot, "HEAD")
	if err != nil {
		return fmt.Errorf("getting current commit: %w", err)
	}

	ext := &subgraph.Extractor{
		WorkspacePath: wsRoot,
		BazelPath:     bazelPath,
		BazelRC:       bazelRC,
		UseCQuery:     opts.useCQuery || cfg.Extraction.UseCQuery,
	}

	scopeMode := extract.ScopeModeFull
	if strings.EqualFold(opts.scope, "SCOPED") {
		scopeMode = extract.ScopeModeScoped
	}

	timeout := time.Duration(cfg.Extraction.Timeout) * time.Second
	fmt.Fprintf(os.Stderr, "Extracting %s snapshot for %s...\n", scopeMode, commitSHA[:minInt(7, len(commitSHA))])

	var snap *graph.Snapshot
	switch scopeMode {
	case extract.ScopeModeFull:
		snap, err = ext.ExtractFull(ctx, commitSHA, timeout)
	default:
		snap, err = ext.Extract(ctx, subgraph.SubgraphRequest{
			CommitSHA: commitSHA,
			RdepDepth: 2,
			Timeout:   timeout,
		})
	}
	if err != nil {
		return fmt.Errorf("extraction failed: %w", err)
	}

	// Determine output path
	outPath := opts.output
	if outPath == "" {
		outPath = filepath.Join(config.SnapshotDir(wsRoot), commitSHA+".json")
	}

	if err := graph.SaveSnapshot(outPath, snap); err != nil {
		return fmt.Errorf("saving snapshot: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Snapshot saved to %s\n", outPath)
	fmt.Fprintf(os.Stderr, "  Nodes:    %d\n", snap.Stats.NodeCount)
	fmt.Fprintf(os.Stderr, "  Edges:    %d\n", snap.Stats.EdgeCount)
	fmt.Fprintf(os.Stderr, "  Packages: %d\n", snap.Stats.PackageCount)
	fmt.Fprintf(os.Stderr, "  Duration: %dms\n", snap.Stats.ExtractionMs)

	return nil
}

func resolveWorkspace(repoPath string) (string, error) {
	if repoPath != "" {
		abs, err := filepath.Abs(repoPath)
		if err != nil {
			return "", fmt.Errorf("resolving repo path: %w", err)
		}
		return config.FindWorkspaceRoot(abs)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}

	return config.FindWorkspaceRoot(cwd)
}

func gitSymbolicRef(ctx context.Context, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "symbolic-ref", "--short", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func gitRevParse(ctx context.Context, dir, ref string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", ref)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse %s: %w", ref, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func loadConfig(wsRoot string) *config.Config {
	cfgFile := config.FindConfigFile(wsRoot)
	if cfgFile == "" {
		return config.DefaultConfig()
	}
	cfg, err := config.Load(cfgFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load config: %v\n", err)
		return config.DefaultConfig()
	}
	return cfg
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

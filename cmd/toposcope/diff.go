package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/toposcope/toposcope/pkg/config"
	"github.com/toposcope/toposcope/pkg/extract"
	"github.com/toposcope/toposcope/pkg/extract/bazeldiff"
	"github.com/toposcope/toposcope/pkg/extract/subgraph"
	"github.com/toposcope/toposcope/pkg/graph"
)

func newDiffCmd() *cobra.Command {
	var (
		baseRef   string
		headRef   string
		repoPath  string
		bazelPath string
		bazelRC   string
		useCQuery bool
	)

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Compare two snapshots and compute a structural delta",
		Long:  `Detects changed targets between two commits and computes structural differences.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDiff(cmd.Context(), diffOpts{
				baseRef:   baseRef,
				headRef:   headRef,
				repoPath:  repoPath,
				bazelPath: bazelPath,
				bazelRC:   bazelRC,
				useCQuery: useCQuery,
			})
		},
	}

	cmd.Flags().StringVar(&baseRef, "base", "", "Base git ref (required)")
	cmd.Flags().StringVar(&headRef, "head", "HEAD", "Head git ref")
	cmd.Flags().StringVar(&repoPath, "repo-path", "", "Path to repository root (default: detect workspace)")
	cmd.Flags().StringVar(&bazelPath, "bazel-path", "", "Path to bazel/bazelisk binary")
	cmd.Flags().StringVar(&bazelRC, "bazelrc", "", "Path to .bazelrc file")
	cmd.Flags().BoolVar(&useCQuery, "cquery", false, "Use cquery instead of query")
	_ = cmd.MarkFlagRequired("base")

	return cmd
}

type diffOpts struct {
	baseRef   string
	headRef   string
	repoPath  string
	bazelPath string
	bazelRC   string
	useCQuery bool
}

func runDiff(ctx context.Context, opts diffOpts) error {
	wsRoot, err := resolveWorkspace(opts.repoPath)
	if err != nil {
		return err
	}

	cfg := loadConfig(wsRoot)
	bp := firstNonEmpty(opts.bazelPath, cfg.Extraction.BazelPath, "bazelisk")
	brc := firstNonEmpty(opts.bazelRC, cfg.Extraction.BazelRC)
	cq := opts.useCQuery || cfg.Extraction.UseCQuery

	// Resolve git refs to SHAs
	baseSHA, err := gitRevParse(ctx, wsRoot, opts.baseRef)
	if err != nil {
		return fmt.Errorf("resolving base ref: %w", err)
	}
	headSHA, err := gitRevParse(ctx, wsRoot, opts.headRef)
	if err != nil {
		return fmt.Errorf("resolving head ref: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Computing diff: %s..%s\n", baseSHA[:minInt(7, len(baseSHA))], headSHA[:minInt(7, len(headSHA))])

	cacheDir := config.HashCacheDir(wsRoot)
	timeout := time.Duration(cfg.Extraction.Timeout) * time.Second

	// Try to load cached snapshots
	baseSnap, err := loadCachedSnapshot(wsRoot, baseSHA)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Extracting base snapshot...\n")
		ext := &subgraph.Extractor{
			WorkspacePath: wsRoot,
			BazelPath:     bp,
			BazelRC:       brc,
			UseCQuery:     cq,
		}
		baseSnap, err = ext.ExtractFull(ctx, baseSHA, timeout)
		if err != nil {
			return fmt.Errorf("extracting base snapshot: %w", err)
		}
		saveCachedSnapshot(wsRoot, baseSHA, baseSnap)
	}

	headSnap, err := loadCachedSnapshot(wsRoot, headSHA)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Extracting head snapshot...\n")
		ext := &subgraph.Extractor{
			WorkspacePath: wsRoot,
			BazelPath:     bp,
			BazelRC:       brc,
			UseCQuery:     cq,
		}
		headSnap, err = ext.ExtractFull(ctx, headSHA, timeout)
		if err != nil {
			return fmt.Errorf("extracting head snapshot: %w", err)
		}
		saveCachedSnapshot(wsRoot, headSHA, headSnap)
	}

	// Run change detection for impacted targets
	runner := &bazeldiff.Runner{
		WorkspacePath: wsRoot,
		BazelPath:     bp,
		BazelRC:       brc,
		UseCQuery:     cq,
		CacheDir:      cacheDir,
	}

	cdResult, err := runner.DetectChanges(ctx, extract.ChangeDetectionRequest{
		RepoPath:  wsRoot,
		BaseSHA:   baseSHA,
		HeadSHA:   headSHA,
		BazelPath: bp,
		BazelRC:   brc,
		UseCQuery: cq,
		CacheDir:  cacheDir,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: bazel-diff change detection failed: %v\nFalling back to structural diff only.\n", err)
	}

	// Compute delta
	delta := graph.ComputeDelta(baseSnap, headSnap)
	if cdResult != nil {
		delta.ImpactedTargets = cdResult.ImpactedTargets
		delta.Stats.ImpactedTargetCount = len(cdResult.ImpactedTargets)
	}

	// Print results
	printDelta(delta)

	return nil
}

func loadCachedSnapshot(wsRoot, sha string) (*graph.Snapshot, error) {
	path := filepath.Join(config.SnapshotDir(wsRoot), sha+".json")
	return graph.LoadSnapshot(path)
}

func saveCachedSnapshot(wsRoot, sha string, snap *graph.Snapshot) {
	path := filepath.Join(config.SnapshotDir(wsRoot), sha+".json")
	if err := graph.SaveSnapshot(path, snap); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to cache snapshot: %v\n", err)
	}
}

func printDelta(delta *graph.Delta) {
	fmt.Printf("Delta: %s -> %s\n", delta.BaseSnapshotID, delta.HeadSnapshotID)
	fmt.Printf("  Impacted targets: %d\n", delta.Stats.ImpactedTargetCount)
	fmt.Printf("  Added nodes:      %d\n", delta.Stats.AddedNodeCount)
	fmt.Printf("  Removed nodes:    %d\n", delta.Stats.RemovedNodeCount)
	fmt.Printf("  Added edges:      %d\n", delta.Stats.AddedEdgeCount)
	fmt.Printf("  Removed edges:    %d\n", delta.Stats.RemovedEdgeCount)

	if len(delta.ImpactedTargets) > 0 {
		fmt.Println("\nImpacted targets:")
		for _, t := range delta.ImpactedTargets {
			fmt.Printf("  %s\n", t)
		}
	}

	if len(delta.AddedNodes) > 0 {
		fmt.Println("\nAdded nodes:")
		for _, n := range delta.AddedNodes {
			fmt.Printf("  + %s (%s)\n", n.Key, n.Kind)
		}
	}

	if len(delta.RemovedNodes) > 0 {
		fmt.Println("\nRemoved nodes:")
		for _, n := range delta.RemovedNodes {
			fmt.Printf("  - %s (%s)\n", n.Key, n.Kind)
		}
	}

	if len(delta.AddedEdges) > 0 {
		fmt.Println("\nAdded edges:")
		for _, e := range delta.AddedEdges {
			fmt.Printf("  + %s -> %s [%s]\n", e.From, e.To, e.Type)
		}
	}

	if len(delta.RemovedEdges) > 0 {
		fmt.Println("\nRemoved edges:")
		for _, e := range delta.RemovedEdges {
			fmt.Printf("  - %s -> %s [%s]\n", e.From, e.To, e.Type)
		}
	}
}

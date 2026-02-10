package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/toposcope/toposcope/pkg/config"
	"github.com/toposcope/toposcope/pkg/extract"
	"github.com/toposcope/toposcope/pkg/extract/bazeldiff"
	"github.com/toposcope/toposcope/pkg/extract/subgraph"
	"github.com/toposcope/toposcope/pkg/graph"
	"github.com/toposcope/toposcope/pkg/scoring"
	"github.com/toposcope/toposcope/pkg/surface"
)

func newScoreCmd() *cobra.Command {
	var (
		baseRef      string
		headRef      string
		repoPath     string
		bazelPath    string
		bazelRC      string
		useCQuery    bool
		outputFmt    string
		bazelDiffJar string
	)

	cmd := &cobra.Command{
		Use:   "score",
		Short: "Full structural health analysis pipeline",
		Long:  `Runs change detection, subgraph extraction, delta computation, scoring, and rendering.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScore(cmd.Context(), scoreOpts{
				baseRef:      baseRef,
				headRef:      headRef,
				repoPath:     repoPath,
				bazelPath:    bazelPath,
				bazelRC:      bazelRC,
				useCQuery:    useCQuery,
				outputFmt:    outputFmt,
				bazelDiffJar: bazelDiffJar,
			})
		},
	}

	cmd.Flags().StringVar(&baseRef, "base", "", "Base git ref (required)")
	cmd.Flags().StringVar(&headRef, "head", "HEAD", "Head git ref")
	cmd.Flags().StringVar(&repoPath, "repo-path", "", "Path to repository root (default: detect workspace)")
	cmd.Flags().StringVar(&bazelPath, "bazel-path", "", "Path to bazel/bazelisk binary")
	cmd.Flags().StringVar(&bazelRC, "bazelrc", "", "Path to .bazelrc file")
	cmd.Flags().BoolVar(&useCQuery, "cquery", false, "Use cquery instead of query")
	cmd.Flags().StringVar(&outputFmt, "output", "text", "Output format: text or json")
	cmd.Flags().StringVar(&bazelDiffJar, "bazel-diff-jar", "", "Path to bazel-diff.jar")
	_ = cmd.MarkFlagRequired("base")

	return cmd
}

type scoreOpts struct {
	baseRef      string
	headRef      string
	repoPath     string
	bazelPath    string
	bazelRC      string
	useCQuery    bool
	outputFmt    string
	bazelDiffJar string
}

func runScore(ctx context.Context, opts scoreOpts) error {
	wsRoot, err := resolveWorkspace(opts.repoPath)
	if err != nil {
		return err
	}

	cfg := loadConfig(wsRoot)
	bp := firstNonEmpty(opts.bazelPath, cfg.Extraction.BazelPath, "bazelisk")
	brc := firstNonEmpty(opts.bazelRC, cfg.Extraction.BazelRC)
	cq := opts.useCQuery || cfg.Extraction.UseCQuery
	jarPath := firstNonEmpty(opts.bazelDiffJar, cfg.Extraction.BazelDiffJar, config.FindBazelDiffJar())

	// Resolve git refs
	baseSHA, err := gitRevParse(ctx, wsRoot, opts.baseRef)
	if err != nil {
		return fmt.Errorf("resolving base ref: %w", err)
	}
	headSHA, err := gitRevParse(ctx, wsRoot, opts.headRef)
	if err != nil {
		return fmt.Errorf("resolving head ref: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Scoring: %s..%s\n", baseSHA[:minInt(7, len(baseSHA))], headSHA[:minInt(7, len(headSHA))])

	cacheDir := config.HashCacheDir(wsRoot)
	timeout := time.Duration(cfg.Extraction.Timeout) * time.Second

	// Step 1: Change detection via bazel-diff (optional, enhances delta)
	var cdResult *extract.ChangeDetectionResult
	if jarPath != "" {
		fmt.Fprintf(os.Stderr, "Step 1/4: Change detection (bazel-diff)...\n")
		runner := &bazeldiff.Runner{
			BazelDiffJarPath: jarPath,
			WorkspacePath:    wsRoot,
			BazelPath:        bp,
			BazelRC:          brc,
			UseCQuery:        cq,
			CacheDir:         cacheDir,
		}

		cdResult, err = runner.DetectChanges(ctx, extract.ChangeDetectionRequest{
			RepoPath:  wsRoot,
			BaseSHA:   baseSHA,
			HeadSHA:   headSHA,
			BazelPath: bp,
			BazelRC:   brc,
			UseCQuery: cq,
			CacheDir:  cacheDir,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Warning: bazel-diff failed: %v\n", err)
			fmt.Fprintf(os.Stderr, "  Falling back to full extraction at both commits.\n")
			cdResult = nil
		} else {
			fmt.Fprintf(os.Stderr, "  Found %d impacted targets\n", len(cdResult.ImpactedTargets))
		}
	} else {
		fmt.Fprintf(os.Stderr, "Step 1/4: Change detection skipped (no bazel-diff.jar found)\n")
		fmt.Fprintf(os.Stderr, "  Hint: download bazel-diff.jar or pass --bazel-diff-jar\n")
	}

	// Step 2: Extract snapshots
	// We need to extract at both commits. This requires git checkout.
	fmt.Fprintf(os.Stderr, "Step 2/4: Extracting snapshots...\n")
	ext := &subgraph.Extractor{
		WorkspacePath: wsRoot,
		BazelPath:     bp,
		BazelRC:       brc,
		UseCQuery:     cq,
	}

	// Try to load cached snapshots first
	baseSnap, _ := loadCachedSnapshot(wsRoot, baseSHA)
	headSnap, _ := loadCachedSnapshot(wsRoot, headSHA)

	// Record current HEAD so we can restore after checkout.
	// Prefer symbolic ref (branch name) over SHA to avoid detached HEAD.
	origRef, err := gitSymbolicRef(ctx, wsRoot)
	if err != nil {
		origRef, err = gitRevParse(ctx, wsRoot, "HEAD")
		if err != nil {
			return fmt.Errorf("getting current HEAD: %w", err)
		}
	}

	// Check if working tree is dirty
	dirty, err := gitIsDirty(ctx, wsRoot)
	if err != nil {
		return fmt.Errorf("checking working tree: %w", err)
	}

	needsCheckout := (baseSnap == nil && baseSHA != origRef) || (headSnap == nil && headSHA != origRef)

	if needsCheckout && dirty {
		return fmt.Errorf("working tree has uncommitted changes; commit or stash them before scoring across commits")
	}

	// Extract base snapshot
	if baseSnap == nil {
		fmt.Fprintf(os.Stderr, "  Extracting base (%s)...\n", baseSHA[:7])
		if baseSHA != origRef {
			if err := gitCheckout(ctx, wsRoot, baseSHA); err != nil {
				return fmt.Errorf("checking out base commit: %w", err)
			}
			defer func() { _ = gitCheckout(ctx, wsRoot, origRef) }() // restore on exit
		}
		baseSnap, err = ext.ExtractFull(ctx, baseSHA, timeout)
		if err != nil {
			return fmt.Errorf("extracting base snapshot: %w", err)
		}
		saveCachedSnapshot(wsRoot, baseSHA, baseSnap)

		// Checkout back to head for head extraction
		if baseSHA != origRef {
			if err := gitCheckout(ctx, wsRoot, origRef); err != nil {
				return fmt.Errorf("restoring HEAD after base extraction: %w", err)
			}
		}
	} else {
		fmt.Fprintf(os.Stderr, "  Base (%s): cached\n", baseSHA[:7])
	}

	// Extract head snapshot
	if headSnap == nil {
		fmt.Fprintf(os.Stderr, "  Extracting head (%s)...\n", headSHA[:7])
		if headSHA != origRef {
			if err := gitCheckout(ctx, wsRoot, headSHA); err != nil {
				return fmt.Errorf("checking out head commit: %w", err)
			}
			defer func() { _ = gitCheckout(ctx, wsRoot, origRef) }()
		}
		headSnap, err = ext.ExtractFull(ctx, headSHA, timeout)
		if err != nil {
			return fmt.Errorf("extracting head snapshot: %w", err)
		}
		saveCachedSnapshot(wsRoot, headSHA, headSnap)

		if headSHA != origRef {
			if err := gitCheckout(ctx, wsRoot, origRef); err != nil {
				return fmt.Errorf("restoring HEAD after head extraction: %w", err)
			}
		}
	} else {
		fmt.Fprintf(os.Stderr, "  Head (%s): cached\n", headSHA[:7])
	}

	// Step 3: Compute delta
	fmt.Fprintf(os.Stderr, "Step 3/4: Computing delta...\n")
	delta := graph.ComputeDelta(baseSnap, headSnap)
	if cdResult != nil {
		delta.ImpactedTargets = cdResult.ImpactedTargets
		delta.Stats.ImpactedTargetCount = len(cdResult.ImpactedTargets)
	}
	fmt.Fprintf(os.Stderr, "  +%d/-%d nodes, +%d/-%d edges\n",
		delta.Stats.AddedNodeCount, delta.Stats.RemovedNodeCount,
		delta.Stats.AddedEdgeCount, delta.Stats.RemovedEdgeCount)

	// Step 4: Score
	fmt.Fprintf(os.Stderr, "Step 4/4: Scoring...\n")

	metrics := scoring.DefaultMetrics()
	engine := scoring.NewEngine(metrics...)

	result, err := engine.Score(delta, baseSnap, headSnap)
	if err != nil {
		return fmt.Errorf("scoring: %w", err)
	}

	// Save result to disk for the UI server
	saveScoreResult(wsRoot, baseSHA, headSHA, result)

	// Render output
	switch opts.outputFmt {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			return fmt.Errorf("encoding JSON: %w", err)
		}
	default:
		renderer := &surface.TerminalRenderer{}
		if err := renderer.Render(os.Stdout, result); err != nil {
			return fmt.Errorf("rendering: %w", err)
		}
	}

	return nil
}

// saveScoreResult persists a score result to the score cache directory.
func saveScoreResult(wsRoot, baseSHA, headSHA string, result *scoring.ScoreResult) {
	scoreDir := config.ScoreDir(wsRoot)
	if err := os.MkdirAll(scoreDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to create score dir: %v\n", err)
		return
	}

	// Wrap result with metadata for the UI server
	wrapped := struct {
		*scoring.ScoreResult
		ID         string `json:"id"`
		AnalyzedAt string `json:"analyzed_at"`
	}{
		ScoreResult: result,
		ID:          baseSHA[:minInt(8, len(baseSHA))] + "_" + headSHA[:minInt(8, len(headSHA))],
		AnalyzedAt:  time.Now().UTC().Format(time.RFC3339),
	}

	data, err := json.MarshalIndent(wrapped, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to marshal score result: %v\n", err)
		return
	}

	path := filepath.Join(scoreDir, baseSHA+"_"+headSHA+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save score result: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "Score saved: %s\n", path)
}

// gitCheckout runs git checkout at the given ref.
func gitCheckout(ctx context.Context, dir, ref string) error {
	cmd := exec.CommandContext(ctx, "git", "checkout", ref, "--quiet")
	cmd.Dir = dir
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// gitIsDirty returns true if the working tree has uncommitted changes.
func gitIsDirty(ctx context.Context, dir string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return len(out) > 0, nil
}

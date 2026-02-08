// Package extract defines interfaces for build graph extraction.
// Implementations handle the specifics of different build systems and execution environments.
package extract

import (
	"context"
	"time"

	"github.com/toposcope/toposcope/pkg/graph"
)

// Extractor extracts a build graph from a repository.
type Extractor interface {
	// Extract produces a graph snapshot for the given request.
	Extract(ctx context.Context, req ExtractionRequest) (*graph.Snapshot, error)
}

// ExtractionRequest specifies what to extract and how.
type ExtractionRequest struct {
	RepoPath  string          `json:"repo_path"`  // local filesystem path to repo root
	CommitSHA string          `json:"commit_sha"`
	Scope     ExtractionScope `json:"scope"`
}

// ExtractionScope controls what portion of the graph to extract.
type ExtractionScope struct {
	Mode            ScopeMode     `json:"mode"`                       // FULL or SCOPED
	Roots           []string      `json:"roots,omitempty"`            // target roots for scoped extraction
	ChangedFiles    []string      `json:"changed_files,omitempty"`    // files that changed (for scope inference)
	RdepsDepth      int           `json:"rdeps_depth,omitempty"`      // reverse deps depth (default: 2)
	DepsDepth       int           `json:"deps_depth,omitempty"`       // forward deps depth (default: 1)
	MaxNodes        int           `json:"max_nodes,omitempty"`        // cap on total nodes (default: 50000)
	ExcludeExternal bool          `json:"exclude_external,omitempty"` // filter @maven, @pip, etc.
	Timeout         time.Duration `json:"timeout,omitempty"`          // extraction timeout
}

// ScopeMode determines extraction scope.
type ScopeMode string

const (
	ScopeModeFull   ScopeMode = "FULL"
	ScopeModeScoped ScopeMode = "SCOPED"
)

// ChangeDetector identifies which targets changed between two commits.
type ChangeDetector interface {
	// DetectChanges returns the list of impacted targets between two commits.
	DetectChanges(ctx context.Context, req ChangeDetectionRequest) (*ChangeDetectionResult, error)
}

// ChangeDetectionRequest specifies the commits to compare.
type ChangeDetectionRequest struct {
	RepoPath   string `json:"repo_path"`
	BaseSHA    string `json:"base_sha"`
	HeadSHA    string `json:"head_sha"`
	BazelPath  string `json:"bazel_path,omitempty"`  // path to bazel/bazelisk binary
	BazelRC    string `json:"bazelrc,omitempty"`      // which .bazelrc to use
	UseCQuery  bool   `json:"use_cquery,omitempty"`
	CacheDir   string `json:"cache_dir,omitempty"`    // where to cache hash files
}

// ChangeDetectionResult holds the output of change detection.
type ChangeDetectionResult struct {
	ImpactedTargets  []string      `json:"impacted_targets"`
	BaseHashFile     string        `json:"base_hash_file"`
	HeadHashFile     string        `json:"head_hash_file"`
	Duration         time.Duration `json:"duration"`
}

// EdgeType constants for dependency classification.
const (
	EdgeTypeCompile   = "COMPILE"
	EdgeTypeRuntime   = "RUNTIME"
	EdgeTypeToolchain = "TOOLCHAIN"
	EdgeTypeData      = "DATA"
)

// DepAttributes lists which Bazel rule attributes constitute structural dependencies.
// deps and runtime_deps for Phase 1; extensible later.
var DepAttributes = []string{"deps", "runtime_deps"}

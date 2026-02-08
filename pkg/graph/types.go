// Package graph defines the core structural data model for Toposcope.
// These types are the shared vocabulary across all modules.
// Changes to this file require review from all teams.
package graph

import "time"

// Snapshot represents a point-in-time structural view of a repository's build graph.
// Snapshots are immutable once created.
type Snapshot struct {
	ID          string            `json:"id"`
	CommitSHA   string            `json:"commit_sha"`
	Branch      string            `json:"branch,omitempty"` // empty for PR heads
	Partial     bool              `json:"partial"`          // true for scoped PR extractions
	Scope       []string          `json:"scope,omitempty"`  // extraction root targets (if partial)
	Nodes       map[string]*Node  `json:"nodes"`            // keyed by canonical label
	Edges       []Edge            `json:"edges"`
	Stats       SnapshotStats     `json:"stats"`
	ExtractedAt time.Time         `json:"extracted_at"`
}

// Node represents a single build target in the dependency graph.
type Node struct {
	Key        string   `json:"key"`         // canonical Bazel label: "//app/foo:lib"
	Kind       string   `json:"kind"`        // rule class: "go_library", "java_test", etc.
	Package    string   `json:"package"`     // Bazel package: "//app/foo"
	Tags       []string `json:"tags,omitempty"`
	Visibility []string `json:"visibility,omitempty"`
	IsTest     bool     `json:"is_test"`
	IsExternal bool     `json:"is_external"` // labels starting with @
}

// Edge represents a dependency relationship between two targets.
type Edge struct {
	From string `json:"from"` // source node key
	To   string `json:"to"`   // target node key
	Type string `json:"type"` // COMPILE, RUNTIME, TOOLCHAIN, DATA
}

// EdgeKey returns a stable string key for deduplication and set operations.
func (e Edge) EdgeKey() string {
	return e.From + "|" + e.To + "|" + e.Type
}

// SnapshotStats holds summary statistics for a snapshot.
type SnapshotStats struct {
	NodeCount    int `json:"node_count"`
	EdgeCount    int `json:"edge_count"`
	PackageCount int `json:"package_count"`
	ExtractionMs int `json:"extraction_ms"`
}

// Delta represents the structural difference between two snapshots.
// Deltas are immutable once computed.
type Delta struct {
	ID               string   `json:"id"`
	BaseSnapshotID   string   `json:"base_snapshot_id"`
	HeadSnapshotID   string   `json:"head_snapshot_id"`
	ImpactedTargets  []string `json:"impacted_targets"`  // from bazel-diff
	AddedNodes       []Node   `json:"added_nodes"`
	RemovedNodes     []Node   `json:"removed_nodes"`
	AddedEdges       []Edge   `json:"added_edges"`
	RemovedEdges     []Edge   `json:"removed_edges"`
	Stats            DeltaStats `json:"stats"`
}

// DeltaStats holds summary statistics for a delta.
type DeltaStats struct {
	ImpactedTargetCount int `json:"impacted_target_count"`
	AddedNodeCount      int `json:"added_node_count"`
	RemovedNodeCount    int `json:"removed_node_count"`
	AddedEdgeCount      int `json:"added_edge_count"`
	RemovedEdgeCount    int `json:"removed_edge_count"`
}

// InDegreeMap maps node keys to their in-degree count.
// Used for centrality and blast radius calculations.
type InDegreeMap map[string]int

// ComputeInDegrees calculates in-degree for every node in the snapshot.
func (s *Snapshot) ComputeInDegrees() InDegreeMap {
	degrees := make(InDegreeMap, len(s.Nodes))
	for key := range s.Nodes {
		degrees[key] = 0
	}
	for _, edge := range s.Edges {
		degrees[edge.To]++
	}
	return degrees
}

// OutDegreeMap maps node keys to their out-degree count.
type OutDegreeMap map[string]int

// ComputeOutDegrees calculates out-degree for every node in the snapshot.
func (s *Snapshot) ComputeOutDegrees() OutDegreeMap {
	degrees := make(OutDegreeMap, len(s.Nodes))
	for key := range s.Nodes {
		degrees[key] = 0
	}
	for _, edge := range s.Edges {
		degrees[edge.From]++
	}
	return degrees
}

// Packages returns the set of unique packages in the snapshot.
func (s *Snapshot) Packages() map[string]bool {
	pkgs := make(map[string]bool)
	for _, node := range s.Nodes {
		if node.Package != "" {
			pkgs[node.Package] = true
		}
	}
	return pkgs
}

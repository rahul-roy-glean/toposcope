package scoring

import (
	"fmt"
	"strings"

	"github.com/toposcope/toposcope/pkg/graph"
)

// CrossPackageMetric (M1) detects new edges that cross package boundaries.
type CrossPackageMetric struct {
	IntraBoundaryWeight float64  // weight for edges crossing packages within the same top-level dir
	CrossBoundaryWeight float64  // weight for edges crossing top-level directory boundaries
	Boundaries          []string // auto-detected from head snapshot if empty
}

func (m *CrossPackageMetric) Key() string  { return "cross_package_deps" }
func (m *CrossPackageMetric) Name() string { return "Cross-package dependencies" }

func (m *CrossPackageMetric) Evaluate(delta *graph.Delta, base, head *graph.Snapshot) MetricResult {
	result := MetricResult{
		Key:      m.Key(),
		Name:     m.Name(),
		Severity: SeverityMedium,
	}

	boundaries := m.Boundaries
	if len(boundaries) == 0 {
		boundaries = detectBoundaries(head)
	}

	var contribution float64

	for _, edge := range delta.AddedEdges {
		srcNode := head.Nodes[edge.From]
		tgtNode := head.Nodes[edge.To]

		// Skip if source is a test target
		if srcNode != nil && srcNode.IsTest {
			continue
		}
		// Skip if target is external
		if tgtNode != nil && tgtNode.IsExternal {
			continue
		}
		// Skip proto deps
		if tgtNode != nil && strings.Contains(tgtNode.Kind, "proto") {
			continue
		}

		srcPkg := ""
		tgtPkg := ""
		if srcNode != nil {
			srcPkg = srcNode.Package
		}
		if tgtNode != nil {
			tgtPkg = tgtNode.Package
		}

		if srcPkg == "" || tgtPkg == "" || srcPkg == tgtPkg {
			continue
		}

		srcBoundary := topLevelDir(srcPkg)
		tgtBoundary := topLevelDir(tgtPkg)

		if srcBoundary == tgtBoundary {
			// Intra-boundary cross-package
			contribution += m.IntraBoundaryWeight
			result.Evidence = append(result.Evidence, EvidenceItem{
				Type:    EvidenceEdgeAdded,
				Summary: fmt.Sprintf("Intra-boundary cross-package edge: %s -> %s", edge.From, edge.To),
				From:    edge.From,
				To:      edge.To,
				Value:   m.IntraBoundaryWeight,
			})
		} else {
			// Cross-boundary
			contribution += m.CrossBoundaryWeight
			result.Evidence = append(result.Evidence, EvidenceItem{
				Type:    EvidenceEdgeAdded,
				Summary: fmt.Sprintf("Cross-boundary edge: %s -> %s (%s -> %s)", edge.From, edge.To, srcBoundary, tgtBoundary),
				From:    edge.From,
				To:      edge.To,
				Value:   m.CrossBoundaryWeight,
			})
		}
	}

	_ = boundaries // boundaries used for auto-detection above

	result.Contribution = contribution
	if contribution > 5 {
		result.Severity = SeverityHigh
	} else if contribution > 0 {
		result.Severity = SeverityMedium
	} else {
		result.Severity = SeverityInfo
	}

	return result
}

// topLevelDir extracts the first path component from a Bazel package label.
// "//app/auth" -> "app", "//lib/session" -> "lib"
func topLevelDir(pkg string) string {
	p := strings.TrimPrefix(pkg, "//")
	parts := strings.SplitN(p, "/", 2)
	if len(parts) > 0 {
		return parts[0]
	}
	return p
}

// detectBoundaries enumerates unique first path components from all packages in the snapshot.
func detectBoundaries(snap *graph.Snapshot) []string {
	seen := make(map[string]bool)
	for _, node := range snap.Nodes {
		if node.Package != "" {
			b := topLevelDir(node.Package)
			seen[b] = true
		}
	}
	var boundaries []string
	for b := range seen {
		boundaries = append(boundaries, b)
	}
	return boundaries
}

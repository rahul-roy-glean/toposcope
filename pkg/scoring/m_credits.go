package scoring

import (
	"fmt"

	"github.com/toposcope/toposcope/pkg/graph"
)

// CreditsMetric (M6) awards negative score for cleanup work.
type CreditsMetric struct {
	PerRemovedCrossBoundaryEdge float64 // credit per removed cross-boundary edge (negative value)
	MaxCreditTotal              float64 // max total credit for edge removals (negative value)
	PerFanoutReduction          float64 // credit per unit of fanout reduction (negative value)
	FanoutMaxCredit             float64 // max total credit for fanout reduction (negative value)
}

func (m *CreditsMetric) Key() string  { return "cleanup_credits" }
func (m *CreditsMetric) Name() string { return "Cleanup credits" }

func (m *CreditsMetric) Evaluate(delta *graph.Delta, base, head *graph.Snapshot) MetricResult {
	result := MetricResult{
		Key:      m.Key(),
		Name:     m.Name(),
		Severity: SeverityInfo,
	}

	// Build a set of base edges for anti-gaming check
	baseEdgeSet := make(map[string]bool, len(base.Edges))
	for _, e := range base.Edges {
		baseEdgeSet[e.EdgeKey()] = true
	}

	// Credit for removed cross-boundary edges that existed in base
	edgeCredit := 0.0
	for _, edge := range delta.RemovedEdges {
		// Anti-gaming: only credit if the edge actually existed in base
		if !baseEdgeSet[edge.EdgeKey()] {
			continue
		}

		srcNode := base.Nodes[edge.From]
		tgtNode := base.Nodes[edge.To]

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

		if srcBoundary != tgtBoundary {
			edgeCredit += m.PerRemovedCrossBoundaryEdge
			result.Evidence = append(result.Evidence, EvidenceItem{
				Type:    EvidenceEdgeRemoved,
				Summary: fmt.Sprintf("Removed cross-boundary edge: %s -> %s", edge.From, edge.To),
				From:    edge.From,
				To:      edge.To,
				Value:   m.PerRemovedCrossBoundaryEdge,
			})
		}
	}

	// Cap edge credits
	if edgeCredit < m.MaxCreditTotal {
		edgeCredit = m.MaxCreditTotal
	}

	// Credit for fanout reduction
	baseOutDeg := base.ComputeOutDegrees()
	headOutDeg := head.ComputeOutDegrees()

	fanoutCredit := 0.0
	for key := range base.Nodes {
		baseDeg := baseOutDeg[key]
		headDeg := headOutDeg[key] // 0 if not in head
		reduction := baseDeg - headDeg
		if reduction <= 0 {
			continue
		}

		c := m.PerFanoutReduction * float64(reduction)
		fanoutCredit += c

		result.Evidence = append(result.Evidence, EvidenceItem{
			Type:    EvidenceFanoutChange,
			Summary: fmt.Sprintf("Fanout reduced: %s %d -> %d (-%d)", key, baseDeg, headDeg, reduction),
			From:    key,
			Value:   float64(-reduction),
		})
	}

	// Cap fanout credits
	if fanoutCredit < m.FanoutMaxCredit {
		fanoutCredit = m.FanoutMaxCredit
	}

	// Total credit capped at MaxCreditTotal + FanoutMaxCredit
	totalCredit := edgeCredit + fanoutCredit
	minCredit := m.MaxCreditTotal + m.FanoutMaxCredit
	if totalCredit < minCredit {
		totalCredit = minCredit
	}

	result.Contribution = totalCredit

	return result
}

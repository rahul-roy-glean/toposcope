package scoring

import (
	"fmt"
	"math"

	"github.com/toposcope/toposcope/pkg/graph"
)

// CentralityMetric (M3) penalizes adding dependencies on highly-depended-upon targets.
type CentralityMetric struct {
	Weight      float64 // score multiplier
	MinInDegree int     // only apply for targets above this in-degree in base
}

func (m *CentralityMetric) Key() string  { return "centrality_penalty" }
func (m *CentralityMetric) Name() string { return "Centrality penalty" }

func (m *CentralityMetric) Evaluate(delta *graph.Delta, base, head *graph.Snapshot) MetricResult {
	result := MetricResult{
		Key:      m.Key(),
		Name:     m.Name(),
		Severity: SeverityLow,
	}

	// If base snapshot has no nodes (first run), return zero contribution
	if len(base.Nodes) == 0 {
		result.Severity = SeverityInfo
		result.Evidence = append(result.Evidence, EvidenceItem{
			Type:    EvidenceCentrality,
			Summary: "No base snapshot nodes available; skipping centrality penalty",
		})
		return result
	}

	baseInDeg := base.ComputeInDegrees()

	var contribution float64

	for _, edge := range delta.AddedEdges {
		targetInDegree := baseInDeg[edge.To]
		if targetInDegree < m.MinInDegree {
			continue
		}

		c := m.Weight * math.Log2(1+float64(targetInDegree))
		contribution += c

		result.Evidence = append(result.Evidence, EvidenceItem{
			Type:    EvidenceCentrality,
			Summary: fmt.Sprintf("New dep on %s (in-degree %d in base)", edge.To, targetInDegree),
			To:      edge.To,
			Value:   float64(targetInDegree),
		})
	}

	result.Contribution = contribution
	if contribution > 5 {
		result.Severity = SeverityHigh
	} else if contribution > 0 {
		result.Severity = SeverityMedium
	}

	return result
}

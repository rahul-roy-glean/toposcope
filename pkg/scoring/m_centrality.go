package scoring

import (
	"fmt"
	"math"

	"github.com/toposcope/toposcope/pkg/graph"
)

// CentralityMetric (M3) penalizes adding dependencies on highly-depended-upon targets.
type CentralityMetric struct {
	Weight          float64 // score multiplier
	MinInDegree     int     // only apply for targets above this in-degree in base
	MaxContribution float64 // safety cap on total contribution (0 = no cap)
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

	// Group added edges by destination, skipping test sources.
	// This deduplicates: if 12 edges all point to //core, we score //core once.
	type destInfo struct {
		sourceCount int
	}
	destMap := make(map[string]*destInfo)

	for _, edge := range delta.AddedEdges {
		// Skip edges where the source node is a test target
		if srcNode := head.Nodes[edge.From]; srcNode != nil && srcNode.IsTest {
			continue
		}

		if _, ok := destMap[edge.To]; !ok {
			destMap[edge.To] = &destInfo{}
		}
		destMap[edge.To].sourceCount++
	}

	var contribution float64

	for dest, info := range destMap {
		targetInDegree := baseInDeg[dest]
		if targetInDegree < m.MinInDegree {
			continue
		}

		c := m.Weight * math.Log2(1+float64(targetInDegree))
		contribution += c

		summary := fmt.Sprintf("New dep on %s (in-degree %d in base)", dest, targetInDegree)
		if info.sourceCount > 1 {
			summary = fmt.Sprintf("New dep on %s (in-degree %d in base, %d sources)", dest, targetInDegree, info.sourceCount)
		}
		result.Evidence = append(result.Evidence, EvidenceItem{
			Type:    EvidenceCentrality,
			Summary: summary,
			To:      dest,
			Value:   float64(targetInDegree),
		})
	}

	if m.MaxContribution > 0 && contribution > m.MaxContribution {
		contribution = m.MaxContribution
	}

	result.Contribution = contribution
	if contribution > 5 {
		result.Severity = SeverityHigh
	} else if contribution > 0 {
		result.Severity = SeverityMedium
	}

	return result
}

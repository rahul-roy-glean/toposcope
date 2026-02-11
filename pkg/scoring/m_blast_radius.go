package scoring

import (
	"fmt"
	"math"
	"sort"

	"github.com/toposcope/toposcope/pkg/graph"
)

// BlastRadiusMetric (M5) estimates the transitive impact of changes.
type BlastRadiusMetric struct {
	Weight          float64 // score multiplier
	MaxContribution float64 // cap on contribution
}

func (m *BlastRadiusMetric) Key() string  { return "blast_radius" }
func (m *BlastRadiusMetric) Name() string { return "Blast radius" }

func (m *BlastRadiusMetric) Evaluate(delta *graph.Delta, base, head *graph.Snapshot) MetricResult {
	result := MetricResult{
		Key:      m.Key(),
		Name:     m.Name(),
		Severity: SeverityLow,
	}

	// Collect all affected nodes: union of From/To from added and removed edges,
	// plus added/removed node keys
	affected := make(map[string]bool)
	for _, e := range delta.AddedEdges {
		affected[e.From] = true
		affected[e.To] = true
	}
	for _, e := range delta.RemovedEdges {
		affected[e.From] = true
		affected[e.To] = true
	}
	for _, n := range delta.AddedNodes {
		affected[n.Key] = true
	}
	for _, n := range delta.RemovedNodes {
		affected[n.Key] = true
	}

	if len(affected) == 0 {
		result.Severity = SeverityInfo
		return result
	}

	baseInDeg := base.ComputeInDegrees()

	// Sum in-degrees of affected nodes from base.
	// Test nodes contribute at a discounted rate (0.3x).
	var blastRadius float64
	type nodeWithDeg struct {
		key    string
		degree int
	}
	var nodeDegs []nodeWithDeg

	for key := range affected {
		deg := baseInDeg[key]
		weight := 1.0
		if node := base.Nodes[key]; node != nil && node.IsTest {
			weight = 0.3
		} else if node == nil {
			if headNode := head.Nodes[key]; headNode != nil && headNode.IsTest {
				weight = 0.3
			}
		}
		blastRadius += float64(deg) * weight
		nodeDegs = append(nodeDegs, nodeWithDeg{key: key, degree: deg})
	}

	contribution := m.Weight * math.Log2(1+blastRadius)
	if contribution > m.MaxContribution {
		contribution = m.MaxContribution
	}

	result.Contribution = contribution

	// Evidence: top 3 nodes by in-degree from affected set
	sort.Slice(nodeDegs, func(i, j int) bool {
		return nodeDegs[i].degree > nodeDegs[j].degree
	})
	top := 3
	if len(nodeDegs) < top {
		top = len(nodeDegs)
	}
	for i := 0; i < top; i++ {
		nd := nodeDegs[i]
		result.Evidence = append(result.Evidence, EvidenceItem{
			Type:    EvidenceBlastRadius,
			Summary: fmt.Sprintf("Affected node %s has %d reverse deps in base", nd.key, nd.degree),
			From:    nd.key,
			Value:   float64(nd.degree),
		})
	}

	if contribution > 5 {
		result.Severity = SeverityHigh
	} else if contribution > 0 {
		result.Severity = SeverityMedium
	}

	return result
}

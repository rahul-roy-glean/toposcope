package scoring

import (
	"fmt"
	"math"

	"github.com/toposcope/toposcope/pkg/graph"
)

// FanoutMetric (M2) detects targets accumulating too many dependencies.
type FanoutMetric struct {
	Weight       float64 // score contribution per unit of fanout increase
	CapPerNode   float64 // max contribution from a single node
	MinThreshold int     // only score if out_degree(head) > this
}

func (m *FanoutMetric) Key() string  { return "fanout_increase" }
func (m *FanoutMetric) Name() string { return "Fanout increase" }

func (m *FanoutMetric) Evaluate(delta *graph.Delta, base, head *graph.Snapshot) MetricResult {
	result := MetricResult{
		Key:      m.Key(),
		Name:     m.Name(),
		Severity: SeverityLow,
	}

	baseOutDeg := base.ComputeOutDegrees()
	headOutDeg := head.ComputeOutDegrees()

	var contribution float64

	for key, node := range head.Nodes {
		if node.IsTest || node.IsExternal {
			continue
		}

		headDeg := headOutDeg[key]
		if headDeg <= m.MinThreshold {
			continue
		}

		baseDeg := baseOutDeg[key] // 0 if node doesn't exist in base
		deg := headDeg - baseDeg
		if deg <= 0 {
			continue
		}

		c := m.Weight * math.Min(float64(deg), m.CapPerNode)
		contribution += c

		result.Evidence = append(result.Evidence, EvidenceItem{
			Type:    EvidenceFanoutChange,
			Summary: fmt.Sprintf("%s fanout %d -> %d (+%d)", key, baseDeg, headDeg, deg),
			From:    key,
			Value:   float64(headDeg),
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

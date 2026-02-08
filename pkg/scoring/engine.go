package scoring

import (
	"fmt"
	"sort"

	"github.com/toposcope/toposcope/pkg/graph"
)

// Metric is the interface that all scoring metrics implement.
type Metric interface {
	// Key returns the machine-readable metric identifier.
	Key() string
	// Name returns the human-readable metric name.
	Name() string
	// Evaluate computes the metric's score contribution for a given delta.
	Evaluate(delta *graph.Delta, base, head *graph.Snapshot) MetricResult
}

// Engine runs all configured metrics against a delta and produces a ScoreResult.
type Engine struct {
	metrics []Metric
}

// NewEngine creates a scoring engine with the given metrics.
func NewEngine(metrics ...Metric) *Engine {
	return &Engine{metrics: metrics}
}

// Score evaluates all metrics and produces a complete ScoreResult.
func (e *Engine) Score(delta *graph.Delta, base, head *graph.Snapshot) (*ScoreResult, error) {
	if delta == nil {
		return nil, fmt.Errorf("delta is nil")
	}
	if base == nil || head == nil {
		return nil, fmt.Errorf("base and head snapshots are required")
	}

	result := &ScoreResult{
		BaseCommit: base.CommitSHA,
		HeadCommit: head.CommitSHA,
		DeltaStats: DeltaStatsView{
			ImpactedTargets: delta.Stats.ImpactedTargetCount,
			AddedNodes:      delta.Stats.AddedNodeCount,
			RemovedNodes:    delta.Stats.RemovedNodeCount,
			AddedEdges:      delta.Stats.AddedEdgeCount,
			RemovedEdges:    delta.Stats.RemovedEdgeCount,
		},
	}

	// Run each metric
	for _, m := range e.metrics {
		mr := m.Evaluate(delta, base, head)
		result.Breakdown = append(result.Breakdown, mr)
		result.TotalScore += mr.Contribution
	}

	// Clamp score to >= 0
	if result.TotalScore < 0 {
		result.TotalScore = 0
	}

	result.Grade = GradeFromScore(result.TotalScore)
	result.Hotspots = computeHotspots(result.Breakdown)
	result.SuggestedActions = generateSuggestions(result.Breakdown, delta)

	return result, nil
}

// computeHotspots identifies nodes that appear across multiple metrics' evidence.
func computeHotspots(breakdown []MetricResult) []Hotspot {
	// Track which metrics each node appears in and its total contribution
	type nodeInfo struct {
		totalContribution float64
		metricKeys        []string
		reasons           []string
	}
	nodeMap := make(map[string]*nodeInfo)

	for _, mr := range breakdown {
		if mr.Contribution <= 0 {
			continue
		}
		seen := make(map[string]bool)
		for _, ev := range mr.Evidence {
			for _, key := range []string{ev.From, ev.To} {
				if key == "" || seen[key] {
					continue
				}
				seen[key] = true
				if _, ok := nodeMap[key]; !ok {
					nodeMap[key] = &nodeInfo{}
				}
				nodeMap[key].totalContribution += mr.Contribution / float64(len(mr.Evidence))
				nodeMap[key].metricKeys = append(nodeMap[key].metricKeys, mr.Key)
				nodeMap[key].reasons = append(nodeMap[key].reasons, mr.Name)
			}
		}
	}

	// Only include nodes flagged by 2+ metrics
	var hotspots []Hotspot
	for key, info := range nodeMap {
		uniqueMetrics := uniqueStrings(info.metricKeys)
		if len(uniqueMetrics) >= 2 {
			hotspots = append(hotspots, Hotspot{
				NodeKey:           key,
				Reason:            fmt.Sprintf("Flagged by %d metrics: %v", len(uniqueMetrics), uniqueMetrics),
				ScoreContribution: info.totalContribution,
				MetricKeys:        uniqueMetrics,
			})
		}
	}

	sort.Slice(hotspots, func(i, j int) bool {
		return hotspots[i].ScoreContribution > hotspots[j].ScoreContribution
	})

	if len(hotspots) > 10 {
		hotspots = hotspots[:10]
	}

	return hotspots
}

// generateSuggestions produces actionable recommendations based on findings.
func generateSuggestions(breakdown []MetricResult, delta *graph.Delta) []SuggestedAction {
	var actions []SuggestedAction

	for _, mr := range breakdown {
		switch mr.Key {
		case "fanout_increase":
			for _, ev := range mr.Evidence {
				if ev.Value >= 20 && ev.From != "" {
					actions = append(actions, SuggestedAction{
						Title:       fmt.Sprintf("Consider splitting %s", ev.From),
						Description: fmt.Sprintf("This target now has %.0f dependencies. Targets with high fanout become fragile and slow to build.", ev.Value),
						Targets:     []string{ev.From},
						Confidence:  0.7,
						Addresses:   []string{mr.Key},
					})
				}
			}
		case "cross_package_deps":
			// Group added edges by source node
			sourceEdges := make(map[string]int)
			for _, ev := range mr.Evidence {
				if ev.From != "" {
					sourceEdges[ev.From]++
				}
			}
			for source, count := range sourceEdges {
				if count >= 3 {
					actions = append(actions, SuggestedAction{
						Title:       fmt.Sprintf("Extract shared dependency for %s", source),
						Description: fmt.Sprintf("This target added %d cross-package dependencies. Consider extracting a shared library.", count),
						Targets:     []string{source},
						Confidence:  0.5,
						Addresses:   []string{mr.Key},
					})
				}
			}
		case "centrality_penalty":
			for _, ev := range mr.Evidence {
				if ev.To != "" && ev.Value >= 100 {
					actions = append(actions, SuggestedAction{
						Title:       fmt.Sprintf("Avoid direct dependency on %s", ev.To),
						Description: fmt.Sprintf("This target has %.0f reverse dependencies. Consider depending on a narrower interface.", ev.Value),
						Targets:     []string{ev.To},
						Confidence:  0.5,
						Addresses:   []string{mr.Key},
					})
				}
			}
		}
	}

	if len(actions) > 5 {
		actions = actions[:5]
	}

	return actions
}

func uniqueStrings(ss []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

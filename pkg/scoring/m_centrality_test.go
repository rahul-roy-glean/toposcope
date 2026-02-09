package scoring_test

import (
	"math"
	"testing"

	"github.com/toposcope/toposcope/pkg/graph"
	"github.com/toposcope/toposcope/pkg/scoring"
)

func TestCentralityMetric_Basic(t *testing.T) {
	// Create a base where //lib:core has high in-degree
	baseNodes := map[string]*graph.Node{
		"//lib:core": {Key: "//lib:core", Package: "//lib"},
	}
	var baseEdges []graph.Edge
	for i := 0; i < 60; i++ {
		key := "//dep" + string(rune('a'+i%26)) + string(rune('0'+i/26)) + ":lib"
		baseNodes[key] = &graph.Node{Key: key, Package: "//dep"}
		baseEdges = append(baseEdges, graph.Edge{From: key, To: "//lib:core", Type: "COMPILE"})
	}

	base := &graph.Snapshot{Nodes: baseNodes, Edges: baseEdges}
	head := &graph.Snapshot{
		Nodes: map[string]*graph.Node{
			"//app:new":  {Key: "//app:new", Package: "//app"},
			"//lib:core": {Key: "//lib:core", Package: "//lib"},
		},
	}
	delta := &graph.Delta{
		AddedEdges: []graph.Edge{
			{From: "//app:new", To: "//lib:core", Type: "COMPILE"},
		},
	}

	m := &scoring.CentralityMetric{
		Weight:      0.7,
		MinInDegree: 50,
	}

	result := m.Evaluate(delta, base, head)

	if result.Key != "centrality_penalty" {
		t.Errorf("expected key centrality_penalty, got %s", result.Key)
	}

	expected := 0.7 * math.Log2(1+60.0)
	if math.Abs(result.Contribution-expected) > 0.01 {
		t.Errorf("expected contribution ~%f, got %f", expected, result.Contribution)
	}
	if len(result.Evidence) != 1 {
		t.Errorf("expected 1 evidence item, got %d", len(result.Evidence))
	}
}

func TestCentralityMetric_BelowMinInDegree(t *testing.T) {
	base := &graph.Snapshot{
		Nodes: map[string]*graph.Node{
			"//lib:core": {Key: "//lib:core", Package: "//lib"},
			"//app:a":    {Key: "//app:a", Package: "//app"},
		},
		Edges: []graph.Edge{
			{From: "//app:a", To: "//lib:core", Type: "COMPILE"},
		},
	}
	head := &graph.Snapshot{
		Nodes: map[string]*graph.Node{
			"//lib:core": {Key: "//lib:core", Package: "//lib"},
		},
	}
	delta := &graph.Delta{
		AddedEdges: []graph.Edge{
			{From: "//app:new", To: "//lib:core", Type: "COMPILE"},
		},
	}

	m := &scoring.CentralityMetric{
		Weight:      0.7,
		MinInDegree: 50,
	}

	result := m.Evaluate(delta, base, head)
	if result.Contribution != 0 {
		t.Errorf("expected zero contribution below min in-degree, got %f", result.Contribution)
	}
}

func TestCentralityMetric_EmptyBase(t *testing.T) {
	base := &graph.Snapshot{
		Nodes: map[string]*graph.Node{},
	}
	head := &graph.Snapshot{
		Nodes: map[string]*graph.Node{
			"//app:lib": {Key: "//app:lib", Package: "//app"},
		},
	}
	delta := &graph.Delta{
		AddedEdges: []graph.Edge{
			{From: "//app:lib", To: "//lib:core", Type: "COMPILE"},
		},
	}

	m := &scoring.CentralityMetric{
		Weight:      0.7,
		MinInDegree: 50,
	}

	result := m.Evaluate(delta, base, head)
	if result.Contribution != 0 {
		t.Errorf("expected zero contribution for empty base, got %f", result.Contribution)
	}
	if result.Severity != scoring.SeverityInfo {
		t.Errorf("expected INFO severity for empty base, got %s", result.Severity)
	}
}

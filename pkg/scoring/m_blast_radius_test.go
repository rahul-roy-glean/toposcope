package scoring_test

import (
	"math"
	"testing"

	"github.com/toposcope/toposcope/pkg/graph"
	"github.com/toposcope/toposcope/pkg/scoring"
)

func TestBlastRadiusMetric_Basic(t *testing.T) {
	// Create base with some in-degrees
	base := &graph.Snapshot{
		Nodes: map[string]*graph.Node{
			"//app:lib":  {Key: "//app:lib", Package: "//app"},
			"//lib:core": {Key: "//lib:core", Package: "//lib"},
			"//dep:a":    {Key: "//dep:a", Package: "//dep"},
			"//dep:b":    {Key: "//dep:b", Package: "//dep"},
		},
		Edges: []graph.Edge{
			{From: "//dep:a", To: "//lib:core", Type: "COMPILE"},
			{From: "//dep:b", To: "//lib:core", Type: "COMPILE"},
			{From: "//dep:a", To: "//app:lib", Type: "COMPILE"},
		},
	}
	head := &graph.Snapshot{
		Nodes: map[string]*graph.Node{
			"//app:lib":  {Key: "//app:lib", Package: "//app"},
			"//lib:core": {Key: "//lib:core", Package: "//lib"},
		},
	}
	delta := &graph.Delta{
		AddedEdges: []graph.Edge{
			{From: "//app:lib", To: "//lib:core", Type: "COMPILE"},
		},
	}

	m := &scoring.BlastRadiusMetric{
		Weight:          2.0,
		MaxContribution: 15.0,
	}

	result := m.Evaluate(delta, base, head)

	if result.Key != "blast_radius" {
		t.Errorf("expected key blast_radius, got %s", result.Key)
	}

	// Affected nodes: //app:lib (in-degree=1), //lib:core (in-degree=2)
	// blastRadius = 1 + 2 = 3
	// contribution = 2.0 * log2(1+3) = 2.0 * 2.0 = 4.0
	expected := 2.0 * math.Log2(4.0)
	if math.Abs(result.Contribution-expected) > 0.01 {
		t.Errorf("expected contribution ~%f, got %f", expected, result.Contribution)
	}
}

func TestBlastRadiusMetric_MaxContribution(t *testing.T) {
	// Create base where affected nodes have very high in-degrees
	baseNodes := map[string]*graph.Node{
		"//lib:core": {Key: "//lib:core", Package: "//lib"},
	}
	var baseEdges []graph.Edge
	for i := 0; i < 1000; i++ {
		key := "//dep" + string(rune('a'+i%26)) + string(rune('0'+i/26)) + ":lib"
		baseNodes[key] = &graph.Node{Key: key}
		baseEdges = append(baseEdges, graph.Edge{From: key, To: "//lib:core", Type: "COMPILE"})
	}

	base := &graph.Snapshot{Nodes: baseNodes, Edges: baseEdges}
	head := &graph.Snapshot{Nodes: map[string]*graph.Node{}}
	delta := &graph.Delta{
		AddedEdges: []graph.Edge{
			{From: "//new:lib", To: "//lib:core", Type: "COMPILE"},
		},
	}

	m := &scoring.BlastRadiusMetric{
		Weight:          2.0,
		MaxContribution: 15.0,
	}

	result := m.Evaluate(delta, base, head)
	if result.Contribution > 15.0 {
		t.Errorf("expected contribution capped at 15.0, got %f", result.Contribution)
	}
}

func TestBlastRadiusMetric_EmptyDelta(t *testing.T) {
	base := &graph.Snapshot{
		Nodes: map[string]*graph.Node{
			"//app:lib": {Key: "//app:lib", Package: "//app"},
		},
	}
	head := &graph.Snapshot{
		Nodes: map[string]*graph.Node{},
	}
	delta := &graph.Delta{}

	m := &scoring.BlastRadiusMetric{
		Weight:          2.0,
		MaxContribution: 15.0,
	}

	result := m.Evaluate(delta, base, head)
	if result.Contribution != 0 {
		t.Errorf("expected zero contribution for empty delta, got %f", result.Contribution)
	}
}

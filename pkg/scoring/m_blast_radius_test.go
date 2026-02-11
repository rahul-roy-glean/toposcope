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

func TestBlastRadiusMetric_TestNodeDiscount(t *testing.T) {
	// Test nodes should contribute at 0.3x their in-degree
	base := &graph.Snapshot{
		Nodes: map[string]*graph.Node{
			"//app:lib":  {Key: "//app:lib", Package: "//app"},
			"//app:test": {Key: "//app:test", Package: "//app", IsTest: true},
		},
		Edges: []graph.Edge{
			// //app:lib has in-degree 0
			// //app:test has in-degree 0
		},
	}

	// Give them in-degrees via other nodes depending on them
	for i := 0; i < 10; i++ {
		key := "//dep" + string(rune('a'+i)) + ":lib"
		base.Nodes[key] = &graph.Node{Key: key, Package: "//dep"}
		base.Edges = append(base.Edges, graph.Edge{From: key, To: "//app:lib", Type: "COMPILE"})
	}
	for i := 0; i < 20; i++ {
		key := "//tdep" + string(rune('a'+i)) + ":lib"
		base.Nodes[key] = &graph.Node{Key: key, Package: "//tdep"}
		base.Edges = append(base.Edges, graph.Edge{From: key, To: "//app:test", Type: "COMPILE"})
	}

	head := &graph.Snapshot{
		Nodes: map[string]*graph.Node{
			"//app:lib":  {Key: "//app:lib", Package: "//app"},
			"//app:test": {Key: "//app:test", Package: "//app", IsTest: true},
		},
	}
	delta := &graph.Delta{
		AddedEdges: []graph.Edge{
			{From: "//app:lib", To: "//app:test", Type: "COMPILE"},
		},
	}

	m := &scoring.BlastRadiusMetric{
		Weight:          2.0,
		MaxContribution: 15.0,
	}

	result := m.Evaluate(delta, base, head)

	// Affected: //app:lib (in-degree=10, prod -> weight 1.0) + //app:test (in-degree=20, test -> weight 0.3)
	// blastRadius = 10*1.0 + 20*0.3 = 10 + 6 = 16
	expectedBlast := 10.0*1.0 + 20.0*0.3
	expected := 2.0 * math.Log2(1+expectedBlast)
	if math.Abs(result.Contribution-expected) > 0.01 {
		t.Errorf("expected contribution ~%f, got %f", expected, result.Contribution)
	}

	// Compare with what it would be without discounting (should be higher)
	nodiscountBlast := 10.0 + 20.0
	noDiscount := 2.0 * math.Log2(1+nodiscountBlast)
	if result.Contribution >= noDiscount {
		t.Errorf("test discount should reduce contribution: got %f, undiscounted would be %f", result.Contribution, noDiscount)
	}
}

package scoring_test

import (
	"testing"

	"github.com/toposcope/toposcope/pkg/graph"
	"github.com/toposcope/toposcope/pkg/scoring"
)

func TestFanoutMetric_Basic(t *testing.T) {
	base := &graph.Snapshot{
		Nodes: map[string]*graph.Node{
			"//app:lib": {Key: "//app:lib", Package: "//app"},
			"//dep1:lib": {Key: "//dep1:lib", Package: "//dep1"},
			"//dep2:lib": {Key: "//dep2:lib", Package: "//dep2"},
			"//dep3:lib": {Key: "//dep3:lib", Package: "//dep3"},
		},
		Edges: []graph.Edge{
			{From: "//app:lib", To: "//dep1:lib", Type: "COMPILE"},
			{From: "//app:lib", To: "//dep2:lib", Type: "COMPILE"},
			{From: "//app:lib", To: "//dep3:lib", Type: "COMPILE"},
		},
	}

	// Build head with many more deps to exceed MinThreshold
	headNodes := map[string]*graph.Node{
		"//app:lib": {Key: "//app:lib", Package: "//app"},
	}
	var headEdges []graph.Edge
	for i := 0; i < 15; i++ {
		key := "//dep" + string(rune('a'+i)) + ":lib"
		headNodes[key] = &graph.Node{Key: key, Package: "//dep" + string(rune('a'+i))}
		headEdges = append(headEdges, graph.Edge{From: "//app:lib", To: key, Type: "COMPILE"})
	}

	head := &graph.Snapshot{Nodes: headNodes, Edges: headEdges}
	delta := &graph.Delta{} // Delta not used directly by fanout

	m := &scoring.FanoutMetric{
		Weight:       0.5,
		CapPerNode:   10,
		MinThreshold: 10,
	}

	result := m.Evaluate(delta, base, head)

	if result.Key != "fanout_increase" {
		t.Errorf("expected key fanout_increase, got %s", result.Key)
	}
	// headDeg=15, baseDeg=3, delta=12, but capped at 10 -> 0.5*10=5.0
	if result.Contribution != 5.0 {
		t.Errorf("expected contribution 5.0, got %f", result.Contribution)
	}
}

func TestFanoutMetric_BelowThreshold(t *testing.T) {
	base := &graph.Snapshot{
		Nodes: map[string]*graph.Node{
			"//app:lib": {Key: "//app:lib", Package: "//app"},
		},
	}
	head := &graph.Snapshot{
		Nodes: map[string]*graph.Node{
			"//app:lib":  {Key: "//app:lib", Package: "//app"},
			"//dep1:lib": {Key: "//dep1:lib", Package: "//dep1"},
		},
		Edges: []graph.Edge{
			{From: "//app:lib", To: "//dep1:lib", Type: "COMPILE"},
		},
	}
	delta := &graph.Delta{}

	m := &scoring.FanoutMetric{
		Weight:       0.5,
		CapPerNode:   10,
		MinThreshold: 10,
	}

	result := m.Evaluate(delta, base, head)
	if result.Contribution != 0 {
		t.Errorf("expected zero contribution below threshold, got %f", result.Contribution)
	}
}

func TestFanoutMetric_SkipsTestTargets(t *testing.T) {
	base := &graph.Snapshot{
		Nodes: map[string]*graph.Node{},
	}

	headNodes := map[string]*graph.Node{
		"//app:lib_test": {Key: "//app:lib_test", Package: "//app", IsTest: true},
	}
	var headEdges []graph.Edge
	for i := 0; i < 15; i++ {
		key := "//dep" + string(rune('a'+i)) + ":lib"
		headNodes[key] = &graph.Node{Key: key, Package: "//dep" + string(rune('a'+i))}
		headEdges = append(headEdges, graph.Edge{From: "//app:lib_test", To: key, Type: "COMPILE"})
	}

	head := &graph.Snapshot{Nodes: headNodes, Edges: headEdges}
	delta := &graph.Delta{}

	m := &scoring.FanoutMetric{
		Weight:       0.5,
		CapPerNode:   10,
		MinThreshold: 10,
	}

	result := m.Evaluate(delta, base, head)
	if result.Contribution != 0 {
		t.Errorf("expected zero contribution for test target, got %f", result.Contribution)
	}
}

func TestFanoutMetric_NoIncrease(t *testing.T) {
	nodes := map[string]*graph.Node{
		"//app:lib":  {Key: "//app:lib", Package: "//app"},
		"//dep1:lib": {Key: "//dep1:lib", Package: "//dep1"},
	}
	edges := []graph.Edge{
		{From: "//app:lib", To: "//dep1:lib", Type: "COMPILE"},
	}

	base := &graph.Snapshot{Nodes: nodes, Edges: edges}
	head := &graph.Snapshot{Nodes: nodes, Edges: edges}
	delta := &graph.Delta{}

	m := &scoring.FanoutMetric{
		Weight:       0.5,
		CapPerNode:   10,
		MinThreshold: 0, // low threshold to test
	}

	result := m.Evaluate(delta, base, head)
	if result.Contribution != 0 {
		t.Errorf("expected zero for no fanout increase, got %f", result.Contribution)
	}
}

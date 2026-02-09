package scoring_test

import (
	"testing"

	"github.com/toposcope/toposcope/pkg/graph"
	"github.com/toposcope/toposcope/pkg/scoring"
)

func TestCreditsMetric_RemovedCrossBoundaryEdge(t *testing.T) {
	base := &graph.Snapshot{
		Nodes: map[string]*graph.Node{
			"//app/auth:handler": {Key: "//app/auth:handler", Package: "//app/auth"},
			"//lib/old:lib":      {Key: "//lib/old:lib", Package: "//lib/old"},
		},
		Edges: []graph.Edge{
			{From: "//app/auth:handler", To: "//lib/old:lib", Type: "COMPILE"},
		},
	}
	head := &graph.Snapshot{
		Nodes: map[string]*graph.Node{
			"//app/auth:handler": {Key: "//app/auth:handler", Package: "//app/auth"},
		},
	}
	delta := &graph.Delta{
		RemovedEdges: []graph.Edge{
			{From: "//app/auth:handler", To: "//lib/old:lib", Type: "COMPILE"},
		},
	}

	m := &scoring.CreditsMetric{
		PerRemovedCrossBoundaryEdge: -0.5,
		MaxCreditTotal:              -5.0,
		PerFanoutReduction:          -0.3,
		FanoutMaxCredit:             -3.0,
	}

	result := m.Evaluate(delta, base, head)

	if result.Key != "cleanup_credits" {
		t.Errorf("expected key cleanup_credits, got %s", result.Key)
	}
	// -0.5 for removed cross-boundary edge + -0.3 for fanout reduction (1 -> 0) = -0.8
	if result.Contribution != -0.8 {
		t.Errorf("expected contribution -0.8, got %f", result.Contribution)
	}
}

func TestCreditsMetric_AntiGaming(t *testing.T) {
	// Edge does not exist in base - should not get credit
	base := &graph.Snapshot{
		Nodes: map[string]*graph.Node{
			"//app/auth:handler": {Key: "//app/auth:handler", Package: "//app/auth"},
			"//lib/old:lib":      {Key: "//lib/old:lib", Package: "//lib/old"},
		},
		Edges: []graph.Edge{}, // Edge NOT in base
	}
	head := &graph.Snapshot{
		Nodes: map[string]*graph.Node{
			"//app/auth:handler": {Key: "//app/auth:handler", Package: "//app/auth"},
		},
	}
	delta := &graph.Delta{
		RemovedEdges: []graph.Edge{
			{From: "//app/auth:handler", To: "//lib/old:lib", Type: "COMPILE"},
		},
	}

	m := &scoring.CreditsMetric{
		PerRemovedCrossBoundaryEdge: -0.5,
		MaxCreditTotal:              -5.0,
		PerFanoutReduction:          -0.3,
		FanoutMaxCredit:             -3.0,
	}

	result := m.Evaluate(delta, base, head)
	if result.Contribution != 0 {
		t.Errorf("expected zero contribution for anti-gaming case, got %f", result.Contribution)
	}
}

func TestCreditsMetric_FanoutReduction(t *testing.T) {
	base := &graph.Snapshot{
		Nodes: map[string]*graph.Node{
			"//app:lib":  {Key: "//app:lib", Package: "//app"},
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

	m := &scoring.CreditsMetric{
		PerRemovedCrossBoundaryEdge: -0.5,
		MaxCreditTotal:              -5.0,
		PerFanoutReduction:          -0.3,
		FanoutMaxCredit:             -3.0,
	}

	result := m.Evaluate(delta, base, head)
	// Fanout reduced by 2: -0.3 * 2 = -0.6
	if result.Contribution != -0.6 {
		t.Errorf("expected contribution -0.6, got %f", result.Contribution)
	}
}

func TestCreditsMetric_SameBoundaryNoEdgeCredit(t *testing.T) {
	base := &graph.Snapshot{
		Nodes: map[string]*graph.Node{
			"//app/auth:handler":    {Key: "//app/auth:handler", Package: "//app/auth"},
			"//app/billing:service": {Key: "//app/billing:service", Package: "//app/billing"},
		},
		Edges: []graph.Edge{
			{From: "//app/auth:handler", To: "//app/billing:service", Type: "COMPILE"},
		},
	}
	head := &graph.Snapshot{
		Nodes: map[string]*graph.Node{
			"//app/auth:handler": {Key: "//app/auth:handler", Package: "//app/auth"},
		},
	}
	delta := &graph.Delta{
		RemovedEdges: []graph.Edge{
			{From: "//app/auth:handler", To: "//app/billing:service", Type: "COMPILE"},
		},
	}

	m := &scoring.CreditsMetric{
		PerRemovedCrossBoundaryEdge: -0.5,
		MaxCreditTotal:              -5.0,
		PerFanoutReduction:          -0.3,
		FanoutMaxCredit:             -3.0,
	}

	result := m.Evaluate(delta, base, head)
	// Same boundary (app -> app) - only fanout credit, not edge credit
	// Fanout reduction: base has 1 edge from //app/auth:handler, head has 0 -> reduction = 1
	// fanout credit = -0.3 * 1 = -0.3
	if result.Contribution != -0.3 {
		t.Errorf("expected contribution -0.3 (fanout only), got %f", result.Contribution)
	}
}

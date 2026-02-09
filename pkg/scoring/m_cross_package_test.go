package scoring_test

import (
	"testing"

	"github.com/toposcope/toposcope/pkg/graph"
	"github.com/toposcope/toposcope/pkg/scoring"
)

func TestCrossPackageMetric_Basic(t *testing.T) {
	base := &graph.Snapshot{
		Nodes: map[string]*graph.Node{
			"//app/auth:handler": {Key: "//app/auth:handler", Package: "//app/auth"},
			"//lib/session:lib":  {Key: "//lib/session:lib", Package: "//lib/session"},
		},
	}
	head := &graph.Snapshot{
		Nodes: map[string]*graph.Node{
			"//app/auth:handler": {Key: "//app/auth:handler", Package: "//app/auth"},
			"//lib/session:lib":  {Key: "//lib/session:lib", Package: "//lib/session"},
		},
	}
	delta := &graph.Delta{
		AddedEdges: []graph.Edge{
			{From: "//app/auth:handler", To: "//lib/session:lib", Type: "COMPILE"},
		},
	}

	m := &scoring.CrossPackageMetric{
		IntraBoundaryWeight: 0.5,
		CrossBoundaryWeight: 1.5,
	}

	result := m.Evaluate(delta, base, head)

	if result.Key != "cross_package_deps" {
		t.Errorf("expected key cross_package_deps, got %s", result.Key)
	}
	// app -> lib is cross-boundary
	if result.Contribution != 1.5 {
		t.Errorf("expected contribution 1.5 for cross-boundary edge, got %f", result.Contribution)
	}
	if len(result.Evidence) != 1 {
		t.Errorf("expected 1 evidence item, got %d", len(result.Evidence))
	}
}

func TestCrossPackageMetric_IntraBoundary(t *testing.T) {
	base := &graph.Snapshot{
		Nodes: map[string]*graph.Node{
			"//app/auth:handler":   {Key: "//app/auth:handler", Package: "//app/auth"},
			"//app/billing:service": {Key: "//app/billing:service", Package: "//app/billing"},
		},
	}
	head := &graph.Snapshot{
		Nodes: map[string]*graph.Node{
			"//app/auth:handler":   {Key: "//app/auth:handler", Package: "//app/auth"},
			"//app/billing:service": {Key: "//app/billing:service", Package: "//app/billing"},
		},
	}
	delta := &graph.Delta{
		AddedEdges: []graph.Edge{
			{From: "//app/auth:handler", To: "//app/billing:service", Type: "COMPILE"},
		},
	}

	m := &scoring.CrossPackageMetric{
		IntraBoundaryWeight: 0.5,
		CrossBoundaryWeight: 1.5,
	}

	result := m.Evaluate(delta, base, head)
	// app -> app is intra-boundary cross-package
	if result.Contribution != 0.5 {
		t.Errorf("expected contribution 0.5 for intra-boundary edge, got %f", result.Contribution)
	}
}

func TestCrossPackageMetric_SkipsTestSource(t *testing.T) {
	base := &graph.Snapshot{
		Nodes: map[string]*graph.Node{},
	}
	head := &graph.Snapshot{
		Nodes: map[string]*graph.Node{
			"//app/auth:handler_test": {Key: "//app/auth:handler_test", Package: "//app/auth", IsTest: true},
			"//lib/session:lib":       {Key: "//lib/session:lib", Package: "//lib/session"},
		},
	}
	delta := &graph.Delta{
		AddedEdges: []graph.Edge{
			{From: "//app/auth:handler_test", To: "//lib/session:lib", Type: "COMPILE"},
		},
	}

	m := &scoring.CrossPackageMetric{
		IntraBoundaryWeight: 0.5,
		CrossBoundaryWeight: 1.5,
	}

	result := m.Evaluate(delta, base, head)
	if result.Contribution != 0 {
		t.Errorf("expected zero contribution for test source, got %f", result.Contribution)
	}
}

func TestCrossPackageMetric_SkipsExternalTarget(t *testing.T) {
	base := &graph.Snapshot{
		Nodes: map[string]*graph.Node{},
	}
	head := &graph.Snapshot{
		Nodes: map[string]*graph.Node{
			"//app/auth:handler": {Key: "//app/auth:handler", Package: "//app/auth"},
			"@com_google//:lib":  {Key: "@com_google//:lib", Package: "@com_google", IsExternal: true},
		},
	}
	delta := &graph.Delta{
		AddedEdges: []graph.Edge{
			{From: "//app/auth:handler", To: "@com_google//:lib", Type: "COMPILE"},
		},
	}

	m := &scoring.CrossPackageMetric{
		IntraBoundaryWeight: 0.5,
		CrossBoundaryWeight: 1.5,
	}

	result := m.Evaluate(delta, base, head)
	if result.Contribution != 0 {
		t.Errorf("expected zero contribution for external target, got %f", result.Contribution)
	}
}

func TestCrossPackageMetric_SkipsProtoTarget(t *testing.T) {
	base := &graph.Snapshot{
		Nodes: map[string]*graph.Node{},
	}
	head := &graph.Snapshot{
		Nodes: map[string]*graph.Node{
			"//app/auth:handler":    {Key: "//app/auth:handler", Package: "//app/auth"},
			"//proto/common:types_go": {Key: "//proto/common:types_go", Kind: "go_proto_library", Package: "//proto/common"},
		},
	}
	delta := &graph.Delta{
		AddedEdges: []graph.Edge{
			{From: "//app/auth:handler", To: "//proto/common:types_go", Type: "COMPILE"},
		},
	}

	m := &scoring.CrossPackageMetric{
		IntraBoundaryWeight: 0.5,
		CrossBoundaryWeight: 1.5,
	}

	result := m.Evaluate(delta, base, head)
	if result.Contribution != 0 {
		t.Errorf("expected zero contribution for proto target, got %f", result.Contribution)
	}
}

func TestCrossPackageMetric_SamePackageNoScore(t *testing.T) {
	base := &graph.Snapshot{
		Nodes: map[string]*graph.Node{},
	}
	head := &graph.Snapshot{
		Nodes: map[string]*graph.Node{
			"//app/auth:handler":    {Key: "//app/auth:handler", Package: "//app/auth"},
			"//app/auth:middleware": {Key: "//app/auth:middleware", Package: "//app/auth"},
		},
	}
	delta := &graph.Delta{
		AddedEdges: []graph.Edge{
			{From: "//app/auth:handler", To: "//app/auth:middleware", Type: "COMPILE"},
		},
	}

	m := &scoring.CrossPackageMetric{
		IntraBoundaryWeight: 0.5,
		CrossBoundaryWeight: 1.5,
	}

	result := m.Evaluate(delta, base, head)
	if result.Contribution != 0 {
		t.Errorf("expected zero contribution for same-package edge, got %f", result.Contribution)
	}
}

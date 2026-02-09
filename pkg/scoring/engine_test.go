package scoring_test

import (
	"testing"

	"github.com/toposcope/toposcope/pkg/graph"
	"github.com/toposcope/toposcope/pkg/scoring"
)

func loadFixtures(t *testing.T) (*graph.Snapshot, *graph.Snapshot, *graph.Delta) {
	t.Helper()
	base, err := graph.LoadSnapshot("../../testdata/snapshot_base.json")
	if err != nil {
		t.Fatalf("loading base snapshot: %v", err)
	}
	head, err := graph.LoadSnapshot("../../testdata/snapshot_head.json")
	if err != nil {
		t.Fatalf("loading head snapshot: %v", err)
	}
	delta := graph.ComputeDelta(base, head)
	return base, head, delta
}

func TestEngineScoreWithFixtures(t *testing.T) {
	base, head, delta := loadFixtures(t)

	metrics := scoring.DefaultMetrics()
	engine := scoring.NewEngine(metrics...)

	result, err := engine.Score(delta, base, head)
	if err != nil {
		t.Fatalf("Score() error: %v", err)
	}

	// Basic checks
	if result.Grade == "" {
		t.Error("expected a non-empty grade")
	}
	if result.TotalScore < 0 {
		t.Errorf("expected non-negative total score, got %f", result.TotalScore)
	}
	if len(result.Breakdown) != len(metrics) {
		t.Errorf("expected %d breakdown entries, got %d", len(metrics), len(result.Breakdown))
	}

	// Check that cross-package metric found findings
	var crossPkg *scoring.MetricResult
	for i := range result.Breakdown {
		if result.Breakdown[i].Key == "cross_package_deps" {
			crossPkg = &result.Breakdown[i]
			break
		}
	}
	if crossPkg == nil {
		t.Fatal("expected cross_package_deps metric in breakdown")
	}
	if crossPkg.Contribution <= 0 {
		t.Errorf("expected positive contribution for cross_package_deps, got %f", crossPkg.Contribution)
	}

	// Check that evidence includes cross-boundary edges from //app/auth -> //lib/session
	foundCrossEdge := false
	for _, ev := range crossPkg.Evidence {
		if ev.From == "//app/auth:handler" {
			foundCrossEdge = true
			break
		}
	}
	if !foundCrossEdge {
		t.Error("expected evidence for //app/auth:handler in cross_package_deps")
	}

	// Check commits
	if result.BaseCommit != "abc123f" {
		t.Errorf("expected base commit abc123f, got %s", result.BaseCommit)
	}
	if result.HeadCommit != "def456a" {
		t.Errorf("expected head commit def456a, got %s", result.HeadCommit)
	}
}

func TestEngineScoreNilDelta(t *testing.T) {
	base := &graph.Snapshot{Nodes: map[string]*graph.Node{}}
	head := &graph.Snapshot{Nodes: map[string]*graph.Node{}}

	engine := scoring.NewEngine()
	_, err := engine.Score(nil, base, head)
	if err == nil {
		t.Error("expected error for nil delta")
	}
}

func TestEngineScoreNilSnapshots(t *testing.T) {
	delta := &graph.Delta{}
	engine := scoring.NewEngine()

	_, err := engine.Score(delta, nil, &graph.Snapshot{})
	if err == nil {
		t.Error("expected error for nil base snapshot")
	}

	_, err = engine.Score(delta, &graph.Snapshot{}, nil)
	if err == nil {
		t.Error("expected error for nil head snapshot")
	}
}

func TestEngineScoreEmptyDelta(t *testing.T) {
	base := &graph.Snapshot{
		Nodes: map[string]*graph.Node{
			"//app:lib": {Key: "//app:lib", Package: "//app"},
		},
	}
	head := &graph.Snapshot{
		Nodes: map[string]*graph.Node{
			"//app:lib": {Key: "//app:lib", Package: "//app"},
		},
	}
	delta := &graph.Delta{}

	metrics := scoring.DefaultMetrics()
	engine := scoring.NewEngine(metrics...)

	result, err := engine.Score(delta, base, head)
	if err != nil {
		t.Fatalf("Score() error: %v", err)
	}

	if result.TotalScore != 0 {
		t.Errorf("expected zero score for empty delta, got %f", result.TotalScore)
	}
	if result.Grade != "A" {
		t.Errorf("expected grade A for zero score, got %s", result.Grade)
	}
}

package graph

import (
	"path/filepath"
	"runtime"
	"testing"
)

func testdataPath(name string) string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "testdata", name)
}

func TestComputeDelta_Testdata(t *testing.T) {
	base, err := LoadSnapshot(testdataPath("snapshot_base.json"))
	if err != nil {
		t.Fatalf("loading base: %v", err)
	}
	head, err := LoadSnapshot(testdataPath("snapshot_head.json"))
	if err != nil {
		t.Fatalf("loading head: %v", err)
	}

	delta := ComputeDelta(base, head)

	// Head has 15 nodes, base has 12. Added: types, store, validate = 3 new nodes.
	if delta.Stats.AddedNodeCount != 3 {
		t.Errorf("AddedNodeCount = %d, want 3", delta.Stats.AddedNodeCount)
	}
	// No nodes were removed
	if delta.Stats.RemovedNodeCount != 0 {
		t.Errorf("RemovedNodeCount = %d, want 0", delta.Stats.RemovedNodeCount)
	}

	// Head has 24 edges, base has 14. Let's verify structurally.
	// Added edges in head not in base:
	// handler -> session:internal, handler -> session:types, handler -> session:store,
	// handler -> session:validate, handler -> config:lib,
	// middleware -> session:internal, middleware -> session:validate,
	// session:store -> session:internal, session:types -> proto/common:types_go,
	// session:validate -> session:types
	// = 10 new edges
	if delta.Stats.AddedEdgeCount != 10 {
		t.Errorf("AddedEdgeCount = %d, want 10", delta.Stats.AddedEdgeCount)
	}

	// No edges were removed
	if delta.Stats.RemovedEdgeCount != 0 {
		t.Errorf("RemovedEdgeCount = %d, want 0", delta.Stats.RemovedEdgeCount)
	}

	// Verify specific added nodes
	addedKeys := make(map[string]bool)
	for _, n := range delta.AddedNodes {
		addedKeys[n.Key] = true
	}
	for _, expected := range []string{"//lib/session:types", "//lib/session:store", "//lib/session:validate"} {
		if !addedKeys[expected] {
			t.Errorf("expected added node %s not found", expected)
		}
	}
}

func TestComputeDelta_Empty(t *testing.T) {
	snap := &Snapshot{
		ID:    "test",
		Nodes: map[string]*Node{},
	}

	delta := ComputeDelta(snap, snap)
	if delta.Stats.AddedNodeCount != 0 {
		t.Errorf("AddedNodeCount = %d, want 0", delta.Stats.AddedNodeCount)
	}
	if delta.Stats.RemovedNodeCount != 0 {
		t.Errorf("RemovedNodeCount = %d, want 0", delta.Stats.RemovedNodeCount)
	}
}

func TestComputeDelta_AllRemoved(t *testing.T) {
	base := &Snapshot{
		ID: "base",
		Nodes: map[string]*Node{
			"//a:a": {Key: "//a:a", Kind: "go_library", Package: "//a"},
			"//b:b": {Key: "//b:b", Kind: "go_library", Package: "//b"},
		},
		Edges: []Edge{
			{From: "//a:a", To: "//b:b", Type: "COMPILE"},
		},
	}
	head := &Snapshot{
		ID:    "head",
		Nodes: map[string]*Node{},
	}

	delta := ComputeDelta(base, head)
	if delta.Stats.RemovedNodeCount != 2 {
		t.Errorf("RemovedNodeCount = %d, want 2", delta.Stats.RemovedNodeCount)
	}
	if delta.Stats.RemovedEdgeCount != 1 {
		t.Errorf("RemovedEdgeCount = %d, want 1", delta.Stats.RemovedEdgeCount)
	}
}

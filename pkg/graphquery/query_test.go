package graphquery

import (
	"testing"

	"github.com/toposcope/toposcope/pkg/graph"
)

func testSnapshot() *graph.Snapshot {
	return &graph.Snapshot{
		Nodes: map[string]*graph.Node{
			"//a:lib":       {Key: "//a:lib", Kind: "go_library", Package: "//a"},
			"//a:test":      {Key: "//a:test", Kind: "go_test", Package: "//a", IsTest: true},
			"//b:lib":       {Key: "//b:lib", Kind: "go_library", Package: "//b"},
			"//c:lib":       {Key: "//c:lib", Kind: "go_library", Package: "//c"},
			"//d:lib":       {Key: "//d:lib", Kind: "go_library", Package: "//d"},
			"@ext//e:lib":   {Key: "@ext//e:lib", Kind: "java_library", Package: "@ext//e", IsExternal: true},
			"//f:lib":       {Key: "//f:lib", Kind: "go_library", Package: "//f"},
			"//f:sub/inner": {Key: "//f:sub/inner", Kind: "go_library", Package: "//f"},
		},
		Edges: []graph.Edge{
			{From: "//a:lib", To: "//b:lib", Type: "COMPILE"},
			{From: "//a:test", To: "//a:lib", Type: "COMPILE"},
			{From: "//b:lib", To: "//c:lib", Type: "COMPILE"},
			{From: "//c:lib", To: "//d:lib", Type: "COMPILE"},
			{From: "//d:lib", To: "@ext//e:lib", Type: "COMPILE"},
			{From: "//f:lib", To: "//a:lib", Type: "COMPILE"},
			{From: "//f:sub/inner", To: "//f:lib", Type: "COMPILE"},
		},
	}
}

func TestExtractSubgraph(t *testing.T) {
	snap := testSnapshot()

	t.Run("single root depth 1", func(t *testing.T) {
		result := ExtractSubgraph(snap, []string{"//b:lib"}, 1)
		if _, ok := result.Nodes["//b:lib"]; !ok {
			t.Error("expected root node //b:lib in result")
		}
		if _, ok := result.Nodes["//a:lib"]; !ok {
			t.Error("expected //a:lib (reverse dep) in result")
		}
		if _, ok := result.Nodes["//c:lib"]; !ok {
			t.Error("expected //c:lib (forward dep) in result")
		}
		if _, ok := result.Nodes["//d:lib"]; ok {
			t.Error("did not expect //d:lib at depth 1")
		}
	})

	t.Run("prefix matching", func(t *testing.T) {
		result := ExtractSubgraph(snap, []string{"//f"}, 0)
		if len(result.Nodes) != 2 {
			t.Errorf("expected 2 nodes matching //f prefix, got %d", len(result.Nodes))
		}
	})
}

func TestCapGraph(t *testing.T) {
	snap := testSnapshot()

	t.Run("under limit", func(t *testing.T) {
		result := CapGraph(snap, 100)
		if len(result.Nodes) != len(snap.Nodes) {
			t.Errorf("expected all %d nodes, got %d", len(snap.Nodes), len(result.Nodes))
		}
	})

	t.Run("capped", func(t *testing.T) {
		result := CapGraph(snap, 3)
		if len(result.Nodes) != 3 {
			t.Errorf("expected 3 nodes, got %d", len(result.Nodes))
		}
		// Verify only edges between kept nodes
		for _, e := range result.Edges {
			if _, ok := result.Nodes[e.From]; !ok {
				t.Errorf("edge from %s but node not in result", e.From)
			}
			if _, ok := result.Nodes[e.To]; !ok {
				t.Errorf("edge to %s but node not in result", e.To)
			}
		}
	})
}

func TestEgoGraph(t *testing.T) {
	snap := testSnapshot()

	t.Run("deps only", func(t *testing.T) {
		result := EgoGraph(snap, "//a:lib", 1, "deps", 0)
		if _, ok := result.Nodes["//a:lib"]; !ok {
			t.Error("expected target node")
		}
		if _, ok := result.Nodes["//b:lib"]; !ok {
			t.Error("expected forward dep //b:lib")
		}
		// Should not include reverse deps
		if _, ok := result.Nodes["//f:lib"]; ok {
			t.Error("did not expect reverse dep //f:lib with direction=deps")
		}
	})

	t.Run("rdeps only", func(t *testing.T) {
		result := EgoGraph(snap, "//a:lib", 1, "rdeps", 0)
		if _, ok := result.Nodes["//f:lib"]; !ok {
			t.Error("expected reverse dep //f:lib")
		}
		if _, ok := result.Nodes["//b:lib"]; ok {
			t.Error("did not expect forward dep //b:lib with direction=rdeps")
		}
	})

	t.Run("package match", func(t *testing.T) {
		result := EgoGraph(snap, "//a", 0, "both", 0)
		if _, ok := result.Nodes["//a:lib"]; !ok {
			t.Error("expected //a:lib from package match")
		}
		if _, ok := result.Nodes["//a:test"]; !ok {
			t.Error("expected //a:test from package match")
		}
	})

	t.Run("no match", func(t *testing.T) {
		result := EgoGraph(snap, "//nonexistent", 2, "both", 0)
		if len(result.Nodes) != 0 {
			t.Errorf("expected empty result, got %d nodes", len(result.Nodes))
		}
	})
}

func TestFindPaths(t *testing.T) {
	snap := testSnapshot()

	t.Run("direct path", func(t *testing.T) {
		result := FindPaths(snap, "//a:lib", "//b:lib", 10)
		if len(result.Paths) != 1 {
			t.Errorf("expected 1 path, got %d", len(result.Paths))
		}
		if result.PathLength != 1 {
			t.Errorf("expected path length 1, got %d", result.PathLength)
		}
	})

	t.Run("multi-hop path", func(t *testing.T) {
		result := FindPaths(snap, "//a:lib", "//d:lib", 10)
		if len(result.Paths) == 0 {
			t.Error("expected at least one path")
		}
		if result.PathLength != 3 {
			t.Errorf("expected path length 3, got %d", result.PathLength)
		}
	})

	t.Run("no path", func(t *testing.T) {
		result := FindPaths(snap, "//d:lib", "//a:lib", 10)
		if len(result.Paths) != 0 {
			t.Errorf("expected no paths, got %d", len(result.Paths))
		}
	})

	t.Run("nonexistent node", func(t *testing.T) {
		result := FindPaths(snap, "//nonexistent", "//a:lib", 10)
		if len(result.Paths) != 0 {
			t.Errorf("expected no paths, got %d", len(result.Paths))
		}
	})
}

func TestAggregatePackages(t *testing.T) {
	snap := testSnapshot()

	t.Run("no filters", func(t *testing.T) {
		result := AggregatePackages(snap, false, false, 1, 0)
		if len(result.Nodes) == 0 {
			t.Error("expected package nodes")
		}
		// Should have //a, //b, //c, //d, @ext//e, //f
		if len(result.Nodes) != 6 {
			t.Errorf("expected 6 packages, got %d", len(result.Nodes))
		}
	})

	t.Run("hide tests", func(t *testing.T) {
		result := AggregatePackages(snap, true, false, 1, 0)
		aPkg := result.Nodes["//a"]
		if aPkg == nil {
			t.Fatal("expected //a package")
		}
		if aPkg.TargetCount != 1 {
			t.Errorf("expected 1 target in //a (test hidden), got %d", aPkg.TargetCount)
		}
	})

	t.Run("hide external", func(t *testing.T) {
		result := AggregatePackages(snap, false, true, 1, 0)
		if _, ok := result.Nodes["@ext//e"]; ok {
			t.Error("expected external package to be hidden")
		}
	})

	t.Run("min edge weight", func(t *testing.T) {
		result := AggregatePackages(snap, false, false, 5, 0)
		if len(result.Edges) != 0 {
			t.Errorf("expected no edges with min_weight=5, got %d", len(result.Edges))
		}
	})

	t.Run("package capping", func(t *testing.T) {
		result := AggregatePackages(snap, false, false, 1, 2)
		if len(result.Nodes) > 2 {
			t.Errorf("expected at most 2 packages, got %d", len(result.Nodes))
		}
		if !result.Truncated {
			t.Error("expected truncated=true")
		}
	})
}

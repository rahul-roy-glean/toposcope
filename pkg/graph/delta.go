package graph

import "github.com/google/uuid"

// ComputeDelta computes the structural difference between a base and head snapshot.
// For nodes, it diffs by key. For edges, it diffs by (from, to, type) triple.
func ComputeDelta(base, head *Snapshot) *Delta {
	delta := &Delta{
		ID:             uuid.New().String(),
		BaseSnapshotID: base.ID,
		HeadSnapshotID: head.ID,
	}

	// Node diff
	for key, node := range head.Nodes {
		if _, exists := base.Nodes[key]; !exists {
			delta.AddedNodes = append(delta.AddedNodes, *node)
		}
	}
	for key, node := range base.Nodes {
		if _, exists := head.Nodes[key]; !exists {
			delta.RemovedNodes = append(delta.RemovedNodes, *node)
		}
	}

	// Edge diff using set operations on edge keys
	baseEdges := make(map[string]Edge, len(base.Edges))
	for _, e := range base.Edges {
		baseEdges[e.EdgeKey()] = e
	}
	headEdges := make(map[string]Edge, len(head.Edges))
	for _, e := range head.Edges {
		headEdges[e.EdgeKey()] = e
	}

	for key, edge := range headEdges {
		if _, exists := baseEdges[key]; !exists {
			delta.AddedEdges = append(delta.AddedEdges, edge)
		}
	}
	for key, edge := range baseEdges {
		if _, exists := headEdges[key]; !exists {
			delta.RemovedEdges = append(delta.RemovedEdges, edge)
		}
	}

	delta.Stats = DeltaStats{
		AddedNodeCount:   len(delta.AddedNodes),
		RemovedNodeCount: len(delta.RemovedNodes),
		AddedEdgeCount:   len(delta.AddedEdges),
		RemovedEdgeCount: len(delta.RemovedEdges),
	}

	return delta
}

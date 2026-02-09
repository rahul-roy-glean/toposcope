// Package baseline provides an adapter from the subgraph extractor
// to the extract.Extractor interface.
package baseline

import (
	"context"

	"github.com/toposcope/toposcope/pkg/extract"
	"github.com/toposcope/toposcope/pkg/extract/subgraph"
	"github.com/toposcope/toposcope/pkg/graph"
)

// Adapter wraps a subgraph.Extractor to implement extract.Extractor.
type Adapter struct {
	Extractor *subgraph.Extractor
}

// Extract implements extract.Extractor.
func (a *Adapter) Extract(ctx context.Context, req extract.ExtractionRequest) (*graph.Snapshot, error) {
	switch req.Scope.Mode {
	case extract.ScopeModeFull:
		return a.Extractor.ExtractFull(ctx, req.CommitSHA, req.Scope.Timeout)
	default:
		depth := req.Scope.RdepsDepth
		if depth <= 0 {
			depth = 2
		}
		return a.Extractor.Extract(ctx, subgraph.SubgraphRequest{
			Targets:   req.Scope.Roots,
			RdepDepth: depth,
			CommitSHA: req.CommitSHA,
			Timeout:   req.Scope.Timeout,
		})
	}
}

// Verify interface satisfaction at compile time.
var _ extract.Extractor = (*Adapter)(nil)

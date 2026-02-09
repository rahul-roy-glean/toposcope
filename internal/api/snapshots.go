package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/toposcope/toposcope/pkg/graph"
	"github.com/toposcope/toposcope/pkg/graphquery"
)

// loadSnapshot loads a snapshot by ID, checking the cache first,
// then falling back to DB metadata lookup + storage client.
func (h *Handler) loadSnapshot(ctx context.Context, snapshotID string) (*graph.Snapshot, error) {
	// Check cache
	if snap := h.cache.Get(snapshotID); snap != nil {
		return snap, nil
	}

	// Look up metadata
	snapshotRow, err := h.tenantSvc.GetSnapshotByID(ctx, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("snapshot metadata: %w", err)
	}

	// Load from storage
	data, err := h.ingestionSvc.Storage().GetSnapshot(ctx, snapshotRow.TenantID, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("load snapshot blob: %w", err)
	}

	var snap graph.Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("unmarshal snapshot: %w", err)
	}

	// Cache it
	h.cache.Put(snapshotID, &snap)

	return &snap, nil
}

func (h *Handler) handleGetSnapshot(w http.ResponseWriter, r *http.Request) {
	snapshotID := r.PathValue("snapshotID")

	snap, err := h.loadSnapshot(r.Context(), snapshotID)
	if err != nil {
		writeError(w, http.StatusNotFound, "snapshot not found")
		return
	}

	writeJSON(w, http.StatusOK, snap)
}

func (h *Handler) handleSubgraph(w http.ResponseWriter, r *http.Request) {
	snapshotID := r.PathValue("snapshotID")

	snap, err := h.loadSnapshot(r.Context(), snapshotID)
	if err != nil {
		writeError(w, http.StatusNotFound, "snapshot not found")
		return
	}

	roots := r.URL.Query()["root"]
	depthStr := r.URL.Query().Get("depth")
	depth := 2
	if depthStr != "" {
		_, _ = fmt.Sscanf(depthStr, "%d", &depth)
	}

	if len(roots) == 0 {
		result := graphquery.CapGraph(snap, 500)
		writeJSON(w, http.StatusOK, result)
		return
	}

	result := graphquery.ExtractSubgraph(snap, roots, depth)
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handlePackages(w http.ResponseWriter, r *http.Request) {
	snapshotID := r.PathValue("snapshotID")

	snap, err := h.loadSnapshot(r.Context(), snapshotID)
	if err != nil {
		writeError(w, http.StatusNotFound, "snapshot not found")
		return
	}

	hideTests := r.URL.Query().Get("hide_tests") == "true"
	hideExternal := r.URL.Query().Get("hide_external") == "true"
	minEdgeWeight := 1
	if v := r.URL.Query().Get("min_edge_weight"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			minEdgeWeight = parsed
		}
	}

	result := graphquery.AggregatePackages(snap, hideTests, hideExternal, minEdgeWeight, 0)
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleEgo(w http.ResponseWriter, r *http.Request) {
	snapshotID := r.PathValue("snapshotID")

	snap, err := h.loadSnapshot(r.Context(), snapshotID)
	if err != nil {
		writeError(w, http.StatusNotFound, "snapshot not found")
		return
	}

	target := r.URL.Query().Get("target")
	if target == "" {
		writeError(w, http.StatusBadRequest, "target parameter required")
		return
	}

	depth := 2
	if v := r.URL.Query().Get("depth"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			depth = parsed
		}
	}

	direction := r.URL.Query().Get("direction")
	if direction == "" {
		direction = "both"
	}

	result := graphquery.EgoGraph(snap, target, depth, direction, 0)
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handlePath(w http.ResponseWriter, r *http.Request) {
	snapshotID := r.PathValue("snapshotID")

	snap, err := h.loadSnapshot(r.Context(), snapshotID)
	if err != nil {
		writeError(w, http.StatusNotFound, "snapshot not found")
		return
	}

	fromQ := r.URL.Query().Get("from")
	toQ := r.URL.Query().Get("to")
	if fromQ == "" || toQ == "" {
		writeError(w, http.StatusBadRequest, "from and to parameters required")
		return
	}

	maxPaths := 10
	if v := r.URL.Query().Get("max_paths"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			maxPaths = parsed
		}
	}

	result := graphquery.FindPaths(snap, fromQ, toQ, maxPaths)
	writeJSON(w, http.StatusOK, result)
}

package api

import (
	"encoding/json"
	"log"
	"net/http"
	"path"

	"github.com/toposcope/toposcope/pkg/graph"
	"github.com/toposcope/toposcope/pkg/scoring"
)

type rescoreRequest struct {
	RepoID string `json:"repo_id"` // optional filter
}

type rescoreResponse struct {
	Rescored int `json:"rescored"`
	Errors   int `json:"errors"`
}

// handleRescore re-runs the scoring engine on all existing score rows.
// It loads base/head snapshots and deltas from storage, recomputes scores,
// and updates the rows in-place.
func (h *Handler) handleRescore(w http.ResponseWriter, r *http.Request) {
	var req rescoreRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
	}

	ctx := r.Context()

	// Query score rows joined with snapshots and deltas to get storage refs.
	// The storage_ref format is "{kind}/{tenant_id}/{object_id}.json", so we
	// extract the object_id to pass to the storage client.
	query := `
		SELECT s.id, s.tenant_id,
			bs.storage_ref, hs.storage_ref, d.storage_ref
		FROM scores s
		JOIN snapshots bs ON bs.id = s.base_snapshot_id
		JOIN snapshots hs ON hs.id = s.head_snapshot_id
		JOIN deltas d ON d.id = s.delta_id`
	var args []any
	if req.RepoID != "" {
		query += ` WHERE s.repo_id = $1`
		args = append(args, req.RepoID)
	}
	query += ` ORDER BY s.created_at ASC`

	rows, err := h.db.QueryContext(ctx, query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query scores: "+err.Error())
		return
	}
	defer rows.Close()

	type scoreRow struct {
		ID              string
		TenantID        string
		BaseStorageRef  string
		HeadStorageRef  string
		DeltaStorageRef string
	}
	var scoreRows []scoreRow
	for rows.Next() {
		var sr scoreRow
		if err := rows.Scan(&sr.ID, &sr.TenantID, &sr.BaseStorageRef, &sr.HeadStorageRef, &sr.DeltaStorageRef); err != nil {
			writeError(w, http.StatusInternalServerError, "scan score row: "+err.Error())
			return
		}
		scoreRows = append(scoreRows, sr)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "iterate scores: "+err.Error())
		return
	}

	engine := scoring.NewEngine(scoring.DefaultMetrics()...)
	resp := rescoreResponse{}

	for _, sr := range scoreRows {
		baseID := storageIDFromRef(sr.BaseStorageRef)
		headID := storageIDFromRef(sr.HeadStorageRef)
		deltaID := storageIDFromRef(sr.DeltaStorageRef)

		// Load base snapshot
		baseData, err := h.ingestionSvc.Storage().GetSnapshot(ctx, sr.TenantID, baseID)
		if err != nil {
			log.Printf("rescore %s: load base snapshot: %v", sr.ID, err)
			resp.Errors++
			continue
		}
		var base graph.Snapshot
		if err := json.Unmarshal(baseData, &base); err != nil {
			log.Printf("rescore %s: unmarshal base snapshot: %v", sr.ID, err)
			resp.Errors++
			continue
		}

		// Load head snapshot
		headData, err := h.ingestionSvc.Storage().GetSnapshot(ctx, sr.TenantID, headID)
		if err != nil {
			log.Printf("rescore %s: load head snapshot: %v", sr.ID, err)
			resp.Errors++
			continue
		}
		var head graph.Snapshot
		if err := json.Unmarshal(headData, &head); err != nil {
			log.Printf("rescore %s: unmarshal head snapshot: %v", sr.ID, err)
			resp.Errors++
			continue
		}

		// Load delta (or recompute if storage ref is missing)
		var delta graph.Delta
		if deltaID != "" {
			deltaData, err := h.ingestionSvc.Storage().GetDelta(ctx, sr.TenantID, deltaID)
			if err != nil {
				log.Printf("rescore %s: load delta failed (%v), recomputing from snapshots", sr.ID, err)
				recomputed := computeDelta(&base, &head)
				delta = *recomputed
			} else if err := json.Unmarshal(deltaData, &delta); err != nil {
				log.Printf("rescore %s: unmarshal delta failed (%v), recomputing from snapshots", sr.ID, err)
				recomputed := computeDelta(&base, &head)
				delta = *recomputed
			}
		} else {
			recomputed := computeDelta(&base, &head)
			delta = *recomputed
		}

		// Re-score
		result, err := engine.Score(&delta, &base, &head)
		if err != nil {
			log.Printf("rescore %s: score: %v", sr.ID, err)
			resp.Errors++
			continue
		}

		// Update score row
		if err := h.ingestionSvc.UpdateScore(ctx, sr.ID, result); err != nil {
			log.Printf("rescore %s: update: %v", sr.ID, err)
			resp.Errors++
			continue
		}

		resp.Rescored++
	}

	writeJSON(w, http.StatusOK, resp)
}

// storageIDFromRef extracts the object ID from a storage_ref like
// "snapshots/{tenant_id}/{id}.json" → "{id}".
func storageIDFromRef(ref string) string {
	base := path.Base(ref)           // "{id}.json"
	ext := path.Ext(base)            // ".json"
	return base[:len(base)-len(ext)] // "{id}"
}

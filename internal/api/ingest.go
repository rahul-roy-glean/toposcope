package api

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/toposcope/toposcope/internal/ingestion"
	"github.com/toposcope/toposcope/pkg/graph"
	"github.com/toposcope/toposcope/pkg/scoring"
)

// ingestRequest is the JSON body for POST /api/v1/ingest.
type ingestRequest struct {
	RepoFullName   string               `json:"repo_full_name"`
	DefaultBranch  string               `json:"default_branch"`
	CommitSHA      string               `json:"commit_sha"`
	Branch         string               `json:"branch"`
	CommittedAt    string               `json:"committed_at"` // RFC3339; if set, used as timestamp instead of now()
	Snapshot       *graph.Snapshot      `json:"snapshot"`
	Score          *scoring.ScoreResult `json:"score"`
	BaseSnapshot   *graph.Snapshot      `json:"base_snapshot"`
	SnapshotID     string               `json:"snapshot_id"`
	BaseSnapshotID string               `json:"base_snapshot_id"`
}

type ingestResponse struct {
	SnapshotID     string `json:"snapshot_id"`
	BaseSnapshotID string `json:"base_snapshot_id,omitempty"`
	DeltaID        string `json:"delta_id,omitempty"`
	ScoreID        string `json:"score_id,omitempty"`
}

// handleUploadSnapshot handles POST /api/v1/snapshots — uploads a single snapshot
// and returns its storage ID. Used for the two-step ingest flow where large
// snapshots are uploaded separately from the ingest request.
func (h *Handler) handleUploadSnapshot(w http.ResponseWriter, r *http.Request) {
	var body io.Reader = r.Body
	if r.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid gzip body: "+err.Error())
			return
		}
		defer gz.Close()
		body = gz
	}

	data, err := io.ReadAll(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read body: "+err.Error())
		return
	}

	// Validate that the body is valid JSON snapshot
	var snap graph.Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		writeError(w, http.StatusBadRequest, "invalid snapshot JSON: "+err.Error())
		return
	}

	// Generate a storage ID and store the blob
	snapshotID := uuid.New().String()
	// Use a synthetic tenant ID for pre-upload; the actual tenant association
	// happens when the ingest request references this snapshot.
	if err := h.ingestionSvc.Storage().PutSnapshot(r.Context(), "_uploads", snapshotID, data); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store snapshot: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"snapshot_id": snapshotID})
}

func (h *Handler) handleIngest(w http.ResponseWriter, r *http.Request) {
	// Support gzip-compressed request bodies
	var body io.Reader = r.Body
	if r.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid gzip body: "+err.Error())
			return
		}
		defer gz.Close()
		body = gz
	}

	var req ingestRequest
	if err := json.NewDecoder(body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	// Reference mode: load snapshot from storage if snapshot_id is provided
	ctx := r.Context()
	if req.SnapshotID != "" && req.Snapshot == nil {
		data, err := h.ingestionSvc.Storage().GetSnapshot(ctx, "_uploads", req.SnapshotID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "failed to load referenced snapshot: "+err.Error())
			return
		}
		var snap graph.Snapshot
		if err := json.Unmarshal(data, &snap); err != nil {
			writeError(w, http.StatusBadRequest, "invalid referenced snapshot: "+err.Error())
			return
		}
		req.Snapshot = &snap
	}
	if req.BaseSnapshotID != "" && req.BaseSnapshot == nil {
		data, err := h.ingestionSvc.Storage().GetSnapshot(ctx, "_uploads", req.BaseSnapshotID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "failed to load referenced base snapshot: "+err.Error())
			return
		}
		var snap graph.Snapshot
		if err := json.Unmarshal(data, &snap); err != nil {
			writeError(w, http.StatusBadRequest, "invalid referenced base snapshot: "+err.Error())
			return
		}
		req.BaseSnapshot = &snap
	}

	if req.RepoFullName == "" || req.CommitSHA == "" || req.Snapshot == nil {
		writeError(w, http.StatusBadRequest, "repo_full_name, commit_sha, and snapshot are required")
		return
	}

	if req.DefaultBranch == "" {
		req.DefaultBranch = "main"
	}

	// Extract org name from repo full name (e.g., "org/repo" -> "org")
	orgName := req.RepoFullName
	if idx := strings.Index(req.RepoFullName, "/"); idx > 0 {
		orgName = req.RepoFullName[:idx]
	}

	// Ensure tenant and repo exist
	tenantID, repoID, err := h.tenantSvc.EnsureTenantAndRepo(ctx, orgName, req.RepoFullName, req.DefaultBranch)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to ensure tenant/repo: "+err.Error())
		return
	}

	ingReq := ingestion.IngestionRequest{
		TenantID:     tenantID,
		RepoID:       repoID,
		RepoFullName: req.RepoFullName,
		CommitSHA:    req.CommitSHA,
		BaseBranch:   req.DefaultBranch,
	}

	// Use commit time if provided
	if req.CommittedAt != "" {
		if t, err := time.Parse(time.RFC3339, req.CommittedAt); err == nil {
			ingReq.CommittedAt = &t
		}
	}

	// Store the head snapshot
	req.Snapshot.CommitSHA = req.CommitSHA
	req.Snapshot.Branch = req.Branch

	snapData, err := json.Marshal(req.Snapshot)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal snapshot: "+err.Error())
		return
	}

	headSnapshotID, err := h.ingestionSvc.StoreSnapshot(ctx, ingReq, req.Snapshot, snapData)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store snapshot: "+err.Error())
		return
	}

	resp := ingestResponse{
		SnapshotID: headSnapshotID,
	}

	// If base snapshot provided, store it and compute delta
	var baseSnapshotID string
	if req.BaseSnapshot != nil {
		baseData, err := json.Marshal(req.BaseSnapshot)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to marshal base snapshot: "+err.Error())
			return
		}

		baseSnapshotID, err = h.ingestionSvc.StoreSnapshot(ctx, ingReq, req.BaseSnapshot, baseData)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to store base snapshot: "+err.Error())
			return
		}
		resp.BaseSnapshotID = baseSnapshotID

		// Compute and store delta
		delta := computeDelta(req.BaseSnapshot, req.Snapshot)
		delta.BaseSnapshotID = baseSnapshotID
		delta.HeadSnapshotID = headSnapshotID

		deltaData, err := json.Marshal(delta)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to marshal delta: "+err.Error())
			return
		}

		deltaID, err := h.ingestionSvc.StoreDelta(ctx, ingReq, delta, deltaData)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to store delta: "+err.Error())
			return
		}
		resp.DeltaID = deltaID

		// Store score if provided
		if req.Score != nil {
			scoreID, err := h.ingestionSvc.StoreScore(ctx, ingReq, baseSnapshotID, headSnapshotID, deltaID, req.Score)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to store score: "+err.Error())
				return
			}
			resp.ScoreID = scoreID
		}
	} else if req.Score != nil {
		// Score without base snapshot: use empty IDs for base/delta
		// Look up existing baseline for the repo
		var existingBaseID string
		err := h.db.QueryRowContext(ctx,
			`SELECT snapshot_id FROM baselines WHERE repo_id = $1`, repoID,
		).Scan(&existingBaseID)
		if err == nil {
			baseSnapshotID = existingBaseID
		}

		if baseSnapshotID != "" {
			scoreID, err := h.ingestionSvc.StoreScore(ctx, ingReq, baseSnapshotID, headSnapshotID, "", req.Score)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to store score: "+err.Error())
				return
			}
			resp.ScoreID = scoreID
		}
	}

	// Update baseline if this is a push to the default branch
	if req.Branch == req.DefaultBranch {
		_, err := h.db.ExecContext(ctx,
			`INSERT INTO baselines (repo_id, snapshot_id) VALUES ($1, $2)
			 ON CONFLICT (repo_id) DO UPDATE SET snapshot_id = $2, updated_at = now()`,
			repoID, headSnapshotID,
		)
		if err != nil {
			// Log but don't fail the request
			fmt.Printf("warning: failed to update baseline: %v\n", err)
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// computeDelta calculates the structural difference between two snapshots.
func computeDelta(base, head *graph.Snapshot) *graph.Delta {
	delta := &graph.Delta{}

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

	baseEdges := make(map[string]graph.Edge)
	for _, e := range base.Edges {
		baseEdges[e.EdgeKey()] = e
	}
	headEdges := make(map[string]graph.Edge)
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

	delta.Stats = graph.DeltaStats{
		AddedNodeCount:   len(delta.AddedNodes),
		RemovedNodeCount: len(delta.RemovedNodes),
		AddedEdgeCount:   len(delta.AddedEdges),
		RemovedEdgeCount: len(delta.RemovedEdges),
	}

	return delta
}

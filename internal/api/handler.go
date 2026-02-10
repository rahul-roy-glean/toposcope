// Package api implements the hosted Toposcope REST API.
// It provides ingest and read endpoints backed by Postgres and blob storage.
package api

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/toposcope/toposcope/internal/ingestion"
	"github.com/toposcope/toposcope/internal/tenant"
)

// Handler is the top-level API handler for the hosted Toposcope service.
type Handler struct {
	db           *sql.DB
	tenantSvc    *tenant.Service
	ingestionSvc *ingestion.Service
	cache        *SnapshotCache
}

// NewHandler creates a new API handler.
func NewHandler(db *sql.DB, tenantSvc *tenant.Service, ingestionSvc *ingestion.Service, cache *SnapshotCache) *Handler {
	if cache == nil {
		cache = NewSnapshotCacheFromEnv()
	}
	return &Handler{
		db:           db,
		tenantSvc:    tenantSvc,
		ingestionSvc: ingestionSvc,
		cache:        cache,
	}
}

// RegisterRoutes registers all API routes on the given ServeMux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Write endpoints (auth-protected)
	mux.HandleFunc("POST /api/v1/ingest", h.handleIngest)
	mux.HandleFunc("POST /api/v1/snapshots", h.handleUploadSnapshot)
	mux.HandleFunc("PATCH /api/repos/{repoID}", h.handleUpdateRepo)
	mux.HandleFunc("DELETE /api/repos/{repoID}", h.handleDeleteRepo)

	// Read endpoints
	mux.HandleFunc("GET /api/repos", h.handleListRepos)
	mux.HandleFunc("GET /api/repos/{repoID}/scores", h.handleListScores)
	mux.HandleFunc("GET /api/repos/{repoID}/scores/{scoreID}", h.handleGetScore)
	mux.HandleFunc("GET /api/repos/{repoID}/history", h.handleHistory)
	mux.HandleFunc("GET /api/repos/{repoID}/prs/{prNumber}/impact", h.handlePRImpact)
	mux.HandleFunc("GET /api/snapshots/{snapshotID}", h.handleGetSnapshot)
	mux.HandleFunc("GET /api/snapshots/{snapshotID}/subgraph", h.handleSubgraph)
	mux.HandleFunc("GET /api/snapshots/{snapshotID}/packages", h.handlePackages)
	mux.HandleFunc("GET /api/snapshots/{snapshotID}/ego", h.handleEgo)
	mux.HandleFunc("GET /api/snapshots/{snapshotID}/path", h.handlePath)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

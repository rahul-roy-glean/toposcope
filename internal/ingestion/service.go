package ingestion

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/toposcope/toposcope/internal/tenant"
	"github.com/toposcope/toposcope/pkg/extract"
	"github.com/toposcope/toposcope/pkg/graph"
	"github.com/toposcope/toposcope/pkg/scoring"
)

// IngestionStatus represents the lifecycle of an ingestion.
const (
	StatusQueued    = "QUEUED"
	StatusRunning   = "RUNNING"
	StatusCompleted = "COMPLETED"
	StatusFailed    = "FAILED"
)

// IngestionRequest describes what to ingest.
type IngestionRequest struct {
	TenantID       string
	RepoID         string
	RepoFullName   string
	CommitSHA      string
	BaseBranch     string
	PRNumber       *int
	InstallationID int64
}

// Scorer abstracts the scoring engine so the ingestion package does not
// depend on a concrete implementation.
type Scorer interface {
	Score(base, head *graph.Snapshot, delta *graph.Delta) (*scoring.ScoreResult, error)
}

// Service orchestrates the ingestion pipeline.
type Service struct {
	db        *sql.DB
	tenants   *tenant.Service
	storage   StorageClient
	extractor extract.Extractor
	scorer    Scorer
}

// NewService creates a new ingestion Service.
func NewService(db *sql.DB, tenants *tenant.Service, storage StorageClient, extractor extract.Extractor, scorer Scorer) *Service {
	return &Service{
		db:        db,
		tenants:   tenants,
		storage:   storage,
		extractor: extractor,
		scorer:    scorer,
	}
}

// CreateIngestion creates a new ingestion record and returns its ID.
// The idempotency key is repo_id + commit_sha (+ pr_number if present).
func (s *Service) CreateIngestion(ctx context.Context, req IngestionRequest) (string, error) {
	idempotencyKey := fmt.Sprintf("%s:%s", req.RepoID, req.CommitSHA)
	if req.PRNumber != nil {
		idempotencyKey = fmt.Sprintf("%s:pr%d", idempotencyKey, *req.PRNumber)
	}

	var id string
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO ingestions (tenant_id, repo_id, commit_sha, pr_number, idempotency_key)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (idempotency_key) DO UPDATE SET updated_at = now()
		 RETURNING id`,
		req.TenantID, req.RepoID, req.CommitSHA, req.PRNumber, idempotencyKey,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("create ingestion: %w", err)
	}
	return id, nil
}

// UpdateIngestionStatus updates the status and optional error message.
func (s *Service) UpdateIngestionStatus(ctx context.Context, id, status string, errMsg *string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE ingestions SET status = $1, error_message = $2, updated_at = now() WHERE id = $3`,
		status, errMsg, id,
	)
	if err != nil {
		return fmt.Errorf("update ingestion status: %w", err)
	}
	return nil
}

// ProcessPR runs the full ingestion pipeline for a PR or push event.
func (s *Service) ProcessPR(ctx context.Context, req IngestionRequest) error {
	// 1. Create or retrieve ingestion record
	ingestionID, err := s.CreateIngestion(ctx, req)
	if err != nil {
		return fmt.Errorf("create ingestion: %w", err)
	}

	if err := s.UpdateIngestionStatus(ctx, ingestionID, StatusRunning, nil); err != nil {
		return fmt.Errorf("update status to running: %w", err)
	}

	// On failure, mark ingestion as failed
	defer func() {
		if err != nil {
			errMsg := err.Error()
			if updateErr := s.UpdateIngestionStatus(ctx, ingestionID, StatusFailed, &errMsg); updateErr != nil {
				log.Printf("failed to update ingestion status: %v", updateErr)
			}
		}
	}()

	// 2. Ensure baseline exists
	baseSnapshotID, err := s.ensureBaseline(ctx, req)
	if err != nil {
		return fmt.Errorf("ensure baseline: %w", err)
	}

	// 3. Extract head snapshot
	start := time.Now()
	headSnapshot, err := s.extractor.Extract(ctx, extract.ExtractionRequest{
		CommitSHA: req.CommitSHA,
		Scope: extract.ExtractionScope{
			Mode: extract.ScopeModeFull,
		},
	})
	if err != nil {
		return fmt.Errorf("extract head snapshot: %w", err)
	}
	headSnapshot.Stats.ExtractionMs = int(time.Since(start).Milliseconds())

	// Store head snapshot
	headSnapshotData, err := json.Marshal(headSnapshot)
	if err != nil {
		return fmt.Errorf("marshal head snapshot: %w", err)
	}

	headSnapshotID, err := s.storeSnapshot(ctx, req, headSnapshot, headSnapshotData)
	if err != nil {
		return fmt.Errorf("store head snapshot: %w", err)
	}

	// 4. Load base snapshot and compute delta
	baseSnapshotData, err := s.storage.GetSnapshot(ctx, req.TenantID, baseSnapshotID)
	if err != nil {
		return fmt.Errorf("load base snapshot: %w", err)
	}

	var baseSnapshot graph.Snapshot
	if err := json.Unmarshal(baseSnapshotData, &baseSnapshot); err != nil {
		return fmt.Errorf("unmarshal base snapshot: %w", err)
	}

	delta := computeDelta(&baseSnapshot, headSnapshot)
	delta.BaseSnapshotID = baseSnapshotID
	delta.HeadSnapshotID = headSnapshotID

	deltaData, err := json.Marshal(delta)
	if err != nil {
		return fmt.Errorf("marshal delta: %w", err)
	}

	deltaID, err := s.storeDelta(ctx, req, delta, deltaData)
	if err != nil {
		return fmt.Errorf("store delta: %w", err)
	}

	// 5. Score
	var scoreResult *scoring.ScoreResult
	if s.scorer != nil {
		scoreResult, err = s.scorer.Score(&baseSnapshot, headSnapshot, delta)
		if err != nil {
			return fmt.Errorf("score: %w", err)
		}
	}

	// 6. Store score
	var scoreID string
	if scoreResult != nil {
		scoreID, err = s.storeScore(ctx, req, baseSnapshotID, headSnapshotID, deltaID, scoreResult)
		if err != nil {
			return fmt.Errorf("store score: %w", err)
		}
	}

	// 7. Update ingestion with results
	_, err = s.db.ExecContext(ctx,
		`UPDATE ingestions SET status = $1, snapshot_id = $2, delta_id = $3, score_id = $4, updated_at = now()
		 WHERE id = $5`,
		StatusCompleted, headSnapshotID, deltaID, nilIfEmpty(scoreID), ingestionID,
	)
	if err != nil {
		return fmt.Errorf("finalize ingestion: %w", err)
	}

	log.Printf("ingestion %s completed: snapshot=%s delta=%s score=%s", ingestionID, headSnapshotID, deltaID, scoreID)
	return nil
}

func (s *Service) ensureBaseline(ctx context.Context, req IngestionRequest) (string, error) {
	var snapshotID string
	err := s.db.QueryRowContext(ctx,
		`SELECT snapshot_id FROM baselines WHERE repo_id = $1`,
		req.RepoID,
	).Scan(&snapshotID)
	if err == nil {
		return snapshotID, nil
	}
	if err != sql.ErrNoRows {
		return "", fmt.Errorf("query baseline: %w", err)
	}

	// No baseline: extract one from the base branch
	baseSnapshot, err := s.extractor.Extract(ctx, extract.ExtractionRequest{
		Scope: extract.ExtractionScope{
			Mode: extract.ScopeModeFull,
		},
	})
	if err != nil {
		return "", fmt.Errorf("extract baseline: %w", err)
	}

	data, err := json.Marshal(baseSnapshot)
	if err != nil {
		return "", fmt.Errorf("marshal baseline: %w", err)
	}

	baseSnapshot.Branch = req.BaseBranch
	id, err := s.storeSnapshot(ctx, req, baseSnapshot, data)
	if err != nil {
		return "", fmt.Errorf("store baseline snapshot: %w", err)
	}

	// Set as baseline
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO baselines (repo_id, snapshot_id) VALUES ($1, $2)
		 ON CONFLICT (repo_id) DO UPDATE SET snapshot_id = $2, updated_at = now()`,
		req.RepoID, id,
	)
	if err != nil {
		return "", fmt.Errorf("set baseline: %w", err)
	}

	return id, nil
}

func (s *Service) storeSnapshot(ctx context.Context, req IngestionRequest, snap *graph.Snapshot, data []byte) (string, error) {
	storageRef := fmt.Sprintf("snapshots/%s/%s.json", req.TenantID, snap.ID)
	if err := s.storage.PutSnapshot(ctx, req.TenantID, snap.ID, data); err != nil {
		return "", fmt.Errorf("put snapshot blob: %w", err)
	}

	var id string
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO snapshots (tenant_id, repo_id, commit_sha, branch, node_count, edge_count, package_count, extraction_ms, storage_ref)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 ON CONFLICT (repo_id, commit_sha) DO UPDATE SET storage_ref = EXCLUDED.storage_ref
		 RETURNING id`,
		req.TenantID, req.RepoID, snap.CommitSHA, nilIfEmpty(snap.Branch),
		snap.Stats.NodeCount, snap.Stats.EdgeCount, snap.Stats.PackageCount, snap.Stats.ExtractionMs,
		storageRef,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert snapshot row: %w", err)
	}
	return id, nil
}

func (s *Service) storeDelta(ctx context.Context, req IngestionRequest, delta *graph.Delta, data []byte) (string, error) {
	storageRef := fmt.Sprintf("deltas/%s/%s.json", req.TenantID, delta.ID)
	if err := s.storage.PutDelta(ctx, req.TenantID, delta.ID, data); err != nil {
		return "", fmt.Errorf("put delta blob: %w", err)
	}

	var id string
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO deltas (tenant_id, repo_id, base_snapshot_id, head_snapshot_id, added_nodes, removed_nodes, added_edges, removed_edges, storage_ref)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 ON CONFLICT (base_snapshot_id, head_snapshot_id) DO UPDATE SET storage_ref = EXCLUDED.storage_ref
		 RETURNING id`,
		req.TenantID, req.RepoID, delta.BaseSnapshotID, delta.HeadSnapshotID,
		delta.Stats.AddedNodeCount, delta.Stats.RemovedNodeCount,
		delta.Stats.AddedEdgeCount, delta.Stats.RemovedEdgeCount,
		storageRef,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert delta row: %w", err)
	}
	return id, nil
}

func (s *Service) storeScore(ctx context.Context, req IngestionRequest, baseSnapshotID, headSnapshotID, deltaID string, result *scoring.ScoreResult) (string, error) {
	breakdownJSON, err := json.Marshal(result.Breakdown)
	if err != nil {
		return "", fmt.Errorf("marshal breakdown: %w", err)
	}
	hotspotsJSON, err := json.Marshal(result.Hotspots)
	if err != nil {
		return "", fmt.Errorf("marshal hotspots: %w", err)
	}
	actionsJSON, err := json.Marshal(result.SuggestedActions)
	if err != nil {
		return "", fmt.Errorf("marshal suggested actions: %w", err)
	}

	var id string
	err = s.db.QueryRowContext(ctx,
		`INSERT INTO scores (tenant_id, repo_id, pr_number, commit_sha, base_snapshot_id, head_snapshot_id, delta_id, total_score, grade, breakdown, hotspots, suggested_actions)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		 RETURNING id`,
		req.TenantID, req.RepoID, req.PRNumber, req.CommitSHA,
		baseSnapshotID, headSnapshotID, deltaID,
		result.TotalScore, result.Grade,
		breakdownJSON, hotspotsJSON, actionsJSON,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert score row: %w", err)
	}
	return id, nil
}

// computeDelta calculates the structural difference between two snapshots.
func computeDelta(base, head *graph.Snapshot) *graph.Delta {
	delta := &graph.Delta{}

	// Added/removed nodes
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

	// Added/removed edges
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

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

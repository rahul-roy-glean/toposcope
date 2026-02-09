// Package tenant manages multi-tenant state: tenants (GitHub App installations)
// and their associated repositories.
package tenant

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Service provides tenant and repository management backed by Postgres.
type Service struct {
	db *sql.DB
}

// Tenant represents a GitHub App installation (one per org/user).
type Tenant struct {
	ID                   string
	DisplayName          string
	GitHubInstallationID *int64
	CredentialsRef       *string
	CreatedAt            time.Time
}

// Repository represents a GitHub repository tracked by Toposcope.
type Repository struct {
	ID            string
	TenantID      string
	GitHubRepoID  *int64
	FullName      string
	DefaultBranch string
	CreatedAt     time.Time
}

// NewService creates a new tenant Service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// CreateTenant creates a new tenant for a GitHub App installation.
func (s *Service) CreateTenant(ctx context.Context, displayName string, installationID int64) (*Tenant, error) {
	t := &Tenant{}
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO tenants (display_name, github_installation_id)
		 VALUES ($1, $2)
		 RETURNING id, display_name, github_installation_id, credentials_ref, created_at`,
		displayName, installationID,
	).Scan(&t.ID, &t.DisplayName, &t.GitHubInstallationID, &t.CredentialsRef, &t.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create tenant: %w", err)
	}
	return t, nil
}

// GetTenantByInstallation looks up a tenant by GitHub App installation ID.
func (s *Service) GetTenantByInstallation(ctx context.Context, installationID int64) (*Tenant, error) {
	t := &Tenant{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, display_name, github_installation_id, credentials_ref, created_at
		 FROM tenants WHERE github_installation_id = $1`,
		installationID,
	).Scan(&t.ID, &t.DisplayName, &t.GitHubInstallationID, &t.CredentialsRef, &t.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get tenant by installation %d: %w", installationID, err)
	}
	return t, nil
}

// UpsertRepository creates or updates a repository record for a tenant.
func (s *Service) UpsertRepository(ctx context.Context, tenantID, fullName string, githubRepoID *int64, defaultBranch string) (*Repository, error) {
	r := &Repository{}
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO repositories (tenant_id, full_name, github_repo_id, default_branch)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (tenant_id, full_name) DO UPDATE
		   SET github_repo_id = COALESCE(EXCLUDED.github_repo_id, repositories.github_repo_id),
		       default_branch = EXCLUDED.default_branch
		 RETURNING id, tenant_id, github_repo_id, full_name, default_branch, created_at`,
		tenantID, fullName, githubRepoID, defaultBranch,
	).Scan(&r.ID, &r.TenantID, &r.GitHubRepoID, &r.FullName, &r.DefaultBranch, &r.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("upsert repository %s: %w", fullName, err)
	}
	return r, nil
}

// GetRepository retrieves a repository by tenant ID and full name.
func (s *Service) GetRepository(ctx context.Context, tenantID, fullName string) (*Repository, error) {
	r := &Repository{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, github_repo_id, full_name, default_branch, created_at
		 FROM repositories WHERE tenant_id = $1 AND full_name = $2`,
		tenantID, fullName,
	).Scan(&r.ID, &r.TenantID, &r.GitHubRepoID, &r.FullName, &r.DefaultBranch, &r.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get repository %s: %w", fullName, err)
	}
	return r, nil
}

// ListRepositories returns all repositories for a tenant.
func (s *Service) ListRepositories(ctx context.Context, tenantID string) ([]Repository, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, tenant_id, github_repo_id, full_name, default_branch, created_at
		 FROM repositories WHERE tenant_id = $1 ORDER BY full_name`,
		tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list repositories: %w", err)
	}
	defer rows.Close()

	var repos []Repository
	for rows.Next() {
		var r Repository
		if err := rows.Scan(&r.ID, &r.TenantID, &r.GitHubRepoID, &r.FullName, &r.DefaultBranch, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan repository: %w", err)
		}
		repos = append(repos, r)
	}
	return repos, rows.Err()
}

// ScoreRow represents a score record from the database.
type ScoreRow struct {
	ID               string
	TenantID         string
	RepoID           string
	PRNumber         *int
	CommitSHA        string
	BaseSnapshotID   string
	HeadSnapshotID   string
	DeltaID          string
	TotalScore       float64
	Grade            string
	Breakdown        json.RawMessage
	Hotspots         json.RawMessage
	SuggestedActions json.RawMessage
	CreatedAt        time.Time
}

// SnapshotRow represents snapshot metadata from the database.
type SnapshotRow struct {
	ID           string
	TenantID     string
	RepoID       string
	CommitSHA    string
	Branch       *string
	NodeCount    int
	EdgeCount    int
	PackageCount int
	ExtractionMs int
	StorageRef   string
	CreatedAt    time.Time
}

// GetTenantByName looks up a tenant by display name (for non-installation tenants).
func (s *Service) GetTenantByName(ctx context.Context, name string) (*Tenant, error) {
	t := &Tenant{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, display_name, github_installation_id, credentials_ref, created_at
		 FROM tenants WHERE display_name = $1`,
		name,
	).Scan(&t.ID, &t.DisplayName, &t.GitHubInstallationID, &t.CredentialsRef, &t.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get tenant by name %s: %w", name, err)
	}
	return t, nil
}

// CreateTenantByName creates a tenant without an installation ID (for CI-based ingest).
func (s *Service) CreateTenantByName(ctx context.Context, name string) (*Tenant, error) {
	t := &Tenant{}
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO tenants (display_name)
		 VALUES ($1)
		 RETURNING id, display_name, github_installation_id, credentials_ref, created_at`,
		name,
	).Scan(&t.ID, &t.DisplayName, &t.GitHubInstallationID, &t.CredentialsRef, &t.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create tenant by name: %w", err)
	}
	return t, nil
}

// EnsureTenantAndRepo gets or creates a tenant (by org name) and repository.
// Returns tenantID, repoID, and any error.
func (s *Service) EnsureTenantAndRepo(ctx context.Context, orgName, repoFullName, defaultBranch string) (string, string, error) {
	// Get or create tenant
	t, err := s.GetTenantByName(ctx, orgName)
	if err != nil {
		t, err = s.CreateTenantByName(ctx, orgName)
		if err != nil {
			// Could be a race condition; try getting again
			if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
				t, err = s.GetTenantByName(ctx, orgName)
				if err != nil {
					return "", "", fmt.Errorf("ensure tenant: %w", err)
				}
			} else {
				return "", "", fmt.Errorf("ensure tenant: %w", err)
			}
		}
	}

	// Get or create repository
	repo, err := s.UpsertRepository(ctx, t.ID, repoFullName, nil, defaultBranch)
	if err != nil {
		return "", "", fmt.Errorf("ensure repository: %w", err)
	}

	return t.ID, repo.ID, nil
}

// ListAllRepos returns all repositories across all tenants.
func (s *Service) ListAllRepos(ctx context.Context) ([]Repository, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, tenant_id, github_repo_id, full_name, default_branch, created_at
		 FROM repositories ORDER BY full_name`,
	)
	if err != nil {
		return nil, fmt.Errorf("list all repositories: %w", err)
	}
	defer rows.Close()

	var repos []Repository
	for rows.Next() {
		var r Repository
		if err := rows.Scan(&r.ID, &r.TenantID, &r.GitHubRepoID, &r.FullName, &r.DefaultBranch, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan repository: %w", err)
		}
		repos = append(repos, r)
	}
	return repos, rows.Err()
}

// ListScoresByRepo returns all scores for a repository, newest first.
func (s *Service) ListScoresByRepo(ctx context.Context, repoID string) ([]ScoreRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, tenant_id, repo_id, pr_number, commit_sha,
		        base_snapshot_id, head_snapshot_id, delta_id,
		        total_score, grade, breakdown, hotspots, suggested_actions, created_at
		 FROM scores WHERE repo_id = $1 ORDER BY created_at DESC`,
		repoID,
	)
	if err != nil {
		return nil, fmt.Errorf("list scores: %w", err)
	}
	defer rows.Close()

	var scores []ScoreRow
	for rows.Next() {
		var sc ScoreRow
		if err := rows.Scan(
			&sc.ID, &sc.TenantID, &sc.RepoID, &sc.PRNumber, &sc.CommitSHA,
			&sc.BaseSnapshotID, &sc.HeadSnapshotID, &sc.DeltaID,
			&sc.TotalScore, &sc.Grade, &sc.Breakdown, &sc.Hotspots, &sc.SuggestedActions, &sc.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan score: %w", err)
		}
		scores = append(scores, sc)
	}
	return scores, rows.Err()
}

// GetScoreByID returns a single score by ID.
func (s *Service) GetScoreByID(ctx context.Context, scoreID string) (*ScoreRow, error) {
	sc := &ScoreRow{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, repo_id, pr_number, commit_sha,
		        base_snapshot_id, head_snapshot_id, delta_id,
		        total_score, grade, breakdown, hotspots, suggested_actions, created_at
		 FROM scores WHERE id = $1`,
		scoreID,
	).Scan(
		&sc.ID, &sc.TenantID, &sc.RepoID, &sc.PRNumber, &sc.CommitSHA,
		&sc.BaseSnapshotID, &sc.HeadSnapshotID, &sc.DeltaID,
		&sc.TotalScore, &sc.Grade, &sc.Breakdown, &sc.Hotspots, &sc.SuggestedActions, &sc.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get score %s: %w", scoreID, err)
	}
	return sc, nil
}

// GetScoreByPR returns the most recent score for a PR.
func (s *Service) GetScoreByPR(ctx context.Context, repoID string, prNumber int) (*ScoreRow, error) {
	sc := &ScoreRow{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, repo_id, pr_number, commit_sha,
		        base_snapshot_id, head_snapshot_id, delta_id,
		        total_score, grade, breakdown, hotspots, suggested_actions, created_at
		 FROM scores WHERE repo_id = $1 AND pr_number = $2
		 ORDER BY created_at DESC LIMIT 1`,
		repoID, prNumber,
	).Scan(
		&sc.ID, &sc.TenantID, &sc.RepoID, &sc.PRNumber, &sc.CommitSHA,
		&sc.BaseSnapshotID, &sc.HeadSnapshotID, &sc.DeltaID,
		&sc.TotalScore, &sc.Grade, &sc.Breakdown, &sc.Hotspots, &sc.SuggestedActions, &sc.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get score for PR %d: %w", prNumber, err)
	}
	return sc, nil
}

// GetSnapshotByID returns snapshot metadata by ID.
func (s *Service) GetSnapshotByID(ctx context.Context, snapshotID string) (*SnapshotRow, error) {
	sn := &SnapshotRow{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, repo_id, commit_sha, branch,
		        node_count, edge_count, package_count, extraction_ms, storage_ref, created_at
		 FROM snapshots WHERE id = $1`,
		snapshotID,
	).Scan(
		&sn.ID, &sn.TenantID, &sn.RepoID, &sn.CommitSHA, &sn.Branch,
		&sn.NodeCount, &sn.EdgeCount, &sn.PackageCount, &sn.ExtractionMs, &sn.StorageRef, &sn.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get snapshot %s: %w", snapshotID, err)
	}
	return sn, nil
}

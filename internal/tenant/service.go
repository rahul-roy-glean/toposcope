// Package tenant manages multi-tenant state: tenants (GitHub App installations)
// and their associated repositories.
package tenant

import (
	"context"
	"database/sql"
	"fmt"
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

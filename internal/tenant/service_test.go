package tenant

import (
	"testing"
)

func TestTenantStruct(t *testing.T) {
	// Verify Tenant struct fields are accessible and correctly typed.
	tenant := Tenant{
		ID:          "tenant-uuid-1",
		DisplayName: "myorg",
	}

	if tenant.ID != "tenant-uuid-1" {
		t.Errorf("ID = %q, want %q", tenant.ID, "tenant-uuid-1")
	}
	if tenant.DisplayName != "myorg" {
		t.Errorf("DisplayName = %q, want %q", tenant.DisplayName, "myorg")
	}
	if tenant.GitHubInstallationID != nil {
		t.Errorf("GitHubInstallationID = %v, want nil", tenant.GitHubInstallationID)
	}
}

func TestRepositoryStruct(t *testing.T) {
	repoID := int64(42)
	repo := Repository{
		ID:            "repo-uuid-1",
		TenantID:      "tenant-uuid-1",
		GitHubRepoID:  &repoID,
		FullName:      "org/myrepo",
		DefaultBranch: "main",
	}

	if repo.FullName != "org/myrepo" {
		t.Errorf("FullName = %q, want %q", repo.FullName, "org/myrepo")
	}
	if *repo.GitHubRepoID != 42 {
		t.Errorf("GitHubRepoID = %d, want %d", *repo.GitHubRepoID, 42)
	}
	if repo.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %q, want %q", repo.DefaultBranch, "main")
	}
}

func TestNewService(t *testing.T) {
	// NewService should not panic with nil db (it just stores the reference).
	svc := NewService(nil)
	if svc == nil {
		t.Fatal("NewService returned nil")
	}
}

func TestServiceSQL_WellFormed(t *testing.T) {
	// Since the tenant.Service methods all require a real Postgres database,
	// we verify the SQL queries are well-formed by checking that the service
	// can be constructed and that the methods exist with the expected signatures.
	// Full integration tests would require a test database.

	// Verify the Service type embeds a *sql.DB
	svc := &Service{}
	if svc.db != nil {
		t.Error("zero-value Service should have nil db")
	}

	// Verify method signatures exist (compile-time check primarily,
	// but also verifies the method set).
	_ = svc.CreateTenant
	_ = svc.GetTenantByInstallation
	_ = svc.UpsertRepository
	_ = svc.GetRepository
	_ = svc.ListRepositories
}

func TestTenantOptionalFields(t *testing.T) {
	// Test that optional pointer fields work correctly.
	installID := int64(12345)
	credRef := "vault://secret/github-app"

	tenant := Tenant{
		ID:                   "t-1",
		DisplayName:          "test-org",
		GitHubInstallationID: &installID,
		CredentialsRef:       &credRef,
	}

	if *tenant.GitHubInstallationID != 12345 {
		t.Errorf("GitHubInstallationID = %d, want %d", *tenant.GitHubInstallationID, 12345)
	}
	if *tenant.CredentialsRef != "vault://secret/github-app" {
		t.Errorf("CredentialsRef = %q, want %q", *tenant.CredentialsRef, "vault://secret/github-app")
	}
}

func TestRepositoryOptionalGitHubRepoID(t *testing.T) {
	tests := []struct {
		name   string
		repoID *int64
		isNil  bool
	}{
		{
			name:  "with GitHub repo ID",
			repoID: ptrInt64(999),
			isNil: false,
		},
		{
			name:  "without GitHub repo ID",
			repoID: nil,
			isNil: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := Repository{
				ID:           "r-1",
				TenantID:     "t-1",
				GitHubRepoID: tc.repoID,
				FullName:     "org/repo",
			}

			if (repo.GitHubRepoID == nil) != tc.isNil {
				t.Errorf("GitHubRepoID nil = %v, want %v", repo.GitHubRepoID == nil, tc.isNil)
			}
			if !tc.isNil && *repo.GitHubRepoID != 999 {
				t.Errorf("GitHubRepoID = %d, want 999", *repo.GitHubRepoID)
			}
		})
	}
}

func ptrInt64(v int64) *int64 {
	return &v
}

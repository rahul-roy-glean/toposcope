package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/toposcope/toposcope/internal/ingestion"
	"github.com/toposcope/toposcope/internal/tenant"
)

// Handler processes incoming GitHub webhook events.
type Handler struct {
	webhookSecret []byte
	tenants       *tenant.Service
	ingestions    *ingestion.Service
}

// NewHandler creates a new webhook Handler.
func NewHandler(webhookSecret []byte, tenants *tenant.Service, ingestions *ingestion.Service) *Handler {
	return &Handler{
		webhookSecret: webhookSecret,
		tenants:       tenants,
		ingestions:    ingestions,
	}
}

// ServeHTTP handles incoming webhook requests.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20)) // 10 MB limit
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	signature := r.Header.Get("X-Hub-Signature-256")
	if err := VerifySignature(body, signature, h.webhookSecret); err != nil {
		log.Printf("webhook signature verification failed: %v", err)
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	eventType := r.Header.Get("X-GitHub-Event")
	if eventType == "" {
		http.Error(w, "missing X-GitHub-Event header", http.StatusBadRequest)
		return
	}

	event, err := ParseEvent(eventType, body)
	if err != nil {
		log.Printf("webhook parse error for %s: %v", eventType, err)
		http.Error(w, "unsupported event", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	switch e := event.(type) {
	case *InstallationEvent:
		if err := h.handleInstallation(ctx, e); err != nil {
			log.Printf("handle installation event: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

	case *InstallationRepositoriesEvent:
		if err := h.handleInstallationRepositories(ctx, e); err != nil {
			log.Printf("handle installation_repositories event: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

	case *PullRequestEvent:
		if err := h.handlePullRequest(ctx, e); err != nil {
			log.Printf("handle pull_request event: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

	case *PushEvent:
		if err := h.handlePush(ctx, e); err != nil {
			log.Printf("handle push event: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
}

func (h *Handler) handleInstallation(ctx context.Context, e *InstallationEvent) error {
	switch e.Action {
	case "created":
		_, err := h.tenants.CreateTenant(ctx, e.Installation.Account.Login, e.Installation.ID)
		if err != nil {
			return fmt.Errorf("create tenant for installation %d: %w", e.Installation.ID, err)
		}
		log.Printf("created tenant for installation %d (%s)", e.Installation.ID, e.Installation.Account.Login)
	case "deleted":
		log.Printf("installation %d deleted, tenant soft-delete not yet implemented", e.Installation.ID)
	}
	return nil
}

func (h *Handler) handleInstallationRepositories(ctx context.Context, e *InstallationRepositoriesEvent) error {
	t, err := h.tenants.GetTenantByInstallation(ctx, e.Installation.ID)
	if err != nil {
		return fmt.Errorf("get tenant for installation %d: %w", e.Installation.ID, err)
	}

	for _, repo := range e.RepositoriesAdded {
		repoID := repo.ID
		_, err := h.tenants.UpsertRepository(ctx, t.ID, repo.FullName, &repoID, repo.DefaultBranch)
		if err != nil {
			return fmt.Errorf("upsert repository %s: %w", repo.FullName, err)
		}
		log.Printf("added repository %s for tenant %s", repo.FullName, t.ID)
	}

	// Removed repos: log only for now (soft-delete not yet implemented)
	for _, repo := range e.RepositoriesRemoved {
		log.Printf("repository %s removed from installation %d (no-op)", repo.FullName, e.Installation.ID)
	}

	return nil
}

func (h *Handler) handlePullRequest(ctx context.Context, e *PullRequestEvent) error {
	switch e.Action {
	case "opened", "synchronize", "reopened":
	default:
		return nil // ignore other PR actions
	}

	t, err := h.tenants.GetTenantByInstallation(ctx, e.Installation.ID)
	if err != nil {
		return fmt.Errorf("get tenant: %w", err)
	}

	repo, err := h.tenants.GetRepository(ctx, t.ID, e.Repository.FullName)
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	req := ingestion.IngestionRequest{
		TenantID:       t.ID,
		RepoID:         repo.ID,
		RepoFullName:   e.Repository.FullName,
		CommitSHA:      e.PullRequest.Head.SHA,
		BaseBranch:     e.PullRequest.Base.Ref,
		PRNumber:       &e.Number,
		InstallationID: e.Installation.ID,
	}

	if _, err := h.ingestions.CreateIngestion(ctx, req); err != nil {
		return fmt.Errorf("create ingestion: %w", err)
	}

	log.Printf("enqueued ingestion for PR #%d on %s (commit %s)", e.Number, e.Repository.FullName, e.PullRequest.Head.SHA)
	return nil
}

func (h *Handler) handlePush(ctx context.Context, e *PushEvent) error {
	expectedRef := "refs/heads/" + e.Repository.DefaultBranch
	if e.Ref != expectedRef {
		return nil // only process pushes to default branch
	}

	t, err := h.tenants.GetTenantByInstallation(ctx, e.Installation.ID)
	if err != nil {
		return fmt.Errorf("get tenant: %w", err)
	}

	repo, err := h.tenants.GetRepository(ctx, t.ID, e.Repository.FullName)
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	req := ingestion.IngestionRequest{
		TenantID:       t.ID,
		RepoID:         repo.ID,
		RepoFullName:   e.Repository.FullName,
		CommitSHA:      e.After,
		BaseBranch:     e.Repository.DefaultBranch,
		InstallationID: e.Installation.ID,
	}

	if _, err := h.ingestions.CreateIngestion(ctx, req); err != nil {
		return fmt.Errorf("create ingestion: %w", err)
	}

	log.Printf("enqueued baseline ingestion for push to %s on %s (commit %s)", e.Repository.DefaultBranch, e.Repository.FullName, e.After)
	return nil
}

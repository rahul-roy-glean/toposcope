package api

import (
	"encoding/json"
	"net/http"
)

type updateRepoRequest struct {
	DefaultBranch string `json:"default_branch"`
}

func (h *Handler) handleUpdateRepo(w http.ResponseWriter, r *http.Request) {
	repoID := r.PathValue("repoID")

	var req updateRepoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.DefaultBranch == "" {
		writeError(w, http.StatusBadRequest, "default_branch is required")
		return
	}

	if err := h.tenantSvc.UpdateRepoDefaultBranch(r.Context(), repoID, req.DefaultBranch); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update repository: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *Handler) handleDeleteRepo(w http.ResponseWriter, r *http.Request) {
	repoID := r.PathValue("repoID")

	if err := h.tenantSvc.DeleteRepo(r.Context(), repoID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete repository: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

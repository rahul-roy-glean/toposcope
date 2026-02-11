package api

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/toposcope/toposcope/internal/tenant"
)

type repoResponse struct {
	ID            string `json:"id"`
	FullName      string `json:"full_name"`
	DefaultBranch string `json:"default_branch"`
}

type deltaStatsResponse struct {
	ImpactedTargets int `json:"impacted_targets"`
	AddedNodes      int `json:"added_nodes"`
	RemovedNodes    int `json:"removed_nodes"`
	AddedEdges      int `json:"added_edges"`
	RemovedEdges    int `json:"removed_edges"`
}

type scoreResponse struct {
	ID               string              `json:"id"`
	TotalScore       float64             `json:"total_score"`
	Grade            string              `json:"grade"`
	CommitSHA        string              `json:"commit_sha"`
	PRNumber         *int                `json:"pr_number,omitempty"`
	BaseSnapshotID   string              `json:"base_snapshot_id"`
	HeadSnapshotID   string              `json:"head_snapshot_id"`
	DeltaID          string              `json:"delta_id"`
	Breakdown        json.RawMessage     `json:"breakdown"`
	Hotspots         json.RawMessage     `json:"hotspots"`
	SuggestedActions json.RawMessage     `json:"suggested_actions"`
	DeltaStats       *deltaStatsResponse `json:"delta_stats,omitempty"`
	CreatedAt        string              `json:"created_at"`
}

func scoreRowToResponse(sc *tenant.ScoreRow) scoreResponse {
	resp := scoreResponse{
		ID:               sc.ID,
		TotalScore:       sc.TotalScore,
		Grade:            sc.Grade,
		CommitSHA:        sc.CommitSHA,
		PRNumber:         sc.PRNumber,
		BaseSnapshotID:   sc.BaseSnapshotID,
		HeadSnapshotID:   sc.HeadSnapshotID,
		DeltaID:          sc.DeltaID,
		Breakdown:        sc.Breakdown,
		Hotspots:         sc.Hotspots,
		SuggestedActions: sc.SuggestedActions,
		CreatedAt:        sc.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if sc.DeltaID != "" {
		resp.DeltaStats = &deltaStatsResponse{
			ImpactedTargets: sc.AddedNodes + sc.RemovedNodes,
			AddedNodes:      sc.AddedNodes,
			RemovedNodes:    sc.RemovedNodes,
			AddedEdges:      sc.AddedEdges,
			RemovedEdges:    sc.RemovedEdges,
		}
	}
	return resp
}

func (h *Handler) handleListRepos(w http.ResponseWriter, r *http.Request) {
	repos, err := h.tenantSvc.ListAllRepos(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, []repoResponse{})
		return
	}

	var result []repoResponse
	for _, repo := range repos {
		result = append(result, repoResponse{
			ID:            repo.ID,
			FullName:      repo.FullName,
			DefaultBranch: repo.DefaultBranch,
		})
	}

	if result == nil {
		result = []repoResponse{}
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleListScores(w http.ResponseWriter, r *http.Request) {
	repoID := r.PathValue("repoID")

	scores, err := h.tenantSvc.ListScoresByRepo(r.Context(), repoID)
	if err != nil {
		writeJSON(w, http.StatusOK, []scoreResponse{})
		return
	}

	var result []scoreResponse
	for i := range scores {
		result = append(result, scoreRowToResponse(&scores[i]))
	}

	if result == nil {
		result = []scoreResponse{}
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleGetScore(w http.ResponseWriter, r *http.Request) {
	scoreID := r.PathValue("scoreID")

	sc, err := h.tenantSvc.GetScoreByID(r.Context(), scoreID)
	if err != nil {
		writeError(w, http.StatusNotFound, "score not found")
		return
	}

	writeJSON(w, http.StatusOK, scoreRowToResponse(sc))
}

// Mapping from score file metric keys to the UI metric keys.
var metricKeyMap = map[string]string{
	"cross_package_deps": "m1_fan_in",
	"fanout_increase":    "m2_fan_out",
	"centrality_penalty": "m3_dep_depth",
	"blast_radius":       "m4_visibility",
	"cleanup_credits":    "m5_cycle",
}

type historyEntry struct {
	Date       string             `json:"date"`
	CommitSHA  string             `json:"commit_sha"`
	TotalScore float64            `json:"total_score"`
	Grade      string             `json:"grade"`
	Count      int                `json:"count"`
	Metrics    map[string]float64 `json:"metrics"`
}

func gradeForScore(score float64) string {
	switch {
	case score < 5:
		return "A"
	case score < 15:
		return "B"
	case score < 30:
		return "C"
	case score < 50:
		return "D"
	default:
		return "F"
	}
}

func (h *Handler) handleHistory(w http.ResponseWriter, r *http.Request) {
	repoID := r.PathValue("repoID")

	// Only show default branch scores in history (exclude PR analyses)
	scores, err := h.tenantSvc.ListDefaultBranchScores(r.Context(), repoID)
	if err != nil {
		writeJSON(w, http.StatusOK, []historyEntry{})
		return
	}

	// Aggregate by date: for each day, compute max score and sum metrics.
	type dayAgg struct {
		date      string
		commitSHA string // commit with the highest score
		maxScore  float64
		count     int
		metrics   map[string]float64
	}

	dayMap := make(map[string]*dayAgg)
	var dayOrder []string

	for _, sc := range scores {
		date := sc.CreatedAt.Format("2006-01-02")

		agg, exists := dayMap[date]
		if !exists {
			agg = &dayAgg{
				date:    date,
				metrics: make(map[string]float64),
			}
			dayMap[date] = agg
			dayOrder = append(dayOrder, date)
		}
		agg.count++

		// Track the commit with the highest score for this day
		if sc.TotalScore > agg.maxScore {
			agg.maxScore = sc.TotalScore
			agg.commitSHA = sc.CommitSHA
		}

		// Parse breakdown and accumulate max metric values per day
		var breakdown []struct {
			Key          string  `json:"key"`
			Contribution float64 `json:"contribution"`
		}
		_ = json.Unmarshal(sc.Breakdown, &breakdown)

		for _, b := range breakdown {
			if uiKey, ok := metricKeyMap[b.Key]; ok {
				abs := b.Contribution
				if abs < 0 {
					abs = -abs
				}
				if abs > agg.metrics[uiKey] {
					agg.metrics[uiKey] = abs
				}
			}
		}
	}

	// Sort by date ascending (oldest first for charts)
	sort.Strings(dayOrder)

	history := make([]historyEntry, 0, len(dayOrder))
	for _, date := range dayOrder {
		agg := dayMap[date]
		history = append(history, historyEntry{
			Date:       agg.date,
			CommitSHA:  agg.commitSHA,
			TotalScore: agg.maxScore,
			Grade:      gradeForScore(agg.maxScore),
			Count:      agg.count,
			Metrics:    agg.metrics,
		})
	}

	writeJSON(w, http.StatusOK, history)
}

func (h *Handler) handlePRImpact(w http.ResponseWriter, r *http.Request) {
	repoID := r.PathValue("repoID")
	prStr := r.PathValue("prNumber")
	prNumber, err := strconv.Atoi(prStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid pr number")
		return
	}

	sc, err := h.tenantSvc.GetScoreByPR(r.Context(), repoID, prNumber)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			writeError(w, http.StatusNotFound, "no score found for PR")
		} else {
			writeError(w, http.StatusInternalServerError, "failed to query score")
		}
		return
	}

	writeJSON(w, http.StatusOK, scoreRowToResponse(sc))
}

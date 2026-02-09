package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/toposcope/toposcope/pkg/config"
	"github.com/toposcope/toposcope/pkg/graph"
	"github.com/toposcope/toposcope/pkg/graphquery"
)

func newUICmd() *cobra.Command {
	var (
		repoPath string
		port     string
	)

	cmd := &cobra.Command{
		Use:   "ui",
		Short: "Start a local API server for the Toposcope web UI",
		Long: `Starts an HTTP server on localhost that serves snapshot and score data
from the local cache. Point the Next.js web UI at this server.

Usage:
  1. Start the API server:  toposcope ui --repo-path /path/to/repo
  2. In another terminal:   cd web && NEXT_PUBLIC_API_MODE=local pnpm dev
  3. Open http://localhost:3000`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUI(repoPath, port)
		},
	}

	cmd.Flags().StringVar(&repoPath, "repo-path", "", "Path to repository root (default: detect workspace)")
	cmd.Flags().StringVar(&port, "port", "7700", "Port to serve on")

	return cmd
}

func runUI(repoPath, port string) error {
	wsRoot, err := resolveWorkspace(repoPath)
	if err != nil {
		return err
	}

	repoName := filepath.Base(wsRoot)
	snapDir := config.SnapshotDir(wsRoot)

	// Detect default branch from git
	defaultBranch := detectDefaultBranch(wsRoot)

	srv := &localAPIServer{
		wsRoot:        wsRoot,
		repoName:      repoName,
		snapDir:       snapDir,
		defaultBranch: defaultBranch,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/repos", srv.handleRepos)
	mux.HandleFunc("/api/repos/", srv.handleRepoRoutes)
	mux.HandleFunc("/api/snapshots/", srv.handleSnapshots)

	// CORS middleware for Next.js dev server
	handler := corsMiddleware(mux)

	fmt.Fprintf(os.Stderr, "Toposcope API server\n")
	fmt.Fprintf(os.Stderr, "  Repo:       %s\n", wsRoot)
	fmt.Fprintf(os.Stderr, "  Snapshots:  %s\n", snapDir)
	fmt.Fprintf(os.Stderr, "  Listening:  http://localhost:%s\n", port)
	fmt.Fprintf(os.Stderr, "\nStart the web UI:  cd web && NEXT_PUBLIC_API_MODE=local pnpm dev\n")

	return http.ListenAndServe(":"+port, handler)
}

type localAPIServer struct {
	wsRoot        string
	repoName      string
	snapDir       string
	defaultBranch string
}

func (s *localAPIServer) handleRepos(w http.ResponseWriter, r *http.Request) {
	repos := []map[string]string{
		{
			"id":             "local",
			"full_name":      s.repoName,
			"default_branch": s.defaultBranch,
		},
	}
	writeJSON(w, repos)
}

func (s *localAPIServer) handleRepoRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/repos/")
	parts := strings.Split(path, "/")

	// /api/repos/{repoId}/scores/{scoreId}
	if len(parts) >= 3 && parts[1] == "scores" {
		s.handleScoreDetail(w, r, parts[2])
		return
	}

	// /api/repos/{repoId}/scores
	if len(parts) >= 2 && parts[1] == "scores" {
		s.handleScores(w, r)
		return
	}

	// /api/repos/{repoId}/history
	if len(parts) >= 2 && parts[1] == "history" {
		s.handleHistory(w, r)
		return
	}

	// /api/repos/{repoId}
	if len(parts) == 1 {
		s.handleRepos(w, r)
		return
	}

	http.NotFound(w, r)
}

func (s *localAPIServer) handleScores(w http.ResponseWriter, r *http.Request) {
	scoreDir := config.ScoreDir(s.wsRoot)
	entries, err := os.ReadDir(scoreDir)
	if err != nil {
		writeJSON(w, []interface{}{})
		return
	}

	var scores []json.RawMessage
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(scoreDir, e.Name()))
		if err != nil {
			continue
		}
		scores = append(scores, json.RawMessage(data))
	}

	if scores == nil {
		writeJSON(w, []interface{}{})
		return
	}
	writeJSON(w, scores)
}

func (s *localAPIServer) handleScoreDetail(w http.ResponseWriter, r *http.Request, scoreID string) {
	scoreDir := config.ScoreDir(s.wsRoot)

	// Try exact filename match first
	path := filepath.Join(scoreDir, scoreID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		// Try prefix match: scoreID might be "abc123_def456" (short prefixes)
		// while filename is "abc123full_def456full.json" (full SHAs)
		entries, readErr := os.ReadDir(scoreDir)
		if readErr != nil {
			http.NotFound(w, r)
			return
		}

		// Split the scoreID on underscore to get base and head prefixes
		idParts := strings.SplitN(scoreID, "_", 2)

		for _, e := range entries {
			name := strings.TrimSuffix(e.Name(), ".json")
			matched := false
			if name == scoreID || strings.HasPrefix(name, scoreID) {
				matched = true
			} else if len(idParts) == 2 {
				// Match if filename contains both the base and head SHA prefixes
				nameParts := strings.SplitN(name, "_", 2)
				if len(nameParts) == 2 &&
					strings.HasPrefix(nameParts[0], idParts[0]) &&
					strings.HasPrefix(nameParts[1], idParts[1]) {
					matched = true
				}
			}
			if matched {
				data, err = os.ReadFile(filepath.Join(scoreDir, e.Name()))
				if err == nil {
					break
				}
			}
		}
		if data == nil {
			http.NotFound(w, r)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}

func (s *localAPIServer) handleHistory(w http.ResponseWriter, r *http.Request) {
	scoreDir := config.ScoreDir(s.wsRoot)
	entries, err := os.ReadDir(scoreDir)
	if err != nil {
		writeJSON(w, []interface{}{})
		return
	}

	// Mapping from score file metric keys to the UI metric keys
	metricKeyMap := map[string]string{
		"cross_package_deps": "m1_fan_in",
		"fanout_increase":    "m2_fan_out",
		"centrality_penalty": "m3_dep_depth",
		"blast_radius":       "m4_visibility",
		"cleanup_credits":    "m5_cycle",
	}

	type historyEntry struct {
		Date       string             `json:"date"`
		TotalScore float64            `json:"total_score"`
		Grade      string             `json:"grade"`
		Metrics    map[string]float64 `json:"metrics"`
	}

	var history []historyEntry
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(scoreDir, e.Name()))
		if err != nil {
			continue
		}

		var score struct {
			TotalScore float64 `json:"total_score"`
			Grade      string  `json:"grade"`
			AnalyzedAt string  `json:"analyzed_at"`
			Breakdown  []struct {
				Key          string  `json:"key"`
				Contribution float64 `json:"contribution"`
			} `json:"breakdown"`
		}
		if err := json.Unmarshal(data, &score); err != nil {
			continue
		}
		if score.AnalyzedAt == "" {
			continue
		}

		// Extract date portion from analyzed_at (e.g. "2026-02-08T18:09:47Z" -> "2026-02-08")
		date := score.AnalyzedAt
		if idx := strings.IndexByte(date, 'T'); idx > 0 {
			date = date[:idx]
		}

		metrics := make(map[string]float64)
		for _, b := range score.Breakdown {
			if uiKey, ok := metricKeyMap[b.Key]; ok {
				metrics[uiKey] = b.Contribution
			}
		}

		history = append(history, historyEntry{
			Date:       date,
			TotalScore: score.TotalScore,
			Grade:      score.Grade,
			Metrics:    metrics,
		})
	}

	// Sort by date descending (newest first)
	sort.Slice(history, func(i, j int) bool {
		return history[i].Date > history[j].Date
	})

	if history == nil {
		writeJSON(w, []interface{}{})
		return
	}
	writeJSON(w, history)
}

func (s *localAPIServer) handleSnapshots(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/snapshots/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		// List available snapshots
		s.listSnapshots(w, r)
		return
	}

	snapshotID := parts[0]

	// /api/snapshots/{id}/subgraph?root=...&depth=...
	if len(parts) >= 2 && parts[1] == "subgraph" {
		s.handleSubgraph(w, r, snapshotID)
		return
	}

	// /api/snapshots/{id}/packages?hide_tests=true&hide_external=true&min_edge_weight=1
	if len(parts) >= 2 && parts[1] == "packages" {
		s.handlePackages(w, r, snapshotID)
		return
	}

	// /api/snapshots/{id}/ego?target=...&depth=...&direction=...
	if len(parts) >= 2 && parts[1] == "ego" {
		s.handleEgo(w, r, snapshotID)
		return
	}

	// /api/snapshots/{id}/path?from=...&to=...&max_paths=10
	if len(parts) >= 2 && parts[1] == "path" {
		s.handlePath(w, r, snapshotID)
		return
	}

	// /api/snapshots/{id} — return full snapshot
	s.handleGetSnapshot(w, r, snapshotID)
}

func (s *localAPIServer) listSnapshots(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(s.snapDir)
	if err != nil {
		writeJSON(w, []interface{}{})
		return
	}

	type snapInfo struct {
		ID        string `json:"id"`
		CommitSHA string `json:"commit_sha"`
		Nodes     int    `json:"node_count"`
		Edges     int    `json:"edge_count"`
		Packages  int    `json:"package_count"`
	}

	var snaps []snapInfo
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		sha := strings.TrimSuffix(e.Name(), ".json")
		snap, err := graph.LoadSnapshot(filepath.Join(s.snapDir, e.Name()))
		if err != nil {
			continue
		}
		snaps = append(snaps, snapInfo{
			ID:        snap.ID,
			CommitSHA: sha,
			Nodes:     snap.Stats.NodeCount,
			Edges:     snap.Stats.EdgeCount,
			Packages:  snap.Stats.PackageCount,
		})
	}

	writeJSON(w, snaps)
}

func (s *localAPIServer) handleGetSnapshot(w http.ResponseWriter, r *http.Request, id string) {
	snap := s.findSnapshot(id)
	if snap == nil {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, snap)
}

func (s *localAPIServer) handleSubgraph(w http.ResponseWriter, r *http.Request, snapshotID string) {
	snap := s.findSnapshot(snapshotID)
	if snap == nil {
		http.NotFound(w, r)
		return
	}

	roots := r.URL.Query()["root"]
	depthStr := r.URL.Query().Get("depth")
	depth := 2
	if depthStr != "" {
		_, _ = fmt.Sscanf(depthStr, "%d", &depth)
	}

	// If no roots specified, return the full graph (capped at 500 nodes for UI performance)
	if len(roots) == 0 {
		result := graphquery.CapGraph(snap, 500)
		writeJSON(w, result)
		return
	}

	// BFS from roots to given depth
	result := graphquery.ExtractSubgraph(snap, roots, depth)
	writeJSON(w, result)
}

func (s *localAPIServer) handlePackages(w http.ResponseWriter, r *http.Request, snapshotID string) {
	snap := s.findSnapshot(snapshotID)
	if snap == nil {
		http.NotFound(w, r)
		return
	}

	hideTests := r.URL.Query().Get("hide_tests") == "true"
	hideExternal := r.URL.Query().Get("hide_external") == "true"
	minEdgeWeight := 1
	if v := r.URL.Query().Get("min_edge_weight"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			minEdgeWeight = parsed
		}
	}

	result := graphquery.AggregatePackages(snap, hideTests, hideExternal, minEdgeWeight, 0)
	writeJSON(w, result)
}

func (s *localAPIServer) handleEgo(w http.ResponseWriter, r *http.Request, snapshotID string) {
	snap := s.findSnapshot(snapshotID)
	if snap == nil {
		http.NotFound(w, r)
		return
	}

	target := r.URL.Query().Get("target")
	if target == "" {
		http.Error(w, "target parameter required", http.StatusBadRequest)
		return
	}

	depth := 2
	if v := r.URL.Query().Get("depth"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			depth = parsed
		}
	}

	direction := r.URL.Query().Get("direction")
	if direction == "" {
		direction = "both"
	}

	result := graphquery.EgoGraph(snap, target, depth, direction, 0)
	writeJSON(w, result)
}

func (s *localAPIServer) handlePath(w http.ResponseWriter, r *http.Request, snapshotID string) {
	snap := s.findSnapshot(snapshotID)
	if snap == nil {
		http.NotFound(w, r)
		return
	}

	fromQ := r.URL.Query().Get("from")
	toQ := r.URL.Query().Get("to")
	if fromQ == "" || toQ == "" {
		http.Error(w, "from and to parameters required", http.StatusBadRequest)
		return
	}

	maxPaths := 10
	if v := r.URL.Query().Get("max_paths"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			maxPaths = parsed
		}
	}

	result := graphquery.FindPaths(snap, fromQ, toQ, maxPaths)
	writeJSON(w, result)
}

// findSnapshot looks up a snapshot by ID or commit SHA prefix.
func (s *localAPIServer) findSnapshot(id string) *graph.Snapshot {
	// Try exact SHA match first
	path := filepath.Join(s.snapDir, id+".json")
	if snap, err := graph.LoadSnapshot(path); err == nil {
		return snap
	}

	// Try SHA prefix match
	entries, err := os.ReadDir(s.snapDir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".json")
		if strings.HasPrefix(name, id) {
			if snap, err := graph.LoadSnapshot(filepath.Join(s.snapDir, e.Name())); err == nil {
				return snap
			}
		}
	}

	return nil
}

// detectDefaultBranch uses git to find the default branch name.
func detectDefaultBranch(repoPath string) string {
	// Try symbolic-ref of origin/HEAD first
	out, err := exec.Command("git", "-C", repoPath, "symbolic-ref", "refs/remotes/origin/HEAD").Output()
	if err == nil {
		ref := strings.TrimSpace(string(out))
		// refs/remotes/origin/master -> master
		parts := strings.Split(ref, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
	}

	// Fallback: check if master or main exists
	for _, branch := range []string{"master", "main"} {
		if err := exec.Command("git", "-C", repoPath, "rev-parse", "--verify", branch).Run(); err == nil {
			return branch
		}
	}

	return "main"
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(data)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

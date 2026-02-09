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
	w.Write(data)
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
		fmt.Sscanf(depthStr, "%d", &depth)
	}

	// If no roots specified, return the full graph (capped at 500 nodes for UI performance)
	if len(roots) == 0 {
		sub := capGraph(snap, 500)
		writeJSON(w, sub)
		return
	}

	// BFS from roots to given depth
	sub := extractSubgraph(snap, roots, depth)
	writeJSON(w, sub)
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

// extractSubgraph does BFS from roots to depth, collecting nodes and edges.
func extractSubgraph(snap *graph.Snapshot, roots []string, depth int) map[string]interface{} {
	// Build adjacency maps
	fwd := make(map[string][]graph.Edge) // outgoing
	rev := make(map[string][]graph.Edge) // incoming
	for _, e := range snap.Edges {
		fwd[e.From] = append(fwd[e.From], e)
		rev[e.To] = append(rev[e.To], e)
	}

	visited := make(map[string]bool)
	queue := make([]string, 0, len(roots))

	for _, r := range roots {
		// Support prefix matching
		for key := range snap.Nodes {
			if key == r || strings.HasPrefix(key, r) {
				if !visited[key] {
					visited[key] = true
					queue = append(queue, key)
				}
			}
		}
	}

	// BFS both directions
	for d := 0; d < depth && len(queue) > 0; d++ {
		var next []string
		for _, node := range queue {
			for _, e := range fwd[node] {
				if !visited[e.To] {
					visited[e.To] = true
					next = append(next, e.To)
				}
			}
			for _, e := range rev[node] {
				if !visited[e.From] {
					visited[e.From] = true
					next = append(next, e.From)
				}
			}
		}
		queue = next
	}

	// Collect nodes and edges within the visited set
	nodes := make(map[string]*graph.Node)
	var edges []graph.Edge

	for key := range visited {
		if n, ok := snap.Nodes[key]; ok {
			nodes[key] = n
		}
	}
	for _, e := range snap.Edges {
		if visited[e.From] && visited[e.To] {
			edges = append(edges, e)
		}
	}

	return map[string]interface{}{
		"nodes": nodes,
		"edges": edges,
	}
}

// capGraph returns a subset of the graph with at most maxNodes nodes,
// preferring high-degree nodes (most connected = most interesting).
func capGraph(snap *graph.Snapshot, maxNodes int) map[string]interface{} {
	if len(snap.Nodes) <= maxNodes {
		return map[string]interface{}{
			"nodes": snap.Nodes,
			"edges": snap.Edges,
		}
	}

	// Rank nodes by total degree
	degree := make(map[string]int)
	for _, e := range snap.Edges {
		degree[e.From]++
		degree[e.To]++
	}

	type ranked struct {
		key string
		deg int
	}
	var rankedNodes []ranked
	for key := range snap.Nodes {
		rankedNodes = append(rankedNodes, ranked{key, degree[key]})
	}
	sort.Slice(rankedNodes, func(i, j int) bool {
		return rankedNodes[i].deg > rankedNodes[j].deg
	})

	keep := make(map[string]bool)
	for i := 0; i < maxNodes && i < len(rankedNodes); i++ {
		keep[rankedNodes[i].key] = true
	}

	nodes := make(map[string]*graph.Node)
	for key := range keep {
		nodes[key] = snap.Nodes[key]
	}

	var edges []graph.Edge
	for _, e := range snap.Edges {
		if keep[e.From] && keep[e.To] {
			edges = append(edges, e)
		}
	}

	return map[string]interface{}{
		"nodes": nodes,
		"edges": edges,
	}
}

// PackageNode represents an aggregated package in the package-level graph.
type PackageNode struct {
	Package     string   `json:"package"`
	TargetCount int      `json:"target_count"`
	Kinds       []string `json:"kinds"`
	HasTests    bool     `json:"has_tests"`
	IsExternal  bool     `json:"is_external"`
}

// PackageEdge represents an aggregated edge between packages.
type PackageEdge struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Weight int    `json:"weight"`
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

	// Aggregate targets into packages
	pkgNodes := make(map[string]*PackageNode)
	for _, node := range snap.Nodes {
		if hideTests && node.IsTest {
			continue
		}
		if hideExternal && node.IsExternal {
			continue
		}
		pkg := node.Package
		if pkg == "" {
			continue
		}
		pn, ok := pkgNodes[pkg]
		if !ok {
			pn = &PackageNode{
				Package:    pkg,
				IsExternal: node.IsExternal,
			}
			pkgNodes[pkg] = pn
		}
		pn.TargetCount++
		if node.IsTest {
			pn.HasTests = true
		}
		// Track unique kinds
		found := false
		for _, k := range pn.Kinds {
			if k == node.Kind {
				found = true
				break
			}
		}
		if !found {
			pn.Kinds = append(pn.Kinds, node.Kind)
		}
	}

	// Build set of included target keys for edge filtering
	includedTargets := make(map[string]bool)
	for _, node := range snap.Nodes {
		if hideTests && node.IsTest {
			continue
		}
		if hideExternal && node.IsExternal {
			continue
		}
		if node.Package != "" && pkgNodes[node.Package] != nil {
			includedTargets[node.Key] = true
		}
	}

	// Aggregate edges between packages
	edgeWeight := make(map[string]int) // "fromPkg|toPkg" -> count
	for _, e := range snap.Edges {
		if !includedTargets[e.From] || !includedTargets[e.To] {
			continue
		}
		fromNode := snap.Nodes[e.From]
		toNode := snap.Nodes[e.To]
		if fromNode == nil || toNode == nil {
			continue
		}
		fromPkg := fromNode.Package
		toPkg := toNode.Package
		if fromPkg == toPkg || fromPkg == "" || toPkg == "" {
			continue
		}
		edgeWeight[fromPkg+"|"+toPkg]++
	}

	pkgEdges := make([]PackageEdge, 0)
	for key, weight := range edgeWeight {
		if weight < minEdgeWeight {
			continue
		}
		parts := strings.SplitN(key, "|", 2)
		pkgEdges = append(pkgEdges, PackageEdge{
			From:   parts[0],
			To:     parts[1],
			Weight: weight,
		})
	}

	// Cap to top 500 packages by degree if too many
	maxPkgs := 500
	if len(pkgNodes) > maxPkgs {
		pkgDegree := make(map[string]int)
		for _, e := range pkgEdges {
			pkgDegree[e.From]++
			pkgDegree[e.To]++
		}
		type rankedPkg struct {
			pkg string
			deg int
		}
		var ranked []rankedPkg
		for pkg := range pkgNodes {
			ranked = append(ranked, rankedPkg{pkg, pkgDegree[pkg]})
		}
		sort.Slice(ranked, func(i, j int) bool {
			return ranked[i].deg > ranked[j].deg
		})
		keep := make(map[string]bool)
		for i := 0; i < maxPkgs && i < len(ranked); i++ {
			keep[ranked[i].pkg] = true
		}
		for pkg := range pkgNodes {
			if !keep[pkg] {
				delete(pkgNodes, pkg)
			}
		}
		filteredEdges := make([]PackageEdge, 0)
		for _, e := range pkgEdges {
			if keep[e.From] && keep[e.To] {
				filteredEdges = append(filteredEdges, e)
			}
		}
		pkgEdges = filteredEdges
	}

	writeJSON(w, map[string]interface{}{
		"nodes":     pkgNodes,
		"edges":     pkgEdges,
		"truncated": len(snap.Packages()) > len(pkgNodes),
	})
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

	// Build adjacency maps
	fwd := make(map[string][]graph.Edge)
	rev := make(map[string][]graph.Edge)
	for _, e := range snap.Edges {
		fwd[e.From] = append(fwd[e.From], e)
		rev[e.To] = append(rev[e.To], e)
	}

	// Find matching root nodes (exact or prefix match)
	visited := make(map[string]bool)
	var queue []string
	for key := range snap.Nodes {
		if key == target || strings.HasPrefix(key, target+":") || strings.HasPrefix(key, target+"/") {
			if !visited[key] {
				visited[key] = true
				queue = append(queue, key)
			}
		}
	}

	// Also match as package
	if len(queue) == 0 {
		for key, node := range snap.Nodes {
			if node.Package == target {
				if !visited[key] {
					visited[key] = true
					queue = append(queue, key)
				}
			}
		}
	}

	if len(queue) == 0 {
		writeJSON(w, map[string]interface{}{
			"nodes":     map[string]*graph.Node{},
			"edges":     []graph.Edge{},
			"truncated": false,
		})
		return
	}

	maxNodes := 500
	truncated := false

	// BFS with direction control
	for d := 0; d < depth && len(queue) > 0; d++ {
		var next []string
		for _, node := range queue {
			if direction == "deps" || direction == "both" {
				for _, e := range fwd[node] {
					if !visited[e.To] {
						visited[e.To] = true
						next = append(next, e.To)
					}
				}
			}
			if direction == "rdeps" || direction == "both" {
				for _, e := range rev[node] {
					if !visited[e.From] {
						visited[e.From] = true
						next = append(next, e.From)
					}
				}
			}
		}
		queue = next

		if len(visited) >= maxNodes {
			truncated = true
			break
		}
	}

	// Collect nodes and edges
	nodes := make(map[string]*graph.Node)
	for key := range visited {
		if n, ok := snap.Nodes[key]; ok {
			nodes[key] = n
		}
	}

	var edges []graph.Edge
	for _, e := range snap.Edges {
		if visited[e.From] && visited[e.To] {
			edges = append(edges, e)
		}
	}

	writeJSON(w, map[string]interface{}{
		"nodes":     nodes,
		"edges":     edges,
		"truncated": truncated,
	})
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

	// Build forward adjacency map
	fwd := make(map[string][]string)
	for _, e := range snap.Edges {
		fwd[e.From] = append(fwd[e.From], e.To)
	}

	// Resolve "from" nodes (exact, prefix, or package match)
	resolveNodes := func(query string) []string {
		var matches []string
		for key := range snap.Nodes {
			if key == query || strings.HasPrefix(key, query+":") || strings.HasPrefix(key, query+"/") {
				matches = append(matches, key)
			}
		}
		if len(matches) == 0 {
			for key, node := range snap.Nodes {
				if node.Package == query {
					matches = append(matches, key)
				}
			}
		}
		return matches
	}

	fromNodes := resolveNodes(fromQ)
	toNodes := resolveNodes(toQ)

	if len(fromNodes) == 0 || len(toNodes) == 0 {
		writeJSON(w, map[string]interface{}{
			"paths":       [][]string{},
			"nodes":       map[string]*graph.Node{},
			"edges":       []graph.Edge{},
			"from":        fromQ,
			"to":          toQ,
			"path_length": 0,
		})
		return
	}

	toSet := make(map[string]bool)
	for _, n := range toNodes {
		toSet[n] = true
	}

	// BFS from fromNodes, tracking parents for shortest-path reconstruction
	type bfsEntry struct {
		node  string
		depth int
	}
	parents := make(map[string][]string) // node -> list of parent nodes at shortest distance
	dist := make(map[string]int)         // node -> BFS depth

	var queue []bfsEntry
	for _, n := range fromNodes {
		dist[n] = 0
		queue = append(queue, bfsEntry{n, 0})
	}

	foundDepth := -1

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		// If we've already found target nodes and we're past that depth, stop
		if foundDepth >= 0 && curr.depth > foundDepth {
			break
		}

		if toSet[curr.node] {
			foundDepth = curr.depth
		}

		for _, neighbor := range fwd[curr.node] {
			nextDepth := curr.depth + 1
			if _, seen := dist[neighbor]; !seen {
				dist[neighbor] = nextDepth
				parents[neighbor] = []string{curr.node}
				queue = append(queue, bfsEntry{neighbor, nextDepth})
			} else if dist[neighbor] == nextDepth {
				// Same shortest distance — add as additional parent
				parents[neighbor] = append(parents[neighbor], curr.node)
			}
		}
	}

	// Find which target nodes were reached
	var reachedTargets []string
	for _, n := range toNodes {
		if _, ok := dist[n]; ok {
			reachedTargets = append(reachedTargets, n)
		}
	}

	if len(reachedTargets) == 0 {
		writeJSON(w, map[string]interface{}{
			"paths":       [][]string{},
			"nodes":       map[string]*graph.Node{},
			"edges":       []graph.Edge{},
			"from":        fromQ,
			"to":          toQ,
			"path_length": 0,
		})
		return
	}

	// Backtrack from reached targets through parents to enumerate all shortest paths
	fromSet := make(map[string]bool)
	for _, n := range fromNodes {
		fromSet[n] = true
	}

	var allPaths [][]string
	var backtrack func(node string, path []string)
	backtrack = func(node string, path []string) {
		if len(allPaths) >= maxPaths {
			return
		}
		current := make([]string, len(path)+1)
		current[0] = node
		copy(current[1:], path)

		if fromSet[node] {
			allPaths = append(allPaths, current)
			return
		}
		for _, p := range parents[node] {
			backtrack(p, current)
		}
	}

	for _, target := range reachedTargets {
		if len(allPaths) >= maxPaths {
			break
		}
		backtrack(target, nil)
	}

	// Collect all nodes and edges on the paths
	pathNodes := make(map[string]bool)
	pathEdgeSet := make(map[string]bool)
	for _, p := range allPaths {
		for _, n := range p {
			pathNodes[n] = true
		}
		for i := 0; i < len(p)-1; i++ {
			pathEdgeSet[p[i]+"->"+p[i+1]] = true
		}
	}

	resultNodes := make(map[string]*graph.Node)
	for key := range pathNodes {
		if n, ok := snap.Nodes[key]; ok {
			resultNodes[key] = n
		}
	}

	var resultEdges []graph.Edge
	for _, e := range snap.Edges {
		if pathEdgeSet[e.From+"->"+e.To] {
			resultEdges = append(resultEdges, e)
		}
	}

	pathLength := 0
	if len(allPaths) > 0 {
		pathLength = len(allPaths[0]) - 1
	}

	writeJSON(w, map[string]interface{}{
		"paths":       allPaths,
		"nodes":       resultNodes,
		"edges":       resultEdges,
		"from":        fromQ,
		"to":          toQ,
		"path_length": pathLength,
	})
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
	enc.Encode(data)
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

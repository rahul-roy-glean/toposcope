// Package graphquery provides shared graph algorithms for querying Toposcope
// dependency graph snapshots. Used by both the local CLI server and the
// hosted platform API.
package graphquery

import (
	"sort"
	"strings"

	"github.com/toposcope/toposcope/pkg/graph"
)

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

// SubgraphResult holds the result of a subgraph extraction or ego graph query.
type SubgraphResult struct {
	Nodes     map[string]*graph.Node `json:"nodes"`
	Edges     []graph.Edge           `json:"edges"`
	Truncated bool                   `json:"truncated,omitempty"`
}

// PackageGraphResult holds the result of a package-level graph aggregation.
type PackageGraphResult struct {
	Nodes     map[string]*PackageNode `json:"nodes"`
	Edges     []PackageEdge           `json:"edges"`
	Truncated bool                    `json:"truncated"`
}

// PathResult holds the result of a shortest-path query.
type PathResult struct {
	Paths      [][]string             `json:"paths"`
	Nodes      map[string]*graph.Node `json:"nodes"`
	Edges      []graph.Edge           `json:"edges"`
	From       string                 `json:"from"`
	To         string                 `json:"to"`
	PathLength int                    `json:"path_length"`
}

// ExtractSubgraph does BFS from roots to depth, collecting nodes and edges
// in both directions. Roots support prefix matching against node keys.
func ExtractSubgraph(snap *graph.Snapshot, roots []string, depth int) *SubgraphResult {
	fwd := make(map[string][]graph.Edge)
	rev := make(map[string][]graph.Edge)
	for _, e := range snap.Edges {
		fwd[e.From] = append(fwd[e.From], e)
		rev[e.To] = append(rev[e.To], e)
	}

	visited := make(map[string]bool)
	queue := make([]string, 0, len(roots))

	for _, r := range roots {
		for key := range snap.Nodes {
			if key == r || strings.HasPrefix(key, r) {
				if !visited[key] {
					visited[key] = true
					queue = append(queue, key)
				}
			}
		}
	}

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

	return &SubgraphResult{Nodes: nodes, Edges: edges}
}

// CapGraph returns a subset of the graph with at most maxNodes nodes,
// preferring high-degree nodes (most connected = most interesting).
func CapGraph(snap *graph.Snapshot, maxNodes int) *SubgraphResult {
	if len(snap.Nodes) <= maxNodes {
		return &SubgraphResult{
			Nodes: snap.Nodes,
			Edges: snap.Edges,
		}
	}

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

	return &SubgraphResult{Nodes: nodes, Edges: edges}
}

// EgoGraph computes the ego graph (neighborhood) of a target node with
// directional control. Direction can be "deps", "rdeps", or "both".
// maxNodes caps the result size (0 means no cap).
func EgoGraph(snap *graph.Snapshot, target string, depth int, direction string, maxNodes int) *SubgraphResult {
	if direction == "" {
		direction = "both"
	}
	if maxNodes == 0 {
		maxNodes = 500
	}

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
		return &SubgraphResult{
			Nodes: map[string]*graph.Node{},
			Edges: []graph.Edge{},
		}
	}

	truncated := false

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

	return &SubgraphResult{
		Nodes:     nodes,
		Edges:     edges,
		Truncated: truncated,
	}
}

// FindPaths finds all shortest paths between from and to node queries.
// Queries support exact match, prefix match, and package match.
func FindPaths(snap *graph.Snapshot, fromQ, toQ string, maxPaths int) *PathResult {
	if maxPaths <= 0 {
		maxPaths = 10
	}

	fwd := make(map[string][]string)
	for _, e := range snap.Edges {
		fwd[e.From] = append(fwd[e.From], e.To)
	}

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

	emptyResult := &PathResult{
		Paths:      [][]string{},
		Nodes:      map[string]*graph.Node{},
		Edges:      []graph.Edge{},
		From:       fromQ,
		To:         toQ,
		PathLength: 0,
	}

	if len(fromNodes) == 0 || len(toNodes) == 0 {
		return emptyResult
	}

	toSet := make(map[string]bool)
	for _, n := range toNodes {
		toSet[n] = true
	}

	type bfsEntry struct {
		node  string
		depth int
	}
	parents := make(map[string][]string)
	dist := make(map[string]int)

	var queue []bfsEntry
	for _, n := range fromNodes {
		dist[n] = 0
		queue = append(queue, bfsEntry{n, 0})
	}

	foundDepth := -1

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

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
				parents[neighbor] = append(parents[neighbor], curr.node)
			}
		}
	}

	var reachedTargets []string
	for _, n := range toNodes {
		if _, ok := dist[n]; ok {
			reachedTargets = append(reachedTargets, n)
		}
	}

	if len(reachedTargets) == 0 {
		return emptyResult
	}

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

	return &PathResult{
		Paths:      allPaths,
		Nodes:      resultNodes,
		Edges:      resultEdges,
		From:       fromQ,
		To:         toQ,
		PathLength: pathLength,
	}
}

// AggregatePackages aggregates the target-level graph into a package-level
// graph with optional filtering. maxPkgs caps the number of packages (0 = 500 default).
func AggregatePackages(snap *graph.Snapshot, hideTests, hideExternal bool, minEdgeWeight, maxPkgs int) *PackageGraphResult {
	if minEdgeWeight < 1 {
		minEdgeWeight = 1
	}
	if maxPkgs <= 0 {
		maxPkgs = 500
	}

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

	edgeWeight := make(map[string]int)
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

	truncated := false
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
		truncated = true
	}

	// Check if we filtered any packages compared to total
	if !truncated {
		truncated = len(snap.Packages()) > len(pkgNodes)
	}

	return &PackageGraphResult{
		Nodes:     pkgNodes,
		Edges:     pkgEdges,
		Truncated: truncated,
	}
}

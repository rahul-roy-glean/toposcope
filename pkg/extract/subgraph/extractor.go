// Package subgraph extracts structural subgraphs from a Bazel workspace
// by running bazel query/cquery and parsing the XML output.
package subgraph

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/toposcope/toposcope/pkg/extract"
	"github.com/toposcope/toposcope/pkg/graph"
)

// maxQueryLabelLength is the max total label length before splitting into chunks.
const maxQueryLabelLength = 75000

// Extractor runs bazel query to extract structural neighborhoods.
type Extractor struct {
	WorkspacePath string
	BazelPath     string
	BazelRC       string
	UseCQuery     bool
}

// SubgraphRequest specifies what subgraph to extract.
type SubgraphRequest struct {
	Targets   []string      // root targets for the subgraph
	RdepDepth int           // reverse dependency depth (default 2)
	CommitSHA string        // current commit
	Timeout   time.Duration // query timeout
}

// Extract runs bazel query and builds a graph.Snapshot from the results.
// Implements the extract.Extractor interface through a wrapping adapter.
func (e *Extractor) Extract(ctx context.Context, req SubgraphRequest) (*graph.Snapshot, error) {
	start := time.Now()

	if req.RdepDepth <= 0 {
		req.RdepDepth = 2
	}

	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}

	chunks := chunkTargets(req.Targets, maxQueryLabelLength)
	var allRules []xmlRule

	for _, chunk := range chunks {
		query := buildRdepsQuery(chunk, req.RdepDepth)
		rules, err := e.runQuery(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("query chunk failed: %w", err)
		}
		allRules = append(allRules, rules...)
	}

	snap := buildSnapshot(allRules, req.CommitSHA, req.Targets, start)
	return snap, nil
}

// ExtractFull runs a full `bazel query kind(rule, //...)` to extract the complete graph.
// Only internal rule targets are included; external deps (@maven, @pip, etc.) are excluded
// as nodes but their edges are tracked for reference.
func (e *Extractor) ExtractFull(ctx context.Context, commitSHA string, timeout time.Duration) (*graph.Snapshot, error) {
	start := time.Now()

	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Use kind(rule, //...) to get only rule targets (excludes source files,
	// generated files, and package groups). This is significantly faster and
	// smaller than //... on large repos.
	rules, err := e.runQuery(ctx, "kind(rule, //...)")
	if err != nil {
		return nil, fmt.Errorf("full query failed: %w", err)
	}

	snap := buildSnapshot(rules, commitSHA, nil, start)
	snap.Partial = false
	return snap, nil
}

func (e *Extractor) runQuery(ctx context.Context, query string) ([]xmlRule, error) {
	bazel := e.BazelPath
	if bazel == "" {
		bazel = "bazelisk"
	}

	// Startup options (before the command) must come first
	var args []string
	if e.BazelRC != "" {
		args = append(args, "--bazelrc="+e.BazelRC)
	}
	args = append(args, "--nohome_rc") // don't load user's .bazelrc

	// Command
	if e.UseCQuery {
		args = append(args, "cquery")
	} else {
		args = append(args, "query")
	}

	// Command flags
	args = append(args, query, "--output=xml", "--order_output=no", "--keep_going", "--noimplicit_deps")

	cmd := exec.CommandContext(ctx, bazel, args...)
	cmd.Dir = e.WorkspacePath

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// bazel query with --keep_going may exit non-zero but still produce output
		if stdout.Len() == 0 {
			return nil, fmt.Errorf("bazel query failed: %w\nstderr: %s", err, stderr.String())
		}
	}

	return parseXML(stdout.Bytes())
}

func buildRdepsQuery(targets []string, depth int) string {
	if len(targets) == 0 {
		return "//..."
	}

	setExpr := "set(" + strings.Join(targets, " ") + ")"
	return fmt.Sprintf("rdeps(//..., %s, %d)", setExpr, depth)
}

// chunkTargets splits targets into chunks where the total label length
// of each chunk is under maxLen, to avoid hitting Bazel's query length limits.
func chunkTargets(targets []string, maxLen int) [][]string {
	if len(targets) == 0 {
		return [][]string{{}}
	}

	var chunks [][]string
	var current []string
	currentLen := 0

	for _, t := range targets {
		tLen := len(t) + 1 // +1 for space separator
		if currentLen+tLen > maxLen && len(current) > 0 {
			chunks = append(chunks, current)
			current = nil
			currentLen = 0
		}
		current = append(current, t)
		currentLen += tLen
	}
	if len(current) > 0 {
		chunks = append(chunks, current)
	}

	return chunks
}

// XML types for parsing bazel query --output=xml

type xmlQuery struct {
	XMLName xml.Name  `xml:"query"`
	Rules   []xmlRule `xml:"rule"`
}

type xmlRule struct {
	Class string        `xml:"class,attr"`
	Name  string        `xml:"name,attr"`
	Lists []xmlList     `xml:"list"`
	Attrs []xmlAttrStr  `xml:"string"`
}

type xmlList struct {
	Name   string          `xml:"name,attr"`
	Labels []xmlLabelValue `xml:"label"`
	Strs   []xmlStrValue   `xml:"string"`
}

type xmlLabelValue struct {
	Value string `xml:"value,attr"`
}

type xmlStrValue struct {
	Value string `xml:"value,attr"`
}

type xmlAttrStr struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

func parseXML(data []byte) ([]xmlRule, error) {
	// Bazel 8+ outputs XML 1.1, but Go's encoding/xml only supports 1.0.
	// Strip the XML declaration — the actual content is 1.0-compatible.
	data = stripXMLDeclaration(data)

	var q xmlQuery
	if err := xml.Unmarshal(data, &q); err != nil {
		return nil, fmt.Errorf("parsing bazel XML output: %w", err)
	}
	return q.Rules, nil
}

// stripXMLDeclaration removes the <?xml ...?> declaration from the start of
// XML data. This works around Bazel 8+ emitting version="1.1" which Go's
// xml package rejects.
func stripXMLDeclaration(data []byte) []byte {
	// Find <?xml and ?>
	start := bytes.Index(data, []byte("<?xml"))
	if start < 0 {
		return data
	}
	end := bytes.Index(data[start:], []byte("?>"))
	if end < 0 {
		return data
	}
	// Skip past "?>" and any following newline
	cutEnd := start + end + 2
	if cutEnd < len(data) && data[cutEnd] == '\n' {
		cutEnd++
	}
	return append(data[:start], data[cutEnd:]...)
}

// isExternalLabel returns true for labels that reference external repositories
// (e.g., @maven//:guava, @pip//numpy, @com_google_protobuf//:protobuf).
func isExternalLabel(label string) bool {
	// @// is a self-reference (same repo), not external
	if strings.HasPrefix(label, "@//") {
		return false
	}
	return strings.HasPrefix(label, "@")
}

func buildSnapshot(rules []xmlRule, commitSHA string, scope []string, start time.Time) *graph.Snapshot {
	nodes := make(map[string]*graph.Node)
	var edges []graph.Edge
	seen := make(map[string]bool) // deduplicate edges

	for _, rule := range rules {
		label := NormalizeLabel(rule.Name)

		// Skip external targets entirely — they're not part of the codebase's
		// architecture. This dramatically reduces graph size on large monorepos.
		if isExternalLabel(rule.Name) {
			continue
		}

		pkg := labelToPackage(label)

		node := &graph.Node{
			Key:        label,
			Kind:       rule.Class,
			Package:    pkg,
			Tags:       extractTags(rule),
			Visibility: extractVisibility(rule),
			IsTest:     isTestRule(rule.Class),
			IsExternal: false,
		}
		nodes[label] = node

		// Extract dependency edges
		for _, list := range rule.Lists {
			edgeType := classifyDep(list.Name)
			if edgeType == "" {
				continue
			}
			for _, dep := range list.Labels {
				depLabel := NormalizeLabel(dep.Value)

				// Skip edges to external deps — they add noise without
				// architectural signal. We care about internal coupling.
				if isExternalLabel(dep.Value) {
					continue
				}

				eKey := label + "|" + depLabel + "|" + edgeType
				if !seen[eKey] {
					seen[eKey] = true
					edges = append(edges, graph.Edge{
						From: label,
						To:   depLabel,
						Type: edgeType,
					})
				}
			}
		}
	}

	pkgs := make(map[string]bool)
	for _, n := range nodes {
		if n.Package != "" {
			pkgs[n.Package] = true
		}
	}

	snap := &graph.Snapshot{
		ID:        uuid.New().String(),
		CommitSHA: commitSHA,
		Partial:   len(scope) > 0,
		Scope:     scope,
		Nodes:     nodes,
		Edges:     edges,
		Stats: graph.SnapshotStats{
			NodeCount:    len(nodes),
			EdgeCount:    len(edges),
			PackageCount: len(pkgs),
			ExtractionMs: int(time.Since(start).Milliseconds()),
		},
		ExtractedAt: time.Now(),
	}

	return snap
}

// NormalizeLabel normalizes a Bazel label to canonical form.
func NormalizeLabel(label string) string {
	label = strings.TrimSpace(label)

	// Strip @// to //
	if strings.HasPrefix(label, "@//") {
		label = label[1:]
	}

	// Handle //pkg:pkg -> //pkg shorthand
	if idx := strings.LastIndex(label, ":"); idx > 0 {
		pkg := label[:idx]
		target := label[idx+1:]
		pkgBase := filepath.Base(pkg)
		if target == pkgBase {
			return pkg
		}
	}

	return label
}

func labelToPackage(label string) string {
	if idx := strings.LastIndex(label, ":"); idx > 0 {
		return label[:idx]
	}
	return label
}

func extractTags(rule xmlRule) []string {
	for _, list := range rule.Lists {
		if list.Name == "tags" {
			var tags []string
			for _, s := range list.Strs {
				tags = append(tags, s.Value)
			}
			return tags
		}
	}
	return nil
}

func extractVisibility(rule xmlRule) []string {
	for _, list := range rule.Lists {
		if list.Name == "visibility" {
			var vis []string
			for _, l := range list.Labels {
				vis = append(vis, l.Value)
			}
			return vis
		}
	}
	return nil
}

func isTestRule(ruleClass string) bool {
	return strings.HasSuffix(ruleClass, "_test") || strings.HasSuffix(ruleClass, "_tests") || ruleClass == "test_suite"
}

func classifyDep(attrName string) string {
	switch attrName {
	case "deps":
		return extract.EdgeTypeCompile
	case "runtime_deps":
		return extract.EdgeTypeRuntime
	case "data":
		return extract.EdgeTypeData
	default:
		return ""
	}
}

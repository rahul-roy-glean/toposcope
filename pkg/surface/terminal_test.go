package surface_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/toposcope/toposcope/pkg/scoring"
	"github.com/toposcope/toposcope/pkg/surface"
)

func sampleResult() *scoring.ScoreResult {
	return &scoring.ScoreResult{
		TotalScore: 14.0,
		Grade:      "C",
		Breakdown: []scoring.MetricResult{
			{
				Key:          "cross_package_deps",
				Name:         "Cross-package dependencies",
				Contribution: 5.0,
				Severity:     scoring.SeverityMedium,
				Evidence: []scoring.EvidenceItem{
					{Type: scoring.EvidenceEdgeAdded, Summary: "//app/auth:handler -> //lib/session:internal", From: "//app/auth:handler", To: "//lib/session:internal"},
					{Type: scoring.EvidenceEdgeAdded, Summary: "//app/auth:handler -> //lib/session:types", From: "//app/auth:handler", To: "//lib/session:types"},
				},
			},
			{
				Key:          "fanout_increase",
				Name:         "Fanout increase",
				Contribution: 4.0,
				Severity:     scoring.SeverityMedium,
				Evidence: []scoring.EvidenceItem{
					{Type: scoring.EvidenceFanoutChange, Summary: "//app/auth:handler fanout 3 -> 8 (+5)", From: "//app/auth:handler", Value: 8},
				},
			},
			{
				Key:          "cleanup_credits",
				Name:         "Cleanup credits",
				Contribution: -1.0,
				Severity:     scoring.SeverityInfo,
			},
		},
		Hotspots: []scoring.Hotspot{
			{NodeKey: "//app/auth:handler", Reason: "Flagged by 2 metrics", ScoreContribution: 9.0, MetricKeys: []string{"cross_package_deps", "fanout_increase"}},
		},
		SuggestedActions: []scoring.SuggestedAction{
			{Title: "Consider splitting //app/auth:handler", Description: "This target now has 8 dependencies."},
		},
		DeltaStats: scoring.DeltaStatsView{
			AddedNodes:   3,
			RemovedNodes: 0,
			AddedEdges:   10,
			RemovedEdges: 0,
		},
		BaseCommit: "abc123f",
		HeadCommit: "def456a",
	}
}

func TestTerminalRenderer_BasicOutput(t *testing.T) {
	// Set NO_COLOR to avoid ANSI codes in test comparison
	os.Setenv("NO_COLOR", "1")
	defer os.Unsetenv("NO_COLOR")

	r := &surface.TerminalRenderer{}
	var buf bytes.Buffer

	err := r.Render(&buf, sampleResult())
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}

	output := buf.String()

	// Check header
	if !strings.Contains(output, "Grade C") {
		t.Error("expected Grade C in output")
	}
	if !strings.Contains(output, "Score 14.0") {
		t.Error("expected Score 14.0 in output")
	}

	// Check findings
	if !strings.Contains(output, "Cross-package dependencies") {
		t.Error("expected Cross-package dependencies finding")
	}
	if !strings.Contains(output, "Fanout increase") {
		t.Error("expected Fanout increase finding")
	}
	if !strings.Contains(output, "(+5.0)") {
		t.Error("expected (+5.0) contribution")
	}
	if !strings.Contains(output, "(-1.0)") {
		t.Error("expected (-1.0) credit contribution")
	}

	// Check evidence
	if !strings.Contains(output, "//app/auth:handler -> //lib/session:internal") {
		t.Error("expected evidence for cross-package edge")
	}

	// Check hotspots
	if !strings.Contains(output, "Hotspots:") {
		t.Error("expected Hotspots section")
	}
	if !strings.Contains(output, "//app/auth:handler") {
		t.Error("expected hotspot node key")
	}

	// Check suggestions
	if !strings.Contains(output, "Suggested fixes:") {
		t.Error("expected Suggested fixes section")
	}
	if !strings.Contains(output, "Consider splitting") {
		t.Error("expected suggestion text")
	}
}

func TestTerminalRenderer_NoFindings(t *testing.T) {
	os.Setenv("NO_COLOR", "1")
	defer os.Unsetenv("NO_COLOR")

	r := &surface.TerminalRenderer{}
	var buf bytes.Buffer

	result := &scoring.ScoreResult{
		TotalScore: 0,
		Grade:      "A",
		Breakdown:  []scoring.MetricResult{},
		DeltaStats: scoring.DeltaStatsView{},
	}

	err := r.Render(&buf, result)
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "No findings") {
		t.Error("expected 'No findings' message")
	}
}

func TestTerminalRenderer_ColorRespected(t *testing.T) {
	// Without NO_COLOR, output should have ANSI codes
	os.Unsetenv("NO_COLOR")

	r := &surface.TerminalRenderer{}
	var buf bytes.Buffer

	err := r.Render(&buf, sampleResult())
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "\033[") {
		t.Error("expected ANSI escape codes when NO_COLOR is not set")
	}
}

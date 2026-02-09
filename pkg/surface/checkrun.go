package surface

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/toposcope/toposcope/pkg/scoring"
)

// CheckRunRenderer produces GitHub Check Run data from a ScoreResult.
type CheckRunRenderer struct{}

func (r *CheckRunRenderer) Render(w io.Writer, result *scoring.ScoreResult) error {
	data := r.BuildCheckRunData(result)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

// BuildCheckRunData creates the CheckRunData struct from a ScoreResult.
func (r *CheckRunRenderer) BuildCheckRunData(result *scoring.ScoreResult) CheckRunData {
	conclusion := gradeToConclusion(result.Grade)
	title := fmt.Sprintf("Toposcope: Grade %s — Score %.1f", result.Grade, result.TotalScore)
	summary := buildMarkdownSummary(result)

	return CheckRunData{
		Title:      title,
		Summary:    summary,
		Conclusion: conclusion,
	}
}

func gradeToConclusion(grade string) string {
	switch grade {
	case "A", "B":
		return "success"
	case "C":
		return "neutral"
	default:
		return "failure"
	}
}

func buildMarkdownSummary(result *scoring.ScoreResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## Toposcope: Grade %s — Score %.1f\n\n", result.Grade, result.TotalScore))

	// Delta stats
	sb.WriteString("### Delta Stats\n\n")
	sb.WriteString(fmt.Sprintf("| Metric | Count |\n|--------|-------|\n"))
	sb.WriteString(fmt.Sprintf("| Added Nodes | %d |\n", result.DeltaStats.AddedNodes))
	sb.WriteString(fmt.Sprintf("| Removed Nodes | %d |\n", result.DeltaStats.RemovedNodes))
	sb.WriteString(fmt.Sprintf("| Added Edges | %d |\n", result.DeltaStats.AddedEdges))
	sb.WriteString(fmt.Sprintf("| Removed Edges | %d |\n", result.DeltaStats.RemovedEdges))
	sb.WriteString("\n")

	// Findings (max 5)
	sb.WriteString("### Findings\n\n")
	count := 0
	for _, mr := range result.Breakdown {
		if mr.Contribution == 0 && len(mr.Evidence) == 0 {
			continue
		}
		if count >= 5 {
			sb.WriteString(fmt.Sprintf("_... and %d more findings_\n", len(result.Breakdown)-5))
			break
		}
		sign := "+"
		if mr.Contribution < 0 {
			sign = ""
		}
		icon := severityIcon(mr.Severity)
		sb.WriteString(fmt.Sprintf("- %s **%s** (%s%.1f) — %s\n",
			icon, mr.Name, sign, mr.Contribution, severityLabel(mr.Severity)))

		// Show top 3 evidence items
		maxEv := 3
		if len(mr.Evidence) < maxEv {
			maxEv = len(mr.Evidence)
		}
		for i := 0; i < maxEv; i++ {
			sb.WriteString(fmt.Sprintf("  - %s\n", mr.Evidence[i].Summary))
		}
		count++
	}
	sb.WriteString("\n")

	// Suggestions (max 3)
	if len(result.SuggestedActions) > 0 {
		sb.WriteString("### Suggestions\n\n")
		max := 3
		if len(result.SuggestedActions) < max {
			max = len(result.SuggestedActions)
		}
		for i := 0; i < max; i++ {
			sa := result.SuggestedActions[i]
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", sa.Title, sa.Description))
		}
	}

	return sb.String()
}

func severityIcon(sev scoring.Severity) string {
	switch sev {
	case scoring.SeverityHigh:
		return ":red_circle:"
	case scoring.SeverityMedium:
		return ":orange_circle:"
	case scoring.SeverityLow:
		return ":yellow_circle:"
	default:
		return ":blue_circle:"
	}
}

func severityLabel(sev scoring.Severity) string {
	switch sev {
	case scoring.SeverityHigh:
		return "HIGH"
	case scoring.SeverityMedium:
		return "MEDIUM"
	case scoring.SeverityLow:
		return "LOW"
	default:
		return "INFO"
	}
}

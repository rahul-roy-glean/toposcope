package surface

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/toposcope/toposcope/pkg/scoring"
)

// TerminalRenderer renders ScoreResult as colored terminal output.
type TerminalRenderer struct{}

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
)

func gradeColor(grade string) string {
	if noColor() {
		return ""
	}
	switch grade {
	case "A", "B":
		return colorGreen
	case "C":
		return colorYellow
	case "D", "F":
		return colorRed
	default:
		return ""
	}
}

func noColor() bool {
	_, ok := os.LookupEnv("NO_COLOR")
	return ok
}

func bold(s string) string {
	if noColor() {
		return s
	}
	return colorBold + s + colorReset
}

func dim(s string) string {
	if noColor() {
		return s
	}
	return colorDim + s + colorReset
}

func colored(s, color string) string {
	if noColor() || color == "" {
		return s
	}
	return color + s + colorReset
}

func (r *TerminalRenderer) Render(w io.Writer, result *scoring.ScoreResult) error {
	gc := gradeColor(result.Grade)

	// Header
	fmt.Fprintf(w, "%s\n\n",
		bold(fmt.Sprintf("Toposcope: Grade %s — Score %.1f",
			colored(result.Grade, gc), result.TotalScore)))

	// Stats
	fmt.Fprintf(w, "Analyzed: %d added nodes / %d removed nodes / %d added edges / %d removed edges\n\n",
		result.DeltaStats.AddedNodes, result.DeltaStats.RemovedNodes,
		result.DeltaStats.AddedEdges, result.DeltaStats.RemovedEdges)

	// Findings
	hasFindings := false
	for _, mr := range result.Breakdown {
		if mr.Contribution == 0 && len(mr.Evidence) == 0 {
			continue
		}
		if !hasFindings {
			fmt.Fprintln(w, "Findings:")
			hasFindings = true
		}

		sign := "+"
		if mr.Contribution < 0 {
			sign = ""
		}
		fmt.Fprintf(w, "  (%s%.1f) %s", sign, mr.Contribution, bold(mr.Name))

		if len(mr.Evidence) > 0 {
			fmt.Fprintf(w, " — %s", mr.Evidence[0].Summary)
		}
		fmt.Fprintln(w)

		// Show additional evidence (up to 5 total)
		maxEvidence := 5
		if len(mr.Evidence) < maxEvidence {
			maxEvidence = len(mr.Evidence)
		}
		for i := 1; i < maxEvidence; i++ {
			fmt.Fprintf(w, "         %s\n", dim(mr.Evidence[i].Summary))
		}
		if len(mr.Evidence) > 5 {
			fmt.Fprintf(w, "         %s\n", dim(fmt.Sprintf("... and %d more", len(mr.Evidence)-5)))
		}
		fmt.Fprintln(w)
	}

	if !hasFindings {
		fmt.Fprintln(w, "No findings.")
		fmt.Fprintln(w)
	}

	// Hotspots
	if len(result.Hotspots) > 0 {
		fmt.Fprintln(w, "Hotspots:")
		for _, hs := range result.Hotspots {
			fmt.Fprintf(w, "  %s %s — %s\n",
				colored("●", colorRed), bold(hs.NodeKey), hs.Reason)
		}
		fmt.Fprintln(w)
	}

	// Suggestions
	if len(result.SuggestedActions) > 0 {
		fmt.Fprintln(w, "Suggested fixes:")
		for _, sa := range result.SuggestedActions {
			fmt.Fprintf(w, "  • %s\n", sa.Title)
			if sa.Description != "" {
				// Wrap description with indent
				lines := wrapText(sa.Description, 70)
				for _, line := range lines {
					fmt.Fprintf(w, "    %s\n", dim(line))
				}
			}
		}
		fmt.Fprintln(w)
	}

	return nil
}

// wrapText wraps a string at the given width, returning lines.
func wrapText(s string, width int) []string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return nil
	}

	var lines []string
	current := words[0]

	for _, word := range words[1:] {
		if len(current)+1+len(word) > width {
			lines = append(lines, current)
			current = word
		} else {
			current += " " + word
		}
	}
	lines = append(lines, current)
	return lines
}

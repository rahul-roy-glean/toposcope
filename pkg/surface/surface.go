// Package surface defines output rendering interfaces for Toposcope results.
// Implementations handle different output targets: terminal, GitHub Check Run, JSON.
package surface

import (
	"io"

	"github.com/toposcope/toposcope/pkg/scoring"
)

// Renderer produces formatted output from a ScoreResult.
type Renderer interface {
	// Render writes the formatted score result to the writer.
	Render(w io.Writer, result *scoring.ScoreResult) error
}

// CheckRunData holds the data needed to create a GitHub Check Run.
type CheckRunData struct {
	Title      string `json:"title"`
	Summary    string `json:"summary"`    // Markdown body
	Conclusion string `json:"conclusion"` // success, neutral, failure
}

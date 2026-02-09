package surface

import (
	"encoding/json"
	"io"

	"github.com/toposcope/toposcope/pkg/scoring"
)

// JSONRenderer marshals ScoreResult to indented JSON.
type JSONRenderer struct{}

func (r *JSONRenderer) Render(w io.Writer, result *scoring.ScoreResult) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

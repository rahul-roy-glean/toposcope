// Package scoring implements the Toposcope structural health scoring engine.
// It evaluates build graph deltas and produces explainable, evidence-backed scores.
package scoring

// ScoreResult is the complete output of scoring a structural change.
// Immutable once computed.
type ScoreResult struct {
	TotalScore       float64           `json:"total_score"`
	Grade            string            `json:"grade"` // A, B, C, D, F
	Breakdown        []MetricResult    `json:"breakdown"`
	Hotspots         []Hotspot         `json:"hotspots"`
	SuggestedActions []SuggestedAction `json:"suggested_actions"`
	DeltaStats       DeltaStatsView    `json:"delta_stats"`
	BaseCommit       string            `json:"base_commit"`
	HeadCommit       string            `json:"head_commit"`
}

// DeltaStatsView is a read-only summary of the delta for display purposes.
type DeltaStatsView struct {
	ImpactedTargets int `json:"impacted_targets"`
	AddedNodes      int `json:"added_nodes"`
	RemovedNodes    int `json:"removed_nodes"`
	AddedEdges      int `json:"added_edges"`
	RemovedEdges    int `json:"removed_edges"`
}

// MetricResult is the output of a single scoring metric.
type MetricResult struct {
	Key          string         `json:"key"`          // machine key: "cross_package_deps"
	Name         string         `json:"name"`         // human name: "Cross-package dependencies"
	Contribution float64        `json:"contribution"` // score contribution (positive = worse, negative = credit)
	Severity     Severity       `json:"severity"`
	Evidence     []EvidenceItem `json:"evidence"`
}

// Severity indicates how concerning a metric finding is.
type Severity string

const (
	SeverityHigh   Severity = "HIGH"
	SeverityMedium Severity = "MEDIUM"
	SeverityLow    Severity = "LOW"
	SeverityInfo   Severity = "INFO"
)

// EvidenceItem is a single piece of concrete evidence backing a score contribution.
type EvidenceItem struct {
	Type    EvidenceType `json:"type"`
	Summary string       `json:"summary"`         // human-readable explanation
	From    string       `json:"from,omitempty"`  // source node key
	To      string       `json:"to,omitempty"`    // target node key
	Value   float64      `json:"value,omitempty"` // numeric value (degree, count, etc.)
}

// EvidenceType classifies what kind of evidence this is.
type EvidenceType string

const (
	EvidenceEdgeAdded    EvidenceType = "EDGE_ADDED"
	EvidenceEdgeRemoved  EvidenceType = "EDGE_REMOVED"
	EvidenceFanoutChange EvidenceType = "FANOUT_CHANGE"
	EvidenceCentrality   EvidenceType = "CENTRALITY"
	EvidenceBlastRadius  EvidenceType = "BLAST_RADIUS"
)

// Hotspot identifies a node that appears across multiple metric findings.
type Hotspot struct {
	NodeKey           string   `json:"node_key"`
	Reason            string   `json:"reason"`
	ScoreContribution float64  `json:"score_contribution"`
	MetricKeys        []string `json:"metric_keys"` // which metrics flagged this node
}

// SuggestedAction is a human- and machine-readable recommendation.
type SuggestedAction struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Targets     []string `json:"targets"`    // affected node keys
	Confidence  float64  `json:"confidence"` // 0.0-1.0
	Addresses   []string `json:"addresses"`  // metric keys this addresses
}

// GradeFromScore maps a total score to a letter grade.
func GradeFromScore(score float64) string {
	switch {
	case score <= 3:
		return "A"
	case score <= 7:
		return "B"
	case score <= 14:
		return "C"
	case score <= 24:
		return "D"
	default:
		return "F"
	}
}

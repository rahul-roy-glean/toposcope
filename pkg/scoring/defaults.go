package scoring

// DefaultMetrics returns the standard set of scoring metrics with default weights.
func DefaultMetrics() []Metric {
	w := Defaults()
	return []Metric{
		&CrossPackageMetric{
			IntraBoundaryWeight: w.CrossPackageIntraBoundary,
			CrossBoundaryWeight: w.CrossPackageCrossBoundary,
		},
		&FanoutMetric{
			Weight:       w.FanoutWeight,
			CapPerNode:   w.FanoutCapPerNode,
			MinThreshold: w.FanoutMinThreshold,
		},
		&CentralityMetric{
			Weight:          w.CentralityWeight,
			MinInDegree:     w.CentralityMinInDegree,
			MaxContribution: w.CentralityMaxContribution,
		},
		&BlastRadiusMetric{
			Weight:          w.BlastRadiusWeight,
			MaxContribution: w.BlastRadiusMaxContribution,
		},
		&CreditsMetric{
			PerRemovedCrossBoundaryEdge: w.CreditPerRemovedCrossBoundaryEdge,
			MaxCreditTotal:              w.CreditMaxTotal,
			PerFanoutReduction:          w.CreditPerFanoutReduction,
			FanoutMaxCredit:             w.CreditFanoutMaxTotal,
		},
	}
}

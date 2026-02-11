package scoring

// DefaultWeights holds the default scoring weights for all metrics.
type DefaultWeights struct {
	// M1: Cross-package dependencies
	CrossPackageIntraBoundary float64
	CrossPackageCrossBoundary float64

	// M2: Fanout increase
	FanoutWeight       float64
	FanoutCapPerNode   float64
	FanoutMinThreshold int // only score if out-degree exceeds this after change

	// M3: Centrality penalty
	CentralityWeight          float64
	CentralityMinInDegree     int     // only apply for targets above this in-degree
	CentralityMaxContribution float64 // safety cap on centrality contribution

	// M5: Blast radius
	BlastRadiusWeight          float64
	BlastRadiusMaxContribution float64

	// M6: Credits
	CreditPerRemovedCrossBoundaryEdge float64
	CreditMaxTotal                    float64
	CreditPerFanoutReduction          float64
	CreditFanoutMaxTotal              float64
}

// Defaults returns the default scoring weights.
func Defaults() DefaultWeights {
	return DefaultWeights{
		// M1
		CrossPackageIntraBoundary: 0.5,
		CrossPackageCrossBoundary: 1.5,

		// M2
		FanoutWeight:       0.5,
		FanoutCapPerNode:   10,
		FanoutMinThreshold: 10,

		// M3
		CentralityWeight:          0.7,
		CentralityMinInDegree:     50,
		CentralityMaxContribution: 40.0,

		// M5
		BlastRadiusWeight:          2.0,
		BlastRadiusMaxContribution: 15.0,

		// M6
		CreditPerRemovedCrossBoundaryEdge: -0.5,
		CreditMaxTotal:                    -15.0,
		CreditPerFanoutReduction:          -0.3,
		CreditFanoutMaxTotal:              -10.0,
	}
}

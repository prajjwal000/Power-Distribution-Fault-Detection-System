package generator

type Range struct {
	Min int
	Max int
}

type Config struct {
	SubstationCount   int
	FeedersPerSub     int
	DTsPerFeeder      Range
	PolesPerDT        Range
	BranchesPerLine   Range
	BranchAngleRange  Range
	ExtremeBranchPct  float64
	ExtremeAngleRange Range
	PoleSpacingMeters Range
	BearingDriftDeg   float64

	MissingTopologyPct float64
	NoDevicePct        float64
	MissingPincodePct  float64
}

func DefaultConfig() Config {
	return Config{
		SubstationCount:   2,
		FeedersPerSub:     3,
		DTsPerFeeder:      Range{Min: 6, Max: 8},
		PolesPerDT:        Range{Min: 9, Max: 240},
		BranchesPerLine:   Range{Min: 1, Max: 5},
		BranchAngleRange:  Range{Min: 45, Max: 90},
		ExtremeBranchPct:  0.05,
		ExtremeAngleRange: Range{Min: 80, Max: 120},
		PoleSpacingMeters: Range{Min: 30, Max: 50},
		BearingDriftDeg:   5.0,

		MissingTopologyPct: 0.60,
		NoDevicePct:        0.09,
		MissingPincodePct:  0.03,
	}
}

func ConfigForPoleCount(targetPoles int) Config {
	cfg := DefaultConfig()

	switch {
	case targetPoles < 500:
		cfg.SubstationCount = 1
		cfg.FeedersPerSub = 2
		cfg.DTsPerFeeder = Range{Min: 3, Max: 5}
	case targetPoles < 2000:
		cfg.SubstationCount = 1
		cfg.FeedersPerSub = 3
		cfg.DTsPerFeeder = Range{Min: 5, Max: 7}
	case targetPoles < 5000:
		cfg.SubstationCount = 2
		cfg.FeedersPerSub = 3
		cfg.DTsPerFeeder = Range{Min: 6, Max: 8}
	case targetPoles < 15000:
		cfg.SubstationCount = 3
		cfg.FeedersPerSub = 6
		cfg.DTsPerFeeder = Range{Min: 8, Max: 12}
	default:
		// Full-scale: 4 subs × ~8 feeders = 31 feeders, 412 DTs, ~38k poles
		cfg.SubstationCount = 4
		cfg.FeedersPerSub = 8
		cfg.DTsPerFeeder = Range{Min: 10, Max: 16}
	}

	// PolesPerDT always uses the assignment's full range (9–240).
	// The piecewise samplePoleCount distribution gives the right shape
	// regardless of the range bounds.

	return cfg
}

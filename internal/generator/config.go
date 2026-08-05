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

	FeederLengthMeters  Range
	DTSpacingMeters     Range
	MainLinePct         float64
	BranchNearDTPct     float64
	BranchNearDTRange   float64
	MaxBranchLength     int
	MinBranchLength     int
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

		FeederLengthMeters: Range{Min: 3000, Max: 8000},
		DTSpacingMeters:    Range{Min: 200, Max: 400},
		MainLinePct:        0.75,
		BranchNearDTPct:    0.80,
		BranchNearDTRange:  0.20,
		MaxBranchLength:    15,
		MinBranchLength:    3,
	}
}

func ConfigForPoleCount(targetPoles int) Config {
	cfg := DefaultConfig()

	switch {
	case targetPoles < 500:
		cfg.SubstationCount = 1
		cfg.FeedersPerSub = 2
		cfg.DTsPerFeeder = Range{Min: 3, Max: 5}
		cfg.PolesPerDT = Range{Min: 20, Max: 80}
		cfg.FeederLengthMeters = Range{Min: 2000, Max: 5000}
		cfg.DTSpacingMeters = Range{Min: 250, Max: 450}
	case targetPoles < 2000:
		cfg.SubstationCount = 1
		cfg.FeedersPerSub = 3
		cfg.DTsPerFeeder = Range{Min: 5, Max: 7}
		cfg.PolesPerDT = Range{Min: 30, Max: 120}
		cfg.FeederLengthMeters = Range{Min: 3000, Max: 6000}
		cfg.DTSpacingMeters = Range{Min: 200, Max: 400}
	case targetPoles < 5000:
		cfg.SubstationCount = 2
		cfg.FeedersPerSub = 3
		cfg.DTsPerFeeder = Range{Min: 6, Max: 8}
		cfg.PolesPerDT = Range{Min: 40, Max: 150}
		cfg.FeederLengthMeters = Range{Min: 3000, Max: 8000}
		cfg.DTSpacingMeters = Range{Min: 200, Max: 400}
	default:
		cfg.SubstationCount = 2
		cfg.FeedersPerSub = 4
		cfg.DTsPerFeeder = Range{Min: 8, Max: 12}
		cfg.PolesPerDT = Range{Min: 50, Max: 200}
		cfg.FeederLengthMeters = Range{Min: 4000, Max: 10000}
		cfg.DTSpacingMeters = Range{Min: 150, Max: 350}
	}

	return cfg
}

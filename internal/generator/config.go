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
		cfg.PolesPerDT = Range{Min: 20, Max: 80}
	case targetPoles < 2000:
		cfg.SubstationCount = 1
		cfg.FeedersPerSub = 3
		cfg.DTsPerFeeder = Range{Min: 5, Max: 7}
		cfg.PolesPerDT = Range{Min: 30, Max: 120}
	case targetPoles < 5000:
		cfg.SubstationCount = 2
		cfg.FeedersPerSub = 3
		cfg.DTsPerFeeder = Range{Min: 6, Max: 8}
		cfg.PolesPerDT = Range{Min: 40, Max: 150}
	default:
		cfg.SubstationCount = 2
		cfg.FeedersPerSub = 4
		cfg.DTsPerFeeder = Range{Min: 8, Max: 12}
		cfg.PolesPerDT = Range{Min: 50, Max: 200}
	}

	return cfg
}

package generator

import (
	"fmt"
	"math"
	"math/rand"
	"power-fault-detector/internal/model"
)

type GeneratedNetwork struct {
	Substations  []model.Substation
	Feeders      []model.Feeder
	Transformers []model.Transformer
	GTPoles      []model.GTPole
}

func Generate(cfg Config) *GeneratedNetwork {
	net := &GeneratedNetwork{}

	poleCounter := 0

	for subIdx := 0; subIdx < cfg.SubstationCount; subIdx++ {
		sub := model.Substation{
			ID:  fmt.Sprintf("S-%02d", subIdx+1),
			Lat: PlaceSubstation(subIdx).Lat,
			Lon: PlaceSubstation(subIdx).Lon,
		}
		net.Substations = append(net.Substations, sub)

		for feederIdx := 0; feederIdx < cfg.FeedersPerSub; feederIdx++ {
			feeder := model.Feeder{
				ID:           fmt.Sprintf("F-%02d-%02d", subIdx+1, feederIdx+1),
				SubstationID: sub.ID,
				Name:         fmt.Sprintf("Feeder %d-%d", subIdx+1, feederIdx+1),
				Lat:          PlaceSubstation(subIdx).Lat,
				Lon:          PlaceSubstation(subIdx).Lon,
			}

			dtCount := rand.Intn(cfg.DTsPerFeeder.Max-cfg.DTsPerFeeder.Min+1) + cfg.DTsPerFeeder.Min

			var feederDTs []model.Transformer
			for dtIdx := 0; dtIdx < dtCount; dtIdx++ {
				dtCoord := PlaceTransformer(dtIdx, feederIdx)
				dt := model.Transformer{
					ID:               fmt.Sprintf("D-%04d", len(net.Transformers)+len(feederDTs)+1),
					FeederID:         feeder.ID,
					Lat:              dtCoord.Lat,
					Lon:              dtCoord.Lon,
					CapacityKVA:      rand.Intn(400) + 100,
					HouseholdsServed: rand.Intn(400) + 100,
				}
				feederDTs = append(feederDTs, dt)
			}

			feeder.Lat, feeder.Lon = computeFeederCentroid(feederDTs)
			net.Feeders = append(net.Feeders, feeder)

			for _, dt := range feederDTs {
				net.Transformers = append(net.Transformers, dt)
				poles := generateTree(dt, cfg, &poleCounter)
				net.GTPoles = append(net.GTPoles, poles...)
			}
		}
	}

	return net
}

func computeFeederCentroid(dts []model.Transformer) (float64, float64) {
	if len(dts) == 0 {
		return 0, 0
	}
	var sumLat, sumLon float64
	for _, dt := range dts {
		sumLat += dt.Lat
		sumLon += dt.Lon
	}
	return sumLat / float64(len(dts)), sumLon / float64(len(dts))
}

func generateTree(dt model.Transformer, cfg Config, poleCounter *int) []model.GTPole {
	var poles []model.GTPole

	totalPoles := samplePoleCount(cfg.PolesPerDT)
	if totalPoles < 3 {
		return buildStraightLine(dt, totalPoles, cfg, poleCounter)
	}

	// Main line cap: LT lines run up to ~1.4 km from the transformer.
	avgSpacing := float64(cfg.PoleSpacingMeters.Min+cfg.PoleSpacingMeters.Max) / 2.0
	mainLineCap := int(1400.0 / avgSpacing) // 35 at 40m avg

	// Desired branch count: 1 to 5 (assignment heuristic).
	desiredBranches := rand.Intn(cfg.BranchesPerLine.Max-cfg.BranchesPerLine.Min+1) + cfg.BranchesPerLine.Min

	// Determine actual branchCount and mainLineLength.
	// Invariant: mainLineLength + Σ(branchSizes) = totalPoles
	// Constraint: mainLineLength <= mainLineCap (35)
	// Constraint: branchCount in [0, 5] (assignment heuristic)
	// Constraint: each branch >= 3 poles
	// Constraint: branchCount <= mainLineLength - 3 (room for branch points)
	branchCount := desiredBranches
	if branchCount > 5 {
		branchCount = 5
	}
	if branchCount > totalPoles/3 {
		branchCount = totalPoles / 3
	}

	mainLineLength := totalPoles - branchCount*3
	if mainLineLength > mainLineCap {
		// Main line capped at distance limit. Excess stays as branch poles
		// (branches get longer, not more numerous).
		mainLineLength = mainLineCap
	}
	// Ensure room for branch points (need indices 3..mainLineLength-1)
	for branchCount > 0 && branchCount > mainLineLength-3 {
		branchCount--
	}
	// Recompute mainLineLength if branchCount was reduced
	if branchCount == 0 {
		mainLineLength = totalPoles
	} else {
		mainLineLength = totalPoles - branchCount*3
		if mainLineLength > mainLineCap {
			mainLineLength = mainLineCap
		}
	}
	if mainLineLength < 3 {
		mainLineLength = 3
	}

	// Distribute remaining poles across branches (each ≥ 3).
	branchSizes := make([]int, branchCount)
	if branchCount > 0 {
		remaining := totalPoles - mainLineLength
		base := remaining / branchCount
		extra := remaining % branchCount
		for i := 0; i < branchCount; i++ {
			branchSizes[i] = base
			if i < extra {
				branchSizes[i]++
			}
		}
	}

	// Build the main line
	mainBearing := RandomBearing()
	spacing := rand.Intn(cfg.PoleSpacingMeters.Max-cfg.PoleSpacingMeters.Min+1) + cfg.PoleSpacingMeters.Min
	firstCoord := PlacePoleAt(Coord{Lat: dt.Lat, Lon: dt.Lon}, mainBearing, spacing)
	firstPole := createPole(dt.ID, firstCoord.Lat, firstCoord.Lon, 1, nil, poleCounter)
	poles = append(poles, firstPole)
	prev := &poles[len(poles)-1]
	seq := 2

	// Pick branch point indices along the main line.
	branchPointIdxs := pickBranchPoints(mainLineLength, branchCount)

	for i := 2; i <= mainLineLength; i++ {
		mainBearing = DriftBearing(mainBearing, cfg.BearingDriftDeg)
		spacing := rand.Intn(cfg.PoleSpacingMeters.Max-cfg.PoleSpacingMeters.Min+1) + cfg.PoleSpacingMeters.Min
		coord := PlacePoleAt(Coord{Lat: prev.Lat, Lon: prev.Lon}, mainBearing, spacing)

		parentID := prev.ID
		pole := createPole(dt.ID, coord.Lat, coord.Lon, seq, &parentID, poleCounter)
		poles = append(poles, pole)
		prev = &poles[len(poles)-1]
		seq++

		bi, isBranch := branchPointIdxs[i]
		if isBranch {
			branchAngle := chooseBranchAngle(cfg, mainBearing)
			branchPoles := generateBranch(pole, branchAngle, branchSizes[bi], cfg, poleCounter, dt.ID, seq)
			poles = append(poles, branchPoles...)
			seq += len(branchPoles)
			poles[findPoleIndex(poles, pole.ID)].IsBranchPoint = true
		}
	}

	return poles
}

// buildStraightLine creates a simple straight line of poles (no branches).
// Used for degenerate cases where totalPoles < 3.
func buildStraightLine(dt model.Transformer, count int, cfg Config, poleCounter *int) []model.GTPole {
	var poles []model.GTPole
	bearing := RandomBearing()
	cursor := Coord{Lat: dt.Lat, Lon: dt.Lon}
	for i := 0; i < count; i++ {
		spacing := rand.Intn(cfg.PoleSpacingMeters.Max-cfg.PoleSpacingMeters.Min+1) + cfg.PoleSpacingMeters.Min
		cursor = PlacePoleAt(cursor, bearing, spacing)
		var parent *string
		if i > 0 {
			p := poles[i-1].ID
			parent = &p
		}
		poles = append(poles, createPole(dt.ID, cursor.Lat, cursor.Lon, i+1, parent, poleCounter))
	}
	return poles
}

func generateBranch(startPole model.GTPole, branchBearing float64, count int, cfg Config, poleCounter *int, dtID string, startSeq int) []model.GTPole {
	var poles []model.GTPole

	current := startPole
	for i := 0; i < count; i++ {
		branchBearing = DriftBearing(branchBearing, cfg.BearingDriftDeg)
		spacing := rand.Intn(cfg.PoleSpacingMeters.Max-cfg.PoleSpacingMeters.Min+1) + cfg.PoleSpacingMeters.Min
		coord := PlacePoleAt(Coord{Lat: current.Lat, Lon: current.Lon}, branchBearing, spacing)

		parentID := current.ID
		pole := createPole(dtID, coord.Lat, coord.Lon, startSeq+i+1, &parentID, poleCounter)
		poles = append(poles, pole)
		current = poles[len(poles)-1]
	}

	return poles
}

func chooseBranchAngle(cfg Config, mainBearing float64) float64 {
	var angleRange Range
	if rand.Float64() < cfg.ExtremeBranchPct {
		angleRange = cfg.ExtremeAngleRange
	} else {
		angleRange = cfg.BranchAngleRange
	}

	angle := rand.Intn(angleRange.Max-angleRange.Min+1) + angleRange.Min
	if rand.Float64() < 0.5 {
		angle = -angle
	}

	return NormalizeBearing(mainBearing + float64(angle))
}

func createPole(dtID string, lat, lon float64, seq int, parentID *string, poleCounter *int) model.GTPole {
	*poleCounter++
	return model.GTPole{
		ID:            fmt.Sprintf("P-%06d", *poleCounter),
		DTID:          dtID,
		ParentPoleID:  parentID,
		SeqOnLine:     seq,
		IsBranchPoint: false,
		Lat:           lat,
		Lon:           lon,
	}
}

// samplePoleCount produces a right-skewed count matching the assignment
// heuristic: poles per DT range [Min, Max] with median ~70.
//
// Distribution: log-normal, mu=4.1, sigma=1.0.
//   - X = exp(mu + sigma * Z), Z ~ N(0,1)
//   - Unclamped median ≈ 57
//   - After clamping at [9, 240], effective median ≈ 70-80
//   - ~23% of values < 30 (good lower tail)
//   - ~1% of values < 10 (rare floor hits)
//   - ~10% of values > 180 (long upper tail)
//
// Values outside [Min, Max] are clamped. The distribution is independent of
// Min/Max — only the clamping bounds change.
func samplePoleCount(r Range) int {
	if r.Max <= r.Min {
		return r.Min
	}
	const mu = 4.1
	const sigma = 1.0
	minF, maxF := float64(r.Min), float64(r.Max)
	val := math.Exp(mu + sigma*rand.NormFloat64())
	if val < minF {
		val = minF
	}
	if val > maxF {
		val = maxF
	}
	return int(val + 0.5)
}

// pickBranchPoints returns a map of main-line index → branch index for each
// branch attachment point. The first 2 poles (near the transformer) and the
// last pole (end of the main run) are excluded so branches don't cluster at
// the transformer or at the dead end.
func pickBranchPoints(mainLineLength, branchCount int) map[int]int {
	points := make(map[int]int)
	if branchCount == 0 || mainLineLength < 5 {
		return points
	}
	minPt := 3
	maxPt := mainLineLength - 1
	available := maxPt - minPt + 1
	if available < branchCount {
		branchCount = available
	}
	// Pick unique points and assign branch indices 0..branchCount-1
	used := make(map[int]bool)
	bi := 0
	for bi < branchCount {
		pt := minPt + rand.Intn(available)
		if used[pt] {
			continue
		}
		used[pt] = true
		points[pt] = bi
		bi++
	}
	return points
}

func findPoleIndex(poles []model.GTPole, id string) int {
	for i, p := range poles {
		if p.ID == id {
			return i
		}
	}
	return -1
}

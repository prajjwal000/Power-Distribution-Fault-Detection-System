package generator

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"power-fault-detector/internal/model"
	"sort"
)

type GeneratedNetwork struct {
	Substations  []model.Substation
	Feeders      []model.Feeder
	Transformers []model.Transformer
	GTPoles      []model.GTPole
}

func findDataDir() string {
	// Start from the current working directory and walk up to find go.mod
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "data")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	// Fallback to relative path
	return "data"
}

func Generate(cfg Config) *GeneratedNetwork {
	if os.Getenv("GENERATOR_NO_ROADS") == "1" {
		return GenerateWithoutRoads(cfg)
	}
	dataDir := findDataDir()
	grid, err := LoadSpatialIndex(dataDir)
	if err != nil {
		return GenerateWithoutRoads(cfg)
	}
	return GenerateWithRoads(cfg, grid)
}

func GenerateWithoutRoads(cfg Config) *GeneratedNetwork {
	net := &GeneratedNetwork{}
	poleCounter := 0

	for subIdx := 0; subIdx < cfg.SubstationCount; subIdx++ {
		subCoord := fallbackSubstations(cfg.SubstationCount)[subIdx]
		sub := model.Substation{
			ID:  fmt.Sprintf("S-%02d", subIdx+1),
			Lat: subCoord.Lat,
			Lon: subCoord.Lon,
		}
		net.Substations = append(net.Substations, sub)

		for feederIdx := 0; feederIdx < cfg.FeedersPerSub; feederIdx++ {
			bearing := (360.0 / float64(cfg.FeedersPerSub)) * float64(feederIdx)
			route := straightRoute(subCoord, bearing, 5000)

			feederID := fmt.Sprintf("F-%02d-%02d", subIdx+1, feederIdx+1)
			feeder := model.Feeder{
				ID:           feederID,
				SubstationID: sub.ID,
				Name:         fmt.Sprintf("Feeder %d-%d", subIdx+1, feederIdx+1),
				Lat:          route[len(route)/2].Lat,
				Lon:          route[len(route)/2].Lon,
			}
			net.Feeders = append(net.Feeders, feeder)

			dtCount := rand.Intn(cfg.DTsPerFeeder.Max-cfg.DTsPerFeeder.Min+1) + cfg.DTsPerFeeder.Min
			dtCoords := PlaceDTsAlongFeeder(route, dtCount, nil)

			for i, dtCoord := range dtCoords {
				dt := model.Transformer{
					ID:               fmt.Sprintf("D-%04d", len(net.Transformers)+1),
					FeederID:         feederID,
					Lat:              dtCoord.Lat,
					Lon:              dtCoord.Lon,
					CapacityKVA:      rand.Intn(400) + 100,
					HouseholdsServed: rand.Intn(400) + 100,
				}
				net.Transformers = append(net.Transformers, dt)
				_ = i

				poles := generateFallbackTree(dt, cfg, &poleCounter)
				net.GTPoles = append(net.GTPoles, poles...)
			}
		}
	}

	return net
}

func GenerateWithRoads(cfg Config, grid *SpatialGrid) *GeneratedNetwork {
	net := &GeneratedNetwork{}
	poleCounter := 0

	subCoords := PlaceSubstations(cfg.SubstationCount, grid)

	for subIdx, subCoord := range subCoords {
		sub := model.Substation{
			ID:  fmt.Sprintf("S-%02d", subIdx+1),
			Lat: subCoord.Lat,
			Lon: subCoord.Lon,
		}
		net.Substations = append(net.Substations, sub)

		feederBearingStep := 360.0 / float64(cfg.FeedersPerSub)
		for feederIdx := 0; feederIdx < cfg.FeedersPerSub; feederIdx++ {
			targetBearing := feederBearingStep*float64(feederIdx) + (rand.Float64()*30 - 15)
			feederLen := rand.Intn(cfg.FeederLengthMeters.Max-cfg.FeederLengthMeters.Min+1) + cfg.FeederLengthMeters.Min

			route := PlaceFeederRoute(subCoord, targetBearing, feederLen, grid)
			if len(route) < 2 {
				route = straightRoute(subCoord, targetBearing, feederLen)
			}

			feederID := fmt.Sprintf("F-%02d-%02d", subIdx+1, feederIdx+1)
			feeder := model.Feeder{
				ID:           feederID,
				SubstationID: sub.ID,
				Name:         fmt.Sprintf("Feeder %d-%d", subIdx+1, feederIdx+1),
				Lat:          route[len(route)/2].Lat,
				Lon:          route[len(route)/2].Lon,
			}
			net.Feeders = append(net.Feeders, feeder)

			dtCount := rand.Intn(cfg.DTsPerFeeder.Max-cfg.DTsPerFeeder.Min+1) + cfg.DTsPerFeeder.Min
			dtCoords := PlaceDTsAlongFeeder(route, dtCount, grid)

			for _, dtCoord := range dtCoords {
				dt := model.Transformer{
					ID:               fmt.Sprintf("D-%04d", len(net.Transformers)+1),
					FeederID:         feederID,
					Lat:              dtCoord.Lat,
					Lon:              dtCoord.Lon,
					CapacityKVA:      rand.Intn(400) + 100,
					HouseholdsServed: rand.Intn(400) + 100,
				}
				net.Transformers = append(net.Transformers, dt)

				feederBearing := bearingTo(route[0], route[len(route)-1])
				poles := generateTree(dt, cfg, grid, &poleCounter, feederBearing)
				net.GTPoles = append(net.GTPoles, poles...)
			}
		}
	}

	return net
}

type poleLayout struct {
	totalPoles           int
	mainLineCount        int
	branchCount          int
	branchPolesPerBranch int
}

// planPoleLayout splits totalPoles between the main line and branches so the
// budget is always feasible and branch attachment points can be placed.
func planPoleLayout(cfg Config, totalPoles int) poleLayout {
	const minMainForBranches = 10

	minMain := minMainForBranches
	if totalPoles < minMain {
		minMain = totalPoles
	}

	branchCount := rand.Intn(cfg.BranchesPerLine.Max-cfg.BranchesPerLine.Min+1) + cfg.BranchesPerLine.Min
	maxBranchTotal := totalPoles - minMain
	if maxBranchTotal < 0 {
		maxBranchTotal = 0
	}

	branchLen := rand.Intn(cfg.MaxBranchLength-cfg.MinBranchLength+1) + cfg.MinBranchLength

	for branchCount > 0 && branchCount*branchLen > maxBranchTotal {
		if branchLen > cfg.MinBranchLength {
			branchLen = maxBranchTotal / branchCount
			if branchLen < cfg.MinBranchLength {
				branchLen = cfg.MinBranchLength
			}
		}
		if branchCount*branchLen > maxBranchTotal {
			branchCount--
		}
	}

	if branchCount > 0 && branchCount*branchLen > maxBranchTotal {
		branchCount = 0
		branchLen = 0
	}

	branchTotal := branchCount * branchLen
	mainLineCount := totalPoles - branchTotal
	if mainLineCount < 1 {
		mainLineCount = 1
	}

	return poleLayout{
		totalPoles:           totalPoles,
		mainLineCount:        mainLineCount,
		branchCount:          branchCount,
		branchPolesPerBranch: branchLen,
	}
}

func generateTree(dt model.Transformer, cfg Config, grid *SpatialGrid, poleCounter *int, feederBearing float64) []model.GTPole {
	totalPoles := samplePoleCount(cfg.PolesPerDT)
	if totalPoles < cfg.PolesPerDT.Min {
		totalPoles = cfg.PolesPerDT.Min
	}

	layout := planPoleLayout(cfg, totalPoles)
	mainLineCount := layout.mainLineCount
	branchCount := layout.branchCount
	branchPolesPerBranch := layout.branchPolesPerBranch

	mainBearing := pickMainLineBearing(dt, feederBearing, grid)

	var poles []model.GTPole
	dtCoord := Coord{Lat: dt.Lat, Lon: dt.Lon}

	firstCoord, firstBearing, _ := PlacePoleAlongRoad(dtCoord, mainBearing, grid)
	firstPole := createPole(dt.ID, firstCoord.Lat, firstCoord.Lon, 1, nil, poleCounter)
	poles = append(poles, firstPole)

	branchPoints := generateBranchPositions(mainLineCount, branchCount, cfg)
	totalPoles = layout.totalPoles

	currentCoord := firstCoord
	currentBearing := firstBearing
	currentPole := &poles[0]
	seq := 2

	for i := 2; i <= mainLineCount; i++ {
		nextCoord, nextBearing, distMoved := PlacePoleAlongRoad(currentCoord, currentBearing, grid)
		if distMoved < 1 {
			spacing := rand.Float64()*20 + 30
			nextCoord = projectForward(currentCoord, currentBearing, spacing)
			nextBearing = currentBearing
		}

		parentID := currentPole.ID
		pole := createPole(dt.ID, nextCoord.Lat, nextCoord.Lon, seq, &parentID, poleCounter)
		poles = append(poles, pole)
		currentPole = &poles[len(poles)-1]
		currentCoord = nextCoord
		currentBearing = nextBearing
		seq++
	}

	for _, bp := range sortedBranchPoints(branchPoints) {
		if bp >= len(poles) {
			continue
		}
		parentPole := poles[bp]
		branchAngle := chooseBranchAngle(cfg, mainBearing)
		branchLen := branchPolesPerBranch
		if branchLen > totalPoles-len(poles) {
			branchLen = totalPoles - len(poles)
		}
		if branchLen < cfg.MinBranchLength {
			continue
		}

		branchPoles := generateBranch(parentPole, branchAngle, branchLen, cfg, grid, poleCounter, dt.ID, seq)
		poles = append(poles, branchPoles...)
		seq += len(branchPoles)
		poles[findPoleIndex(poles, parentPole.ID)].IsBranchPoint = true
	}

	poles = extendMainLineToTarget(poles, totalPoles, dt, cfg, grid, poleCounter, &currentBearing, &seq)
	return poles
}

func generateFallbackTree(dt model.Transformer, cfg Config, poleCounter *int) []model.GTPole {
	totalPoles := samplePoleCount(cfg.PolesPerDT)
	if totalPoles < cfg.PolesPerDT.Min {
		totalPoles = cfg.PolesPerDT.Min
	}

	layout := planPoleLayout(cfg, totalPoles)
	mainLineCount := layout.mainLineCount
	branchCount := layout.branchCount
	branchPolesPerBranch := layout.branchPolesPerBranch

	mainBearing := RandomBearing()

	var poles []model.GTPole
	dtCoord := Coord{Lat: dt.Lat, Lon: dt.Lon}
	spacing := rand.Float64()*20 + 35

	firstCoord := PlacePoleAt(dtCoord, mainBearing, int(spacing))
	firstPole := createPole(dt.ID, firstCoord.Lat, firstCoord.Lon, 1, nil, poleCounter)
	poles = append(poles, firstPole)

	branchPoints := generateBranchPositions(mainLineCount, branchCount, cfg)
	totalPoles = layout.totalPoles

	prev := &poles[0]
	currentCoord := firstCoord
	seq := 2

	for i := 2; i <= mainLineCount; i++ {
		mainBearing = DriftBearing(mainBearing, cfg.BearingDriftDeg)
		spacing = rand.Float64()*20 + 35
		coord := PlacePoleAt(currentCoord, mainBearing, int(spacing))

		parentID := prev.ID
		pole := createPole(dt.ID, coord.Lat, coord.Lon, seq, &parentID, poleCounter)
		poles = append(poles, pole)
		prev = &poles[len(poles)-1]
		currentCoord = coord
		seq++
	}

	for _, bp := range sortedBranchPoints(branchPoints) {
		if bp >= len(poles) {
			continue
		}
		parentPole := poles[bp]
		branchAngle := chooseBranchAngle(cfg, mainBearing)
		branchLen := branchPolesPerBranch
		if branchLen > totalPoles-len(poles) {
			branchLen = totalPoles - len(poles)
		}
		if branchLen < cfg.MinBranchLength {
			continue
		}

		branchPoles := generateBranch(parentPole, branchAngle, branchLen, cfg, nil, poleCounter, dt.ID, seq)
		poles = append(poles, branchPoles...)
		seq += len(branchPoles)
		poles[findPoleIndex(poles, parentPole.ID)].IsBranchPoint = true
	}

	poles = extendMainLineToTargetFallback(poles, layout.totalPoles, dt, cfg, poleCounter, &mainBearing, &seq)
	return poles
}

func extendMainLineToTarget(poles []model.GTPole, target int, dt model.Transformer, cfg Config, grid *SpatialGrid, poleCounter *int, bearing *float64, seq *int) []model.GTPole {
	*seq = nextSeqOnLine(poles)
	for len(poles) < target {
		parentPole := poles[len(poles)-1]
		currentCoord := Coord{Lat: parentPole.Lat, Lon: parentPole.Lon}
		nextCoord, nextBearing, distMoved := PlacePoleAlongRoad(currentCoord, *bearing, grid)
		if distMoved < 1 {
			spacing := rand.Float64()*20 + 30
			nextCoord = projectForward(currentCoord, *bearing, spacing)
			nextBearing = *bearing
		}
		parentID := parentPole.ID
		pole := createPole(dt.ID, nextCoord.Lat, nextCoord.Lon, *seq, &parentID, poleCounter)
		poles = append(poles, pole)
		*seq++
		*bearing = nextBearing
	}
	return poles
}

func extendMainLineToTargetFallback(poles []model.GTPole, target int, dt model.Transformer, cfg Config, poleCounter *int, bearing *float64, seq *int) []model.GTPole {
	*seq = nextSeqOnLine(poles)
	currentCoord := Coord{Lat: poles[len(poles)-1].Lat, Lon: poles[len(poles)-1].Lon}
	for len(poles) < target {
		*bearing = DriftBearing(*bearing, cfg.BearingDriftDeg)
		spacing := rand.Float64()*20 + 35
		coord := PlacePoleAt(currentCoord, *bearing, int(spacing))
		parentID := poles[len(poles)-1].ID
		pole := createPole(dt.ID, coord.Lat, coord.Lon, *seq, &parentID, poleCounter)
		poles = append(poles, pole)
		currentCoord = coord
		*seq++
	}
	return poles
}

func nextSeqOnLine(poles []model.GTPole) int {
	max := 0
	for _, p := range poles {
		if p.SeqOnLine > max {
			max = p.SeqOnLine
		}
	}
	return max + 1
}

func generateBranchPositions(mainLineLen, branchCount int, cfg Config) map[int]bool {
	points := make(map[int]bool)
	if branchCount == 0 || mainLineLen < 3 {
		return points
	}

	nearEnd := int(float64(mainLineLen) * cfg.BranchNearDTRange)
	if nearEnd < 5 {
		nearEnd = 5
	}

	nearCount := 0
	if branchCount > 1 {
		nearCount = int(float64(branchCount) * cfg.BranchNearDTPct)
		if nearCount < 1 {
			nearCount = 1
		}
	} else {
		nearCount = 1
	}
	farCount := branchCount - nearCount

	for i := 0; i < nearCount; i++ {
		maxAttempts := 50
		for attempt := 0; attempt < maxAttempts; attempt++ {
			squared := rand.Float64() * rand.Float64()
			pos := 3 + int(squared*float64(nearEnd-3))
			if pos >= mainLineLen-2 {
				pos = mainLineLen - 3
			}
			if pos < 3 {
				pos = 3
			}
			if !points[pos] {
				points[pos] = true
				break
			}
		}
	}

	for i := 0; i < farCount; i++ {
		maxAttempts := 50
		for attempt := 0; attempt < maxAttempts; attempt++ {
			span := mainLineLen - nearEnd - 5
			if span <= 0 {
				span = 1
			}
			pos := nearEnd + rand.Intn(span) + 2
			if pos >= mainLineLen-2 {
				pos = mainLineLen - 3
			}
			if pos < 3 {
				pos = 3
			}
			if !points[pos] {
				points[pos] = true
				break
			}
		}
	}

	return points
}

func sortedBranchPoints(points map[int]bool) []int {
	result := make([]int, 0, len(points))
	for p := range points {
		result = append(result, p)
	}
	sort.Ints(result)
	return result
}

func pickMainLineBearing(dt model.Transformer, feederBearing float64, grid *SpatialGrid) float64 {
	roads := grid.FindRoadsInRadius(Coord{Lat: dt.Lat, Lon: dt.Lon}, 200)
	if len(roads) == 0 {
		return feederBearing + (rand.Float64()*60 - 30)
	}

	bestBearing := feederBearing
	bestScore := -1.0

	for _, road := range roads {
		for i := 0; i < len(road.Geometry)-1; i++ {
			bAB := bearingTo(road.Geometry[i], road.Geometry[i+1])
			bBA := bearingTo(road.Geometry[i+1], road.Geometry[i])

			for _, b := range []float64{bAB, bBA} {
				dLat := math.Abs(road.Geometry[i].Lat-dt.Lat) + math.Abs(road.Geometry[i+1].Lat-dt.Lat)
				dLon := math.Abs(road.Geometry[i].Lon-dt.Lon) + math.Abs(road.Geometry[i+1].Lon-dt.Lon)
				proximity := 1.0 / (1.0 + dLat*10000+dLon*10000)

				aligned := math.Abs(normalizeAngleDifference(b - feederBearing))
				alignment := 1.0 - aligned/180.0

				score := proximity*0.5 + alignment*0.5
				if score > bestScore {
					bestScore = score
					bestBearing = b
				}
			}
		}
	}

	return bestBearing + (rand.Float64()*20 - 10)
}

func generateBranch(startPole model.GTPole, branchBearing float64, count int, cfg Config, grid *SpatialGrid, poleCounter *int, dtID string, startSeq int) []model.GTPole {
	var poles []model.GTPole

	current := startPole
	currentCoord := Coord{Lat: startPole.Lat, Lon: startPole.Lon}
	currentBearing := branchBearing

	for i := 0; i < count; i++ {
		var nextCoord Coord
		var nextBearing float64
		var distMoved float64

		if grid != nil {
			nextCoord, nextBearing, distMoved = PlacePoleAlongRoad(currentCoord, currentBearing, grid)
		} else {
			currentBearing = DriftBearing(currentBearing, cfg.BearingDriftDeg)
			spacing := rand.Intn(cfg.PoleSpacingMeters.Max-cfg.PoleSpacingMeters.Min+1) + cfg.PoleSpacingMeters.Min
			nextCoord = PlacePoleAt(currentCoord, currentBearing, spacing)
			nextBearing = currentBearing
			distMoved = float64(spacing)
		}

		if distMoved < 1 {
			nextCoord = projectForward(currentCoord, currentBearing, 40)
			nextBearing = currentBearing
		}

		parentID := current.ID
		pole := createPole(dtID, nextCoord.Lat, nextCoord.Lon, startSeq+i+1, &parentID, poleCounter)
		poles = append(poles, pole)
		current = poles[len(poles)-1]
		currentCoord = nextCoord
		currentBearing = nextBearing
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

func samplePoleCount(r Range) int {
	if r.Max <= r.Min {
		return r.Min
	}

	mean := float64(r.Min+r.Max) / 2.0
	stddev := float64(r.Max-r.Min) / 4.0

	for {
		val := rand.NormFloat64()*stddev + mean
		if val >= float64(r.Min) && val <= float64(r.Max) {
			return int(val)
		}
	}
}

func findPoleIndex(poles []model.GTPole, id string) int {
	for i, p := range poles {
		if p.ID == id {
			return i
		}
	}
	return -1
}

func PlacePoleAt(from Coord, bearingDeg float64, distanceMeters int) Coord {
	bearingRad := bearingDeg * math.Pi / 180.0

	dLat := float64(distanceMeters) * math.Cos(bearingRad) / MetersPerDegLat
	dLon := float64(distanceMeters) * math.Sin(bearingRad) / MetersPerDegLon

	return Coord{
		Lat: from.Lat + dLat,
		Lon: from.Lon + dLon,
	}
}

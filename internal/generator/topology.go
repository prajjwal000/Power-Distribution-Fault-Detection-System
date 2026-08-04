package generator

import (
	"fmt"
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
	branchCount := rand.Intn(cfg.BranchesPerLine.Max-cfg.BranchesPerLine.Min+1) + cfg.BranchesPerLine.Min

	mainLineLength := int(float64(totalPoles) * 0.7)
	if mainLineLength < 10 {
		mainLineLength = 10
	}
	if mainLineLength > totalPoles-branchCount*3 {
		mainLineLength = totalPoles - branchCount*3
	}

	branchPolesPerBranch := (totalPoles - mainLineLength) / branchCount
	if branchPolesPerBranch < 3 {
		branchPolesPerBranch = 3
	}
	if branchPolesPerBranch > 15 {
		branchPolesPerBranch = 15
	}

	mainBearing := RandomBearing()
	spacing := rand.Intn(cfg.PoleSpacingMeters.Max-cfg.PoleSpacingMeters.Min+1) + cfg.PoleSpacingMeters.Min
	firstCoord := PlacePoleAt(Coord{Lat: dt.Lat, Lon: dt.Lon}, mainBearing, spacing)

	firstPole := createPole(dt.ID, firstCoord.Lat, firstCoord.Lon, 1, nil, poleCounter)
	poles = append(poles, firstPole)

	prev := &poles[len(poles)-1]
	seq := 2

	branchPoints := make(map[int]bool)
	for i := 0; i < branchCount; i++ {
		point := rand.Intn(mainLineLength-5) + 3
		branchPoints[point] = true
	}

	for i := 2; i <= mainLineLength && len(poles) < totalPoles; i++ {
		mainBearing = DriftBearing(mainBearing, cfg.BearingDriftDeg)
		spacing := rand.Intn(cfg.PoleSpacingMeters.Max-cfg.PoleSpacingMeters.Min+1) + cfg.PoleSpacingMeters.Min
		coord := PlacePoleAt(Coord{Lat: prev.Lat, Lon: prev.Lon}, mainBearing, spacing)

		parentID := prev.ID
		pole := createPole(dt.ID, coord.Lat, coord.Lon, seq, &parentID, poleCounter)
		poles = append(poles, pole)
		prev = &poles[len(poles)-1]
		seq++

		if branchPoints[i] && branchCount > 0 {
			branchAngle := chooseBranchAngle(cfg, mainBearing)
			branchLen := branchPolesPerBranch
			if branchLen > totalPoles-len(poles) {
				branchLen = totalPoles - len(poles)
			}

			if branchLen >= 3 {
				branchPoles := generateBranch(pole, branchAngle, branchLen, cfg, poleCounter, dt.ID, seq)
				poles = append(poles, branchPoles...)
				seq += len(branchPoles)

				poles[findPoleIndex(poles, pole.ID)].IsBranchPoint = true
				branchCount--
			}
		}
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

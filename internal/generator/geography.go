package generator

import (
	"math"
	"math/rand"
)

const (
	BangaloreCenterLat = 12.9716
	BangaloreCenterLon = 77.5946
	MetersPerDegLat    = 111000.0
	MetersPerDegLon    = 85000.0
)

type Coord struct {
	Lat float64
	Lon float64
}

func PlaceSubstations(count int, grid *SpatialGrid) []Coord {
	majorRoads := filterMajorRoads(grid)
	if len(majorRoads) == 0 {
		return fallbackSubstations(count)
	}

	bearingStep := 360.0 / float64(count)
	results := make([]Coord, 0, count)

	for i := 0; i < count; i++ {
		targetBearing := bearingStep * float64(i)
		coord := placeOnMajorRoad(grid, majorRoads, targetBearing)
		results = append(results, coord)
	}

	return results
}

func filterMajorRoads(grid *SpatialGrid) []*RoadSegment {
	var major []*RoadSegment
	for i := range grid.Segments {
		seg := &grid.Segments[i]
		switch seg.Highway {
		case "trunk", "primary", "secondary":
			if len(seg.Geometry) >= 3 {
				major = append(major, seg)
			}
		}
	}
	return major
}

func placeOnMajorRoad(grid *SpatialGrid, majorRoads []*RoadSegment, targetBearing float64) Coord {
	bestSeg := majorRoads[rand.Intn(len(majorRoads))]
	bestPt := bestSeg.Geometry[len(bestSeg.Geometry)/2]

	distFromCenter := 500.0 + rand.Float64()*2000.0
	candidate := projectForward(Coord{Lat: BangaloreCenterLat, Lon: BangaloreCenterLon}, targetBearing, distFromCenter)
	snapped := grid.SnapToRoad(candidate)

	dCenter := HaversineDistance(Coord{Lat: BangaloreCenterLat, Lon: BangaloreCenterLon}, snapped)
	if dCenter > 3000 || dCenter < 200 {
		snapped = bestPt
	}

	return snapped
}

func fallbackSubstations(count int) []Coord {
	results := make([]Coord, 0, count)
	bearingStep := 360.0 / float64(count)
	for i := 0; i < count; i++ {
		bearing := bearingStep * float64(i)
		dist := 2000.0 + rand.Float64()*1000.0
		results = append(results, projectForward(
			Coord{Lat: BangaloreCenterLat, Lon: BangaloreCenterLon},
			bearing, dist,
		))
	}
	return results
}

func PlaceFeederRoute(start Coord, targetBearing float64, lengthMeters int, grid *SpatialGrid) []Coord {
	path := []Coord{start}
	current := start
	remaining := float64(lengthMeters)
	currentBearing := targetBearing + (rand.Float64()*30 - 15)

	seg, bearing := grid.PickRoadDirection(current, currentBearing)
	if seg == nil {
		return straightRoute(start, targetBearing, lengthMeters)
	}
	currentBearing = bearing

	for remaining > 0 && len(path) < 200 {
		stepDist := math.Min(remaining, 300.0+rand.Float64()*200.0)
		candidate := projectForward(current, currentBearing, stepDist)
		snapped := grid.SnapToRoad(candidate)
		d := HaversineDistance(current, snapped)

		if d < 1 {
			nextSeg, nextBearing := grid.PickRoadDirection(current, currentBearing)
			if nextSeg == nil {
				break
			}
			currentBearing = nextBearing + (rand.Float64()*20 - 10)
			continue
		}

		path = append(path, snapped)
		current = snapped
		remaining -= d

		if rand.Float64() < 0.15 {
			intersections := grid.FindIntersections(current, 150)
			if len(intersections) > 1 {
				for _, ix := range intersections[1:] {
					dToIx := HaversineDistance(current, ix)
					if dToIx > 30 && dToIx < 300 {
						ixBearing := bearingTo(current, ix)
						currentBearing = ixBearing
						break
					}
				}
			}
		}
	}

	return path
}

func straightRoute(start Coord, bearing float64, lengthMeters int) []Coord {
	path := []Coord{start}
	current := start
	remaining := float64(lengthMeters)
	for remaining > 0 {
		step := math.Min(remaining, 200.0)
		current = projectForward(current, bearing, step)
		path = append(path, current)
		remaining -= step
	}
	return path
}

func PlaceDTsAlongFeeder(route []Coord, count int, grid *SpatialGrid) []Coord {
	if len(route) < 2 || count == 0 {
		return nil
	}

	totalLen := 0.0
	segLens := make([]float64, len(route)-1)
	for i := 0; i < len(route)-1; i++ {
		d := HaversineDistance(route[i], route[i+1])
		segLens[i] = d
		totalLen += d
	}

	spacing := totalLen / float64(count+1)
	if spacing < 100 {
		spacing = 100
	}

	results := make([]Coord, 0, count)
	accumDist := spacing * (0.5 + rand.Float64()*0.3)
	segIdx := 0
	segAccum := 0.0

	for i := 0; i < count && segIdx < len(route)-1; i++ {
		for segIdx < len(route)-1 {
			segLen := segLens[segIdx]
			if segAccum+segLen >= accumDist {
				t := (accumDist - segAccum) / segLen
				lat := route[segIdx].Lat + t*(route[segIdx+1].Lat-route[segIdx].Lat)
				lon := route[segIdx].Lon + t*(route[segIdx+1].Lon-route[segIdx].Lon)
				dtCoord := Coord{Lat: lat, Lon: lon}
				if grid != nil {
					dtCoord = grid.SnapToRoad(dtCoord)
				}
				results = append(results, dtCoord)
				break
			}
			segAccum += segLen
			segIdx++
		}
		accumDist += spacing + (rand.Float64()*60 - 30)
	}

	return results
}

func PlacePoleAlongRoad(from Coord, bearing float64, grid *SpatialGrid) (Coord, float64, float64) {
	snapped, seg := grid.SnapToRoadKeepSeg(from)
	if seg == nil {
		next := projectForward(from, bearing, 40)
		return next, bearing, 40
	}
	bestNext := snapped
	bestDist := 0.0
	bestBearing := bearing

	bestIdx := -1
	for i, pt := range seg.Geometry {
		d := HaversineDistance(snapped, pt)
		if d < 5 || d > 80 {
			continue
		}
		ptBearing := bearingTo(snapped, pt)
		angleDiff := math.Abs(normalizeAngleDifference(ptBearing - bearing))
		if angleDiff > 100 {
			continue
		}
		if bestIdx == -1 || d > bestDist {
			bestIdx = i
			bestDist = d
			bestNext = pt
			bestBearing = ptBearing
		}
	}

	if bestIdx >= 0 {
		nearestOnSeg := nearestPointOnSegment(bestNext, seg)
		d := HaversineDistance(snapped, nearestOnSeg)
		if d > 2 {
			return nearestOnSeg, bestBearing, d
		}
	}

	candidate := projectForward(snapped, bearing, 40)
	snappedCandidate := grid.SnapToRoad(candidate)
	d := HaversineDistance(snapped, snappedCandidate)
	if d > 2 {
		newBearing := bearingTo(snapped, snappedCandidate)
		return snappedCandidate, newBearing, d
	}

	next := projectForward(snapped, bearing, 40)
	d = HaversineDistance(snapped, next)
	return next, bearing, d
}

func HaversineDistance(a, b Coord) float64 {
	lat1 := a.Lat * math.Pi / 180.0
	lat2 := b.Lat * math.Pi / 180.0
	dLat := (b.Lat - a.Lat) * math.Pi / 180.0
	dLon := (b.Lon - a.Lon) * math.Pi / 180.0

	h := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLon/2)*math.Sin(dLon/2)

	return 2 * 6371000 * math.Asin(math.Sqrt(h))
}

func RandomBearing() float64 {
	return rand.Float64() * 360.0
}

func DriftBearing(bearing float64, maxDrift float64) float64 {
	drift := rand.Float64()*2*maxDrift - maxDrift
	return NormalizeBearing(bearing + drift)
}

func NormalizeBearing(bearing float64) float64 {
	for bearing < 0 {
		bearing += 360
	}
	for bearing >= 360 {
		bearing -= 360
	}
	return bearing
}

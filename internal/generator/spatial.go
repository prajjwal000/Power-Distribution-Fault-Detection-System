package generator

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type RoadSegment struct {
	ID       int    `json:"id"`
	Highway  string `json:"highway"`
	Name     string `json:"name"`
	Geometry []Coord `json:"geometry"`
}

type SpatialGrid struct {
	CellSizeLat float64
	CellSizeLon float64
	Cells       map[[2]int][]int
	Segments    []RoadSegment
}

type spatialIndexJSON struct {
	CellSizeLat float64            `json:"cell_size_lat"`
	CellSizeLon float64            `json:"cell_size_lon"`
	Cells       map[string][]int   `json:"cells"`
}

type spatialSegmentsJSON struct {
	ID       int      `json:"id"`
	Highway  string   `json:"highway"`
	Name     string   `json:"name"`
	Geometry []struct {
		Lat float64 `json:"lat"`
		Lon float64 `json:"lon"`
	} `json:"geometry"`
}

var (
	cachedGrid *SpatialGrid
	cacheOnce  sync.Once
	cacheErr   error
)

func LoadSpatialIndex(dataDir string) (*SpatialGrid, error) {
	cacheOnce.Do(func() {
		cachedGrid, cacheErr = loadSpatialIndexUncached(dataDir)
	})
	return cachedGrid, cacheErr
}

func loadSpatialIndexUncached(dataDir string) (*SpatialGrid, error) {
	indexPath := filepath.Join(dataDir, "road_index.json")
	segmentsPath := filepath.Join(dataDir, "road_segments.json")

	// Load segments
	segData, err := os.ReadFile(segmentsPath)
	if err != nil {
		return nil, fmt.Errorf("reading segments: %w", err)
	}
	var rawSegs []spatialSegmentsJSON
	if err := json.Unmarshal(segData, &rawSegs); err != nil {
		return nil, fmt.Errorf("parsing segments: %w", err)
	}

	segments := make([]RoadSegment, len(rawSegs))
	for i, rs := range rawSegs {
		segments[i] = RoadSegment{
			ID:      rs.ID,
			Highway: rs.Highway,
			Name:    rs.Name,
			Geometry: make([]Coord, len(rs.Geometry)),
		}
		for j, pt := range rs.Geometry {
			segments[i].Geometry[j] = Coord{Lat: pt.Lat, Lon: pt.Lon}
		}
	}

	// Load index
	idxData, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, fmt.Errorf("reading index: %w", err)
	}
	var rawIdx spatialIndexJSON
	if err := json.Unmarshal(idxData, &rawIdx); err != nil {
		return nil, fmt.Errorf("parsing index: %w", err)
	}

	cells := make(map[[2]int][]int, len(rawIdx.Cells))
	for k, v := range rawIdx.Cells {
		parts := strings.SplitN(k, ",", 2)
		if len(parts) != 2 {
			continue
		}
		ci, err1 := strconv.Atoi(parts[0])
		cj, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil {
			continue
		}
		cells[[2]int{ci, cj}] = v
	}

	return &SpatialGrid{
		CellSizeLat: rawIdx.CellSizeLat,
		CellSizeLon: rawIdx.CellSizeLon,
		Cells:       cells,
		Segments:    segments,
	}, nil
}

func (g *SpatialGrid) cellKey(lat, lon float64) [2]int {
	ci := int(math.Floor(lat / g.CellSizeLat))
	cj := int(math.Floor(lon / g.CellSizeLon))
	return [2]int{ci, cj}
}

func (g *SpatialGrid) segmentIndicesInRadius(lat, lon, radiusMeters float64) []int {
	rLat := radiusMeters / MetersPerDegLat
	rLon := radiusMeters / MetersPerDegLon

	centerCi := int(math.Floor(lat / g.CellSizeLat))
	centerCj := int(math.Floor(lon / g.CellSizeLon))
	dCi := int(math.Ceil(rLat / g.CellSizeLat)) + 1
	dCj := int(math.Ceil(rLon / g.CellSizeLon)) + 1

	seen := make(map[int]bool)
	var result []int
	for di := -dCi; di <= dCi; di++ {
		for dj := -dCj; dj <= dCj; dj++ {
			key := [2]int{centerCi + di, centerCj + dj}
			for _, segIdx := range g.Cells[key] {
				if !seen[segIdx] {
					seen[segIdx] = true
					result = append(result, segIdx)
				}
			}
		}
	}
	return result
}

func (g *SpatialGrid) FindNearestRoad(point Coord, radiusMeters float64) (*RoadSegment, float64) {
	if radiusMeters <= 0 {
		radiusMeters = 150
	}
	indices := g.segmentIndicesInRadius(point.Lat, point.Lon, radiusMeters)
	if len(indices) == 0 {
		return nil, math.MaxFloat64
	}

	var bestSeg *RoadSegment
	bestDist := math.MaxFloat64
	const maxChecks = 64

	for i, idx := range indices {
		if i >= maxChecks {
			break
		}
		seg := &g.Segments[idx]
		d := distToSegment(point, seg)
		if d < bestDist {
			bestDist = d
			bestSeg = seg
			if bestDist < 3 {
				break
			}
		}
	}
	return bestSeg, bestDist
}

func (g *SpatialGrid) SnapToRoad(point Coord) Coord {
	seg, _ := g.FindNearestRoad(point, 150)
	if seg == nil {
		return point
	}
	return nearestPointOnSegment(point, seg)
}

func (g *SpatialGrid) SnapToRoadKeepSeg(point Coord) (Coord, *RoadSegment) {
	seg, _ := g.FindNearestRoad(point, 150)
	if seg == nil {
		return point, nil
	}
	return nearestPointOnSegment(point, seg), seg
}

func (g *SpatialGrid) FindRoadsInRadius(point Coord, radiusMeters float64) []*RoadSegment {
	indices := g.segmentIndicesInRadius(point.Lat, point.Lon, radiusMeters)
	radiusDegLat := radiusMeters / MetersPerDegLat
	radiusDegLon := radiusMeters / MetersPerDegLon
	radiusDeg := math.Max(radiusDegLat, radiusDegLon)

	var result []*RoadSegment
	for _, idx := range indices {
		seg := &g.Segments[idx]
		for _, pt := range seg.Geometry {
			dLat := math.Abs(pt.Lat - point.Lat)
			dLon := math.Abs(pt.Lon - point.Lon)
			if dLat <= radiusDeg && dLon <= radiusDeg {
				result = append(result, seg)
				break
			}
		}
	}
	return result
}

func (g *SpatialGrid) FollowRoad(start Coord, bearing float64, distanceMeters int) ([]Coord, float64) {
	snapped, seg := g.SnapToRoadKeepSeg(start)
	if seg == nil {
		return []Coord{start}, bearing
	}
	_ = snapped
	path := []Coord{snapped}
	remaining := float64(distanceMeters)
	currentBearing := bearing

	for remaining > 0 && len(path) < 500 {
		bestNext, distMoved, newBearing := walkAlongRoad(path[len(path)-1], currentBearing, seg, remaining, g)
		if distMoved < 1 {
			break
		}
		path = append(path, bestNext)
		remaining -= distMoved
		currentBearing = newBearing
	}

	return path, bearing
}

func walkAlongRoad(from Coord, bearing float64, seg *RoadSegment, maxDist float64, g *SpatialGrid) (Coord, float64, float64) {
	bestIdx := -1
	bestScore := -1.0
	bestDist := 0.0

	for i, pt := range seg.Geometry {
		d := HaversineDistance(from, pt)
		if d < 0.5 || d > maxDist*1.5 {
			continue
		}
		ptBearing := bearingTo(from, pt)
		angleDiff := math.Abs(normalizeAngleDifference(ptBearing - bearing))
		if angleDiff > 90 {
			continue
		}
		score := (1.0 - angleDiff/90.0) * (1.0 / (1.0 + d/50.0))
		if score > bestScore {
			bestScore = score
			bestIdx = i
			bestDist = d
		}
	}

	if bestIdx >= 0 {
		pt := seg.Geometry[bestIdx]
		newBearing := bearingTo(from, pt)
		return pt, bestDist, newBearing
	}

	nextPt := projectForward(from, bearing, maxDist)
	nearestOnSeg := nearestPointOnSegment(nextPt, seg)
	d := HaversineDistance(from, nearestOnSeg)
	if d > maxDist {
		d = maxDist
		nearestOnSeg = projectForward(from, bearing, d)
	}
	return nearestOnSeg, d, bearing
}

func distToSegment(point Coord, seg *RoadSegment) float64 {
	minD := math.MaxFloat64
	for i := 0; i < len(seg.Geometry)-1; i++ {
		a := seg.Geometry[i]
		b := seg.Geometry[i+1]
		if quickRejectSegment(point, a, b, minD) {
			continue
		}
		d := distToLineSegment(point, a, b)
		if d < minD {
			minD = d
		}
	}
	return minD
}

func quickRejectSegment(point, a, b Coord, bestDist float64) bool {
	minLat := a.Lat
	maxLat := a.Lat
	minLon := a.Lon
	maxLon := a.Lon
	if b.Lat < minLat {
		minLat = b.Lat
	}
	if b.Lat > maxLat {
		maxLat = b.Lat
	}
	if b.Lon < minLon {
		minLon = b.Lon
	}
	if b.Lon > maxLon {
		maxLon = b.Lon
	}

	margin := bestDist / MetersPerDegLat
	if margin < 0.0003 {
		margin = 0.0003
	}
	if point.Lat < minLat-margin || point.Lat > maxLat+margin ||
		point.Lon < minLon-margin || point.Lon > maxLon+margin {
		return true
	}
	return false
}

func distToLineSegment(point, a, b Coord) float64 {
	dA := HaversineDistance(point, a)
	dB := HaversineDistance(point, b)
	dAB := HaversineDistance(a, b)

	if dAB < 0.1 {
		return math.Min(dA, dB)
	}

	t := ((point.Lat-a.Lat)*(b.Lat-a.Lat) + (point.Lon-a.Lon)*(b.Lon-a.Lon)) / (dAB*dAB / (MetersPerDegLat * MetersPerDegLon * MetersPerDegLat * MetersPerDegLon))
	t = math.Max(0, math.Min(1, t))

	projLat := a.Lat + t*(b.Lat-a.Lat)
	projLon := a.Lon + t*(b.Lon-a.Lon)
	proj := Coord{Lat: projLat, Lon: projLon}
	return HaversineDistance(point, proj)
}

func nearestPointOnSegment(point Coord, seg *RoadSegment) Coord {
	bestDist := math.MaxFloat64
	bestPt := seg.Geometry[0]

	for i := 0; i < len(seg.Geometry)-1; i++ {
		pt := closestPointOnLine(point, seg.Geometry[i], seg.Geometry[i+1])
		d := HaversineDistance(point, pt)
		if d < bestDist {
			bestDist = d
			bestPt = pt
		}
	}
	return bestPt
}

func closestPointOnLine(point, a, b Coord) Coord {
	dAB := HaversineDistance(a, b)
	if dAB < 0.1 {
		return a
	}

	t := ((point.Lat-a.Lat)*(b.Lat-a.Lat) + (point.Lon-a.Lon)*(b.Lon-a.Lon)) / (dAB*dAB / (MetersPerDegLat * MetersPerDegLon * MetersPerDegLat * MetersPerDegLon))
	t = math.Max(0, math.Min(1, t))

	return Coord{
		Lat: a.Lat + t*(b.Lat-a.Lat),
		Lon: a.Lon + t*(b.Lon-a.Lon),
	}
}

func bearingTo(from, to Coord) float64 {
	dLon := (to.Lon - from.Lon) * math.Pi / 180.0
	lat1 := from.Lat * math.Pi / 180.0
	lat2 := to.Lat * math.Pi / 180.0

	y := math.Sin(dLon) * math.Cos(lat2)
	x := math.Cos(lat1)*math.Sin(lat2) - math.Sin(lat1)*math.Cos(lat2)*math.Cos(dLon)

	bearing := math.Atan2(y, x) * 180.0 / math.Pi
	return NormalizeBearing(bearing)
}

func normalizeAngleDifference(angle float64) float64 {
	for angle > 180 {
		angle -= 360
	}
	for angle < -180 {
		angle += 360
	}
	return angle
}

func projectForward(from Coord, bearing float64, distanceMeters float64) Coord {
	bearingRad := bearing * math.Pi / 180.0
	dLat := distanceMeters * math.Cos(bearingRad) / MetersPerDegLat
	dLon := distanceMeters * math.Sin(bearingRad) / MetersPerDegLon
	return Coord{Lat: from.Lat + dLat, Lon: from.Lon + dLon}
}

func (g *SpatialGrid) FindIntersections(point Coord, radiusMeters float64) []Coord {
	roads := g.FindRoadsInRadius(point, radiusMeters)
	nodeCount := make(map[Coord]int)
	for _, road := range roads {
		for _, pt := range road.Geometry {
			key := Coord{
				Lat: math.Round(pt.Lat*100000) / 100000,
				Lon: math.Round(pt.Lon*100000) / 100000,
			}
			nodeCount[key]++
		}
	}

	var intersections []Coord
	for pt, count := range nodeCount {
		if count >= 2 {
			intersections = append(intersections, pt)
		}
	}

	sort.Slice(intersections, func(i, j int) bool {
		di := HaversineDistance(point, intersections[i])
		dj := HaversineDistance(point, intersections[j])
		return di < dj
	})

	return intersections
}

func (g *SpatialGrid) PickRoadDirection(from Coord, targetBearing float64) (*RoadSegment, float64) {
	roads := g.FindRoadsInRadius(from, 200)
	if len(roads) == 0 {
		return nil, targetBearing
	}

	type scored struct {
		seg     *RoadSegment
		bearing float64
		score   float64
	}

	var candidates []scored
	for _, road := range roads {
		for i := 0; i < len(road.Geometry)-1; i++ {
			a := road.Geometry[i]
			b := road.Geometry[i+1]

			dA := HaversineDistance(from, a)
			dB := HaversineDistance(from, b)
			if dA > 200 && dB > 200 {
				continue
			}

			bearingAB := bearingTo(a, b)
			bearingBA := bearingTo(b, a)

			for _, bDir := range []float64{bearingAB, bearingBA} {
				angleDiff := math.Abs(normalizeAngleDifference(bDir - targetBearing))
				if angleDiff > 90 {
					continue
				}
				score := 1.0 - angleDiff/90.0
				candidates = append(candidates, scored{seg: road, bearing: bDir, score: score})
			}
		}
	}

	if len(candidates) == 0 {
		return nil, targetBearing
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	best := candidates[0]
	return best.seg, best.bearing
}

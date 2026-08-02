package generator

import (
	"math"
	"math/rand"
)

// currently just a naive stuv implementation

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

func PlaceSubstation(idx int) Coord {
	offset := float64(idx) * 0.005
	return Coord{
		Lat: BangaloreCenterLat + offset,
		Lon: BangaloreCenterLon + offset*0.5,
	}
}

func PlaceTransformer(dtIdx int, feederIdx int) Coord {
	gridSize := 8
	row := dtIdx / gridSize
	col := dtIdx % gridSize

	baseLat := BangaloreCenterLat + float64(row)*0.003 - 0.01
	baseLon := BangaloreCenterLon + float64(col)*0.003 - 0.01

	jitterLat := rand.Float64()*0.002 - 0.001
	jitterLon := rand.Float64()*0.002 - 0.001

	return Coord{
		Lat: baseLat + float64(feederIdx)*0.001 + jitterLat,
		Lon: baseLon + float64(feederIdx)*0.001 + jitterLon,
	}
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

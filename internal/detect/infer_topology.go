package detect

import (
	"math"
	"sort"
)

type PoleCoord struct {
	ID     string
	Lat    float64
	Lon    float64
	Dist   float64
	Angle  float64
}

type InferredEdge struct {
	ParentID   string
	ChildID    string
	Distance   float64
	Confidence float64
	Method     string
}

type InferredTopology struct {
	DTID   string
	Edges  []InferredEdge
	Method string
}

type mstEdge struct {
	Parent   string
	Child    string
	Distance float64
}

func InferTopologyForDT(dtID string, topo *TopologyIndex) *InferredTopology {
	poles := topo.PolesForDT(dtID)
	if len(poles) < 2 {
		return &InferredTopology{DTID: dtID, Edges: []InferredEdge{}, Method: "mst_radial"}
	}

	dt := topo.TransformerByID[dtID]
	if dt == nil {
		return &InferredTopology{DTID: dtID, Edges: []InferredEdge{}, Method: "mst_radial"}
	}

	var poleCoords []PoleCoord
	for _, pid := range poles {
		p := topo.PoleByID[pid]
		if p == nil {
			continue
		}
		angle := math.Atan2(p.Lat-dt.Lat, p.Lon-dt.Lon)
		dist := haversine(dt.Lat, dt.Lon, p.Lat, p.Lon)
		poleCoords = append(poleCoords, PoleCoord{
			ID: pid, Lat: p.Lat, Lon: p.Lon, Dist: dist, Angle: angle,
		})
	}

	if len(poleCoords) < 2 {
		return &InferredTopology{DTID: dtID, Edges: []InferredEdge{}, Method: "mst_radial"}
	}

	sort.Slice(poleCoords, func(i, j int) bool {
		return poleCoords[i].Angle < poleCoords[j].Angle
	})

	_ = poleCoords[0].ID // root pole (closest to DT by angle)

	edges := buildMST(poleCoords)

	var inferredEdges []InferredEdge
	for _, e := range edges {
		inferredEdges = append(inferredEdges, InferredEdge{
			ParentID:   e.Parent,
			ChildID:    e.Child,
			Distance:   e.Distance,
			Confidence: computeEdgeConfidence(e, poleCoords, dt),
			Method:     "mst_radial",
		})
	}

	return &InferredTopology{
		DTID:   dtID,
		Edges:  inferredEdges,
		Method: "mst_radial",
	}
}

func buildMST(poles []PoleCoord) []mstEdge {
	n := len(poles)
	if n == 0 {
		return nil
	}

	idToIdx := make(map[string]int)
	for i, p := range poles {
		idToIdx[p.ID] = i
	}

	distMatrix := make([][]float64, n)
	for i := 0; i < n; i++ {
		distMatrix[i] = make([]float64, n)
		for j := 0; j < n; j++ {
			if i == j {
				distMatrix[i][j] = 0
			} else {
				distMatrix[i][j] = haversine(poles[i].Lat, poles[i].Lon, poles[j].Lat, poles[j].Lon)
			}
		}
	}

	inMST := make([]bool, n)
	parent := make([]int, n)
	key := make([]float64, n)
	for i := range key {
		key[i] = math.Inf(1)
		parent[i] = -1
	}

	key[0] = 0

	for count := 0; count < n-1; count++ {
		u := -1
		minKey := math.Inf(1)
		for v := 0; v < n; v++ {
			if !inMST[v] && key[v] < minKey {
				minKey = key[v]
				u = v
			}
		}

		if u == -1 {
			break
		}

		inMST[u] = true

		for v := 0; v < n; v++ {
			if !inMST[v] && distMatrix[u][v] < key[v] {
				key[v] = distMatrix[u][v]
				parent[v] = u
			}
		}
	}

	var edges []mstEdge
	for v := 1; v < n; v++ {
		if parent[v] != -1 {
			edges = append(edges, mstEdge{
				Parent:   poles[parent[v]].ID,
				Child:    poles[v].ID,
				Distance: distMatrix[parent[v]][v],
			})
		}
	}

	return edges
}

func computeEdgeConfidence(e mstEdge, poles []PoleCoord, dt *Transformer) float64 {
	base := 0.6

	maxDist := 0.0
	for _, p := range poles {
		d := haversine(dt.Lat, dt.Lon, p.Lat, p.Lon)
		if d > maxDist {
			maxDist = d
		}
	}

	if maxDist > 0 {
		normDist := e.Distance / maxDist
		base -= normDist * 0.2
	}

	if base < 0.3 {
		base = 0.3
	}
	if base > 0.9 {
		base = 0.9
	}

	return base
}

func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371000

	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}

func InferAllMissingTopology(topo *TopologyIndex) map[string]*InferredTopology {
	result := make(map[string]*InferredTopology)

	for dtID, hasTopo := range topo.HasTopology {
		if !hasTopo {
			result[dtID] = InferTopologyForDT(dtID, topo)
		}
	}

	return result
}
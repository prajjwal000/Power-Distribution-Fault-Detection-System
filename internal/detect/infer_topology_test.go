package detect

import (
	"testing"
)

func TestInferTopologyForDT_SimpleLine(t *testing.T) {
	topo := buildTestTopoWithCoords()

	inferred := InferTopologyForDT("D-02", topo)

	if len(inferred.Edges) == 0 {
		t.Fatal("expected inferred edges for DT without topology")
	}

	t.Logf("Inferred %d edges for D-02", len(inferred.Edges))
	for _, e := range inferred.Edges {
		t.Logf("  %s -> %s (dist: %.1fm, conf: %.2f)", e.ParentID, e.ChildID, e.Distance, e.Confidence)
	}
}

func TestInferTopologyForDT_WithKnownTopology(t *testing.T) {
	topo := buildTestTopoWithCoords()

	inferred := InferTopologyForDT("D-01", topo)

	if len(inferred.Edges) > 0 {
		t.Logf("D-01 has known topology, but got %d inferred edges", len(inferred.Edges))
	}
}

func TestInferAllMissingTopology(t *testing.T) {
	topo := buildTestTopoWithCoords()

	all := InferAllMissingTopology(topo)

	t.Logf("Inferred topology for %d DTs", len(all))
	for dtID, inf := range all {
		t.Logf("  %s: %d edges", dtID, len(inf.Edges))
	}
}

func TestHaversine(t *testing.T) {
	// At Bangalore lat (~13°), 0.001 deg longitude ≈ 97m, 0.001 deg latitude ≈ 111m
	dist := haversine(12.9716, 77.5946, 12.9716, 77.5956) // 0.001 deg longitude
	if dist < 80 || dist > 120 {
		t.Errorf("expected ~97m for 0.001 deg longitude at 13°N, got %.1f", dist)
	}

	dist = haversine(12.9716, 77.5946, 13.9716, 77.5946)
	if dist < 110000 || dist > 112000 {
		t.Errorf("expected ~111km for 1 deg latitude, got %.1f", dist)
	}
}

func buildTestTopoWithCoords() *TopologyIndex {
	idx := &TopologyIndex{
		PoleByID:        make(map[string]*Pole),
		Children:        make(map[string][]string),
		Parent:          make(map[string]string),
		DTIDToPoles:     make(map[string][]string),
		DeviceToPole:    make(map[string]*Pole),
		HasTopology:     make(map[string]bool),
		TransformerByID: make(map[string]*Transformer),
		FeederByID:      make(map[string]*Feeder),
		FeederToSub:     make(map[string]string),
	}

	idx.TransformerByID["D-01"] = &Transformer{ID: "D-01", FeederID: "F-01", Lat: 12.9700, Lon: 77.5900}
	idx.TransformerByID["D-02"] = &Transformer{ID: "D-02", FeederID: "F-01", Lat: 12.9800, Lon: 77.6000}
	idx.HasTopology["D-01"] = true
	idx.HasTopology["D-02"] = false

	// D-01: Known topology (4 poles in a line)
	d01Poles := []struct {
		id       string
		lat      float64
		lon      float64
		seq      *int
		parentID *string
		deviceID *string
	}{
		{"P-1", 12.9690, 77.5890, intPtr(1), nil, stringPtr("DEV-1")},
		{"P-2", 12.9695, 77.5895, intPtr(2), stringPtr("P-1"), stringPtr("DEV-2")},
		{"P-3", 12.9700, 77.5900, intPtr(3), stringPtr("P-2"), stringPtr("DEV-3")},
		{"P-4", 12.9705, 77.5905, intPtr(4), stringPtr("P-3"), stringPtr("DEV-4")},
	}

	for _, p := range d01Poles {
		pole := &Pole{ID: p.id, DTID: "D-01", FeederID: "F-01", Lat: p.lat, Lon: p.lon, SeqOnLine: p.seq, ParentID: p.parentID, DeviceID: p.deviceID}
		idx.PoleByID[p.id] = pole
		idx.DTIDToPoles["D-01"] = append(idx.DTIDToPoles["D-01"], p.id)
		if p.deviceID != nil {
			idx.DeviceToPole[*p.deviceID] = pole
		}
		if p.parentID != nil {
			idx.Parent[p.id] = *p.parentID
		}
	}

	// D-02: Unknown topology (6 poles in a branch pattern)
	d02Poles := []struct {
		id  string
		lat float64
		lon float64
	}{
		{"P-10", 12.9790, 77.5990},
		{"P-11", 12.9795, 77.5995},
		{"P-12", 12.9800, 77.6000},
		{"P-13", 12.9805, 77.6005},
		{"P-14", 12.9800, 77.6010},
		{"P-15", 12.9795, 77.6015},
	}

	for _, p := range d02Poles {
		pole := &Pole{ID: p.id, DTID: "D-02", FeederID: "F-01", Lat: p.lat, Lon: p.lon}
		idx.PoleByID[p.id] = pole
		idx.DTIDToPoles["D-02"] = append(idx.DTIDToPoles["D-02"], p.id)
	}

	return idx
}
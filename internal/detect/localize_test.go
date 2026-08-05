package detect

import (
	"testing"
)

func intPtr(v int) *int    { return &v }
func stringPtr(v string) *string { return &v }

func buildTestTopo() *TopologyIndex {
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

	poles := []struct {
		id       string
		dtID     string
		seq      *int
		parentID *string
		deviceID *string
	}{
		{"P-1", "D-01", intPtr(1), nil, stringPtr("DEV-1")},
		{"P-2", "D-01", intPtr(2), stringPtr("P-1"), stringPtr("DEV-2")},
		{"P-3", "D-01", intPtr(3), stringPtr("P-2"), stringPtr("DEV-3")},
		{"P-4", "D-01", intPtr(4), stringPtr("P-3"), stringPtr("DEV-4")},
		{"P-5", "D-01", intPtr(5), stringPtr("P-4"), stringPtr("DEV-5")},
		{"P-6", "D-01", intPtr(6), stringPtr("P-5"), stringPtr("DEV-6")},
		{"P-7", "D-01", intPtr(7), stringPtr("P-6"), stringPtr("DEV-7")},
		{"P-8", "D-01", intPtr(8), stringPtr("P-7"), stringPtr("DEV-8")},
	}

	for _, p := range poles {
		pole := &Pole{
			ID:       p.id,
			DTID:     p.dtID,
			FeederID: "F-01",
			SeqOnLine: p.seq,
			ParentID: p.parentID,
			DeviceID: p.deviceID,
		}
		idx.PoleByID[p.id] = pole
		idx.DTIDToPoles[p.dtID] = append(idx.DTIDToPoles[p.dtID], p.id)
		if p.deviceID != nil {
			idx.DeviceToPole[*p.deviceID] = pole
		}
		if p.parentID != nil {
			idx.Parent[p.id] = *p.parentID
		}
	}

	idx.HasTopology["D-01"] = true
	idx.HasTopology["D-02"] = false

	return idx
}

func TestLocalizeKnownTopology_SingleSpanFault(t *testing.T) {
	topo := buildTestTopo()

	job := DetectionJob{
		DTID:      "D-01",
		DarkPoles: []string{"P-5", "P-6", "P-7", "P-8"},
		LitPoles:  []string{"P-1", "P-2", "P-3", "P-4"},
	}

	candidates := Localize(job, topo)

	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}

	c := candidates[0]
	if c.Scope != ScopeSpan {
		t.Errorf("expected scope span, got %s", c.Scope)
	}
	if c.TargetID == "" {
		t.Error("expected non-empty target ID")
	}
	if c.Confidence < 0.5 || c.Confidence > 1.0 {
		t.Errorf("confidence out of range: %f", c.Confidence)
	}
	if len(c.AffectedPoles) != 4 {
		t.Errorf("expected 4 affected poles, got %d", len(c.AffectedPoles))
	}
}

func TestLocalizeKnownTopology_TwoFaults(t *testing.T) {
	topo := buildTestTopo()

	job := DetectionJob{
		DTID:      "D-01",
		DarkPoles: []string{"P-2", "P-3", "P-6", "P-7"},
		LitPoles:  []string{"P-1", "P-4", "P-5", "P-8"},
	}

	candidates := Localize(job, topo)

	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates for two faults, got %d", len(candidates))
	}

	for _, c := range candidates {
		if c.Scope != ScopeSpan {
			t.Errorf("expected scope span, got %s", c.Scope)
		}
		if c.Confidence < 0.1 {
			t.Errorf("confidence too low: %f", c.Confidence)
		}
	}
}

func TestLocalizeKnownTopology_FullDTOutage(t *testing.T) {
	topo := buildTestTopo()

	job := DetectionJob{
		DTID:      "D-01",
		DarkPoles: []string{"P-1", "P-2", "P-3", "P-4", "P-5", "P-6", "P-7", "P-8"},
		LitPoles:  []string{},
	}

	candidates := Localize(job, topo)

	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}

	c := candidates[0]
	if c.Scope != ScopeSpan {
		t.Errorf("expected scope span, got %s", c.Scope)
	}
	if c.Confidence < 0.7 {
		t.Errorf("confidence should be high for full outage with topology, got %f", c.Confidence)
	}
}

func TestLocalizeUnknownTopology_PartialOutage(t *testing.T) {
	topo := buildTestTopo()

	_ = buildTestTopo()

	job := DetectionJob{
		DTID:      "D-02",
		DarkPoles: []string{"P-1", "P-2"},
		LitPoles:  []string{"P-3", "P-4"},
	}

	candidates := Localize(job, topo)

	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}

	c := candidates[0]
	if c.Scope != ScopeDT {
		t.Errorf("expected scope dt, got %s", c.Scope)
	}
	if c.Confidence > 0.7 {
		t.Errorf("confidence should be lower without topology, got %f", c.Confidence)
	}
}

func TestLocalizeUnknownTopology_FullOutage(t *testing.T) {
	topo := buildTestTopo()

	job := DetectionJob{
		DTID:      "D-02",
		DarkPoles: []string{"P-1", "P-2", "P-3", "P-4", "P-5", "P-6", "P-7", "P-8"},
		LitPoles:  []string{},
	}

	candidates := Localize(job, topo)

	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}

	c := candidates[0]
	if c.Scope != ScopeDT {
		t.Errorf("expected scope dt, got %s", c.Scope)
	}
}

func TestLocalizeNoDarkPoles(t *testing.T) {
	topo := buildTestTopo()

	job := DetectionJob{
		DTID:      "D-01",
		DarkPoles: []string{},
		LitPoles:  []string{"P-1", "P-2"},
	}

	candidates := Localize(job, topo)

	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates, got %d", len(candidates))
	}
}

func TestComputeConfidence(t *testing.T) {
	tests := []struct {
		name           string
		affected       int
		deviceCount    int
		expectedDark   int
		reportedDark   int
		litCount       int
		hasTopology    bool
		minConfidence  float64
		maxConfidence  float64
	}{
		{
			name:          "high confidence with topology and full reporting",
			affected:      5,
			deviceCount:   6,
			expectedDark:  5,
			reportedDark:  5,
			litCount:      0,
			hasTopology:   true,
			minConfidence: 0.8,
			maxConfidence: 0.99,
		},
		{
			name:          "medium confidence without topology",
			affected:      3,
			deviceCount:   6,
			expectedDark:  3,
			reportedDark:  3,
			litCount:      3,
			hasTopology:   false,
			minConfidence: 0.5,
			maxConfidence: 0.7,
		},
		{
			name:          "low confidence with few devices",
			affected:      1,
			deviceCount:   20,
			expectedDark:  1,
			reportedDark:  1,
			litCount:      19,
			hasTopology:   false,
			minConfidence: 0.1,
			maxConfidence: 0.6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confidence := computeConfidence(
				tt.affected, tt.deviceCount, tt.expectedDark,
				tt.reportedDark, tt.litCount, tt.hasTopology,
			)
			if confidence < tt.minConfidence || confidence > tt.maxConfidence {
				t.Errorf("confidence %f not in range [%f, %f]", confidence, tt.minConfidence, tt.maxConfidence)
			}
		})
	}
}

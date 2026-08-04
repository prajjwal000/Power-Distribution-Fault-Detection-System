package simulator

import (
	"testing"

	"power-fault-detector/internal/model"
)

func TestDescendants(t *testing.T) {
	st := NewSimulatorState()
	st.Children = map[string][]string{
		"P-001": {"P-002", "P-003"},
		"P-002": {"P-004", "P-005"},
		"P-003": {},
		"P-004": {},
		"P-005": {"P-006"},
		"P-006": {},
	}

	desc := st.Descendants("P-002")
	if len(desc) != 3 {
		t.Errorf("P-002 descendants: got %d, want 3", len(desc))
	}

	desc = st.Descendants("P-003")
	if len(desc) != 0 {
		t.Errorf("P-003 descendants: got %d, want 0", len(desc))
	}

	desc = st.Descendants("P-001")
	if len(desc) != 5 {
		t.Errorf("P-001 descendants: got %d, want 5", len(desc))
	}
}

func TestWillEmitPowerLost(t *testing.T) {
	emitted := 0
	for i := 0; i < 1000; i++ {
		d := &DeviceState{Firmware: "1.3.0"}
		if d.WillEmitPowerLost() {
			emitted++
		}
	}
	rate := float64(emitted) / 1000.0
	if rate < 0.60 || rate > 0.80 {
		t.Errorf("emit rate = %.2f; want ~0.70", rate)
	}
}

func TestFW1_2NeverEmits(t *testing.T) {
	for i := 0; i < 100; i++ {
		d := &DeviceState{Firmware: "1.2.0"}
		if d.WillEmitPowerLost() {
			t.Fatal("fw 1.2.x must never emit power_lost")
		}
	}
}

func TestAffectedPolesForSpan(t *testing.T) {
	st := NewSimulatorState()
	st.Children = map[string][]string{
		"P-001": {"P-002", "P-003"},
		"P-002": {"P-004"},
		"P-003": {},
		"P-004": {},
	}

	got := st.AffectedPolesForSpan("P-002")
	if len(got) != 2 {
		t.Errorf("span P-002: got %v (%d poles), want [P-002, P-004]", got, len(got))
	}

	got = st.AffectedPolesForSpan("P-003")
	if len(got) != 1 {
		t.Errorf("span P-003: got %v, want [P-003]", got)
	}
}

func TestAffectedPolesForDT(t *testing.T) {
	st := NewSimulatorState()
	st.PoleByID = map[string]*model.Pole{
		"P-001": {ID: "P-001", DTID: "D-001"},
		"P-002": {ID: "P-002", DTID: "D-001"},
		"P-003": {ID: "P-003", DTID: "D-002"},
	}

	got := st.AffectedPolesForDT("D-001")
	if len(got) != 2 {
		t.Errorf("D-001: got %d poles, want 2", len(got))
	}
}

func TestAffectedPolesForFeeder(t *testing.T) {
	st := NewSimulatorState()
	st.PoleByID = map[string]*model.Pole{
		"P-001": {ID: "P-001", FeederID: "F-001"},
		"P-002": {ID: "P-002", FeederID: "F-001"},
		"P-003": {ID: "P-003", FeederID: "F-002"},
	}

	got := st.AffectedPolesForFeeder("F-001")
	if len(got) != 2 {
		t.Errorf("F-001: got %d poles, want 2", len(got))
	}
}

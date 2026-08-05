package simulator

import (
	"math"
	"testing"

	"power-fault-detector/internal/model"
)

func TestFaultUniqueIDs(t *testing.T) {
	st := buildLineState()
	clock := NewClock(30)
	te := NewTelemetryEngine(st, clock, func(TelemetryEvent) {})

	f1 := te.InjectFault("P-1", "P-2")
	f2 := te.InjectFault("P-1", "P-2") // duplicate — should return nil
	f3 := te.InjectFault("P-2", "P-4")

	if f1 == nil {
		t.Fatal("first fault should inject")
	}
	if f2 != nil {
		t.Error("duplicate fault should return nil")
	}
	if f3 == nil {
		t.Fatal("third fault should inject")
	}
	if f1.ID == f3.ID {
		t.Errorf("fault IDs not unique: %s, %s", f1.ID, f3.ID)
	}
}

func TestInjectFaultValidatesEdge(t *testing.T) {
	st := buildLineState()
	clock := NewClock(30)
	te := NewTelemetryEngine(st, clock, func(TelemetryEvent) {})

	// Non-existent edge
	f := te.InjectFault("P-99", "P-1")
	if f != nil {
		t.Errorf("injecting non-existent edge should return nil, got %+v", f)
	}
	// Wrong direction (P-2 is not parent of P-1)
	f = te.InjectFault("P-2", "P-1")
	if f != nil {
		t.Errorf("injecting wrong direction should return nil, got %+v", f)
	}
}

func TestOverlappingFaultsRepairDoesNotReenergize(t *testing.T) {
	st := buildLineState() // P-1 -> P-2 -> P-4
	clock := NewClock(30)
	te := NewTelemetryEngine(st, clock, func(TelemetryEvent) {})

	// Fault 1: span P-1->P-2 (affects P-2, P-4)
	f1 := te.InjectFault("P-1", "P-2")
	if f1 == nil {
		t.Fatal("first fault injection failed")
	}
	// Fault 2: span P-2->P-4 (affects P-4)
	f2 := te.InjectFault("P-2", "P-4")
	if f2 == nil {
		t.Fatal("second fault injection failed")
	}

	// Repair f1 (outer fault) - P-2 and P-4 should not re-energize
	// because P-4 is still covered by f2, and P-2 is covered by f1 itself
	// Wait: f1 covers P-2 and P-4. f2 covers P-4. When we repair f1,
	// we delete f1 and call repairDevices([P-2, P-4]).
	// stillDark is computed from remaining active faults (f2) -> {P-4: true}.
	// So P-4 should stay dark, P-2 should re-energize.
	if err := te.RepairFault(f1.ID); err != nil {
		t.Fatalf("repair f1: %v", err)
	}

	if !st.Devices["D-2"].Energized {
		t.Error("D-2 should re-energize after outer fault repair (not covered by f2)")
	}
	if st.Devices["D-4"].Energized {
		t.Error("D-4 should stay dark after outer fault repair (still covered by f2)")
	}

	// Now repair f2 - P-4 should re-energize
	if err := te.RepairFault(f2.ID); err != nil {
		t.Fatalf("repair f2: %v", err)
	}
	if !st.Devices["D-4"].Energized {
		t.Error("D-4 should re-energize after inner fault repair")
	}
}

func TestRepairAllClearsAll(t *testing.T) {
	st := buildLineState()
	clock := NewClock(30)
	te := NewTelemetryEngine(st, clock, func(TelemetryEvent) {})

	te.InjectFault("P-1", "P-2")
	te.InjectFault("P-2", "P-4")

	te.RepairAll()

	if len(te.st.ActiveFaults) != 0 {
		t.Errorf("ActiveFaults = %d, want 0", len(te.st.ActiveFaults))
	}
	for _, dev := range st.Devices {
		if !dev.Energized {
			t.Errorf("device %s should be energized after RepairAll", dev.DeviceID)
		}
	}
}

func TestRepairFaultNotFoundReturnsError(t *testing.T) {
	st := buildLineState()
	clock := NewClock(30)
	te := NewTelemetryEngine(st, clock, func(TelemetryEvent) {})

	err := te.RepairFault("fault-nonexistent")
	if err == nil {
		t.Error("repairing unknown fault should return error")
	}
}

// Test that InjectFault doesn't emit power_lost for already-dark devices
func TestInjectFaultOnAlreadyDarkDoesNotReemit(t *testing.T) {
	old := powerLostDeliveryRate
	powerLostDeliveryRate = 1.0
	defer func() { powerLostDeliveryRate = old }()

	st := buildLineState()
	clock := NewClock(30)
	te := NewTelemetryEngine(st, clock, func(TelemetryEvent) {})

	// Inject fault P-1->P-2 (darkens P-2, P-4)
	te.InjectFault("P-1", "P-2")
	// Queue has 2 events: D-2 (reported=true) and D-4 (reported=false, fw 1.2)

	// Inject another fault on P-2->P-4 (inner, affects P-4 again)
	// D-4 is already dark, so no new power_lost should be queued for it
	te.InjectFault("P-2", "P-4")

	// Queue still has 2 events (D-2 and D-4 from first fault only)
	if got := te.queue.Len(); got != 2 {
		t.Errorf("queue len = %d, want 2", got)
	}
}

func TestInjectDT(t *testing.T) {
	old := powerLostDeliveryRate
	powerLostDeliveryRate = 1.0
	defer func() { powerLostDeliveryRate = old }()

	st := buildLineState()
	// Add a transformer ID to the state
	st.TransformerByID["D-123"] = &model.Transformer{ID: "D-123", FeederID: "F-1"}
	// Add some poles under this DT
	d5 := "D-5"
	st.PoleByID["P-5"] = &model.Pole{ID: "P-5", DTID: "D-123", DeviceID: &d5}
	st.Devices["D-5"] = &DeviceState{DeviceID: "D-5", PoleID: "P-5", Firmware: "1.4.2", Energized: true, RadioDelaySecs: 5}

	clock := NewClock(30)
	te := NewTelemetryEngine(st, clock, func(TelemetryEvent) {})

	// Inject DT fault
	fault := te.InjectDT("D-123")
	if fault == nil {
		t.Fatal("InjectDT returned nil")
	}
	if fault.Type != "dt" || fault.Target != "D-123" {
		t.Errorf("fault = %+v, want type=dt target=D-123", fault)
	}
	if fault.Affected != 1 {
		t.Errorf("affected count = %d, want 1", fault.Affected)
	}
	// Queue should have 1 event (D-5 emits with reported=true)
	if got := te.queue.Len(); got != 1 {
		t.Errorf("queue len = %d, want 1", got)
	}
	due := te.queue.Due(math.MaxInt64)
	if len(due) != 1 || due[0].PoleID != "P-5" {
		t.Errorf("due event = %+v, want P-5 power_lost", due)
	}
	if !due[0].Reported {
		t.Errorf("D-5 should have reported=true (fw 1.4.2)")
	}
}

func TestInjectFeeder(t *testing.T) {
	old := powerLostDeliveryRate
	powerLostDeliveryRate = 1.0
	defer func() { powerLostDeliveryRate = old }()

	st := buildLineState()
	// Add a feeder ID
	st.FeederByID["F-99"] = &model.Feeder{ID: "F-99"}
	// Add poles under this feeder
	d6 := "D-6"
	st.PoleByID["P-6"] = &model.Pole{ID: "P-6", DTID: "D-123", FeederID: "F-99", DeviceID: &d6}
	st.Devices["D-6"] = &DeviceState{DeviceID: "D-6", PoleID: "P-6", Firmware: "1.4.2", Energized: true, RadioDelaySecs: 3}

	clock := NewClock(30)
	te := NewTelemetryEngine(st, clock, func(TelemetryEvent) {})

	fault := te.InjectFeeder("F-99")
	if fault == nil {
		t.Fatal("InjectFeeder returned nil")
	}
	if fault.Type != "feeder" || fault.Target != "F-99" {
		t.Errorf("fault = %+v, want type=feeder target=F-99", fault)
	}
	if fault.Affected != 1 {
		t.Errorf("affected count = %d, want 1", fault.Affected)
	}
	if got := te.queue.Len(); got != 1 {
		t.Errorf("queue len = %d, want 1", got)
	}
	due := te.queue.Due(math.MaxInt64)
	if len(due) != 1 || due[0].PoleID != "P-6" {
		t.Errorf("due event = %+v, want P-6 power_lost", due)
	}
}

func TestInjectDTUnknownReturnsNil(t *testing.T) {
	st := buildLineState()
	clock := NewClock(30)
	te := NewTelemetryEngine(st, clock, func(TelemetryEvent) {})

	fault := te.InjectDT("unknown-dt")
	if fault != nil {
		t.Errorf("InjectDT unknown should return nil, got %+v", fault)
	}
}

func TestInjectFeederUnknownReturnsNil(t *testing.T) {
	st := buildLineState()
	clock := NewClock(30)
	te := NewTelemetryEngine(st, clock, func(TelemetryEvent) {})

	fault := te.InjectFeeder("unknown-feeder")
	if fault != nil {
		t.Errorf("InjectFeeder unknown should return nil, got %+v", fault)
	}
}
package simulator

import (
	"testing"
)

func TestFaultUniqueIDs(t *testing.T) {
	st := buildLineState()
	clock := NewClock(30)
	te := NewTelemetryEngine(st, clock, func(TelemetryEvent) {})

	f1 := te.InjectFault("P-1", "P-2")
	f2 := te.InjectFault("P-1", "P-2")
	f3 := te.InjectFault("P-2", "P-4")

	if f1 == nil || f2 == nil || f3 == nil {
		t.Fatal("all faults should inject")
	}
	ids := []string{f1.ID, f2.ID, f3.ID}
	for i, id := range ids {
		if id == "" {
			t.Errorf("fault %d has empty ID", i)
		}
	}
	if f1.ID == f2.ID || f1.ID == f3.ID || f2.ID == f3.ID {
		t.Errorf("fault IDs not unique: %v", ids)
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
	// D-2 and D-4 are dark, NextEmitSim=0
	// Queue should have 1 event (D-2 emits)

	// Inject another fault on P-2->P-4 (inner, affects P-4 again)
	// D-4 is already dark, so no new power_lost should be queued for it
	te.InjectFault("P-2", "P-4")

	// Total queued events should still be 1 (only D-2's first power_lost)
	// Wait - D-2 is also dark from the first fault, so InjectFault("P-2","P-4")
	// won't emit for D-2 (it's not in the affected set - affected is P-4)
	// and D-4 is dark and fw 1.2 silent. So queue len should be 1.
	// Actually, P-4 is affected, D-4 is fw 1.2 -> no emit. So queue stays at 1.
	if got := te.queue.Len(); got != 1 {
		t.Errorf("queue len = %d, want 1", got)
	}
}
package ingestor

import (
	"testing"
)

func TestCheckDedup_NewDevice(t *testing.T) {
	ev := TelemetryEvent{
		DeviceID: "DEV-1",
		Event:    "heartbeat",
		Seq:      1,
	}

	result := CheckDedup(ev, nil)

	if !result.ShouldProcess {
		t.Error("expected ShouldProcess=true for new device")
	}
	if result.IsDuplicate {
		t.Error("expected IsDuplicate=false for new device")
	}
}

func TestCheckDedup_SameSeqSameEvent(t *testing.T) {
	state := NewDeviceState("DEV-1", "P-1", "D-01", "F-01")
	state.LastSeq = 5
	state.LastEvent = "heartbeat"

	ev := TelemetryEvent{
		DeviceID: "DEV-1",
		Event:    "heartbeat",
		Seq:      5,
	}

	result := CheckDedup(ev, state)

	if result.ShouldProcess {
		t.Error("expected ShouldProcess=false for same seq and event")
	}
	if !result.IsDuplicate {
		t.Error("expected IsDuplicate=true for same seq and event")
	}
}

func TestCheckDedup_SameSeqDifferentEvent(t *testing.T) {
	state := NewDeviceState("DEV-1", "P-1", "D-01", "F-01")
	state.LastSeq = 5
	state.LastEvent = "heartbeat"

	ev := TelemetryEvent{
		DeviceID: "DEV-1",
		Event:    "power_lost",
		Seq:      5,
	}

	result := CheckDedup(ev, state)

	if !result.ShouldProcess {
		t.Error("expected ShouldProcess=true for same seq but different event")
	}
}

func TestCheckDedup_HigherSeq(t *testing.T) {
	state := NewDeviceState("DEV-1", "P-1", "D-01", "F-01")
	state.LastSeq = 5
	state.LastEvent = "heartbeat"

	ev := TelemetryEvent{
		DeviceID: "DEV-1",
		Event:    "heartbeat",
		Seq:      6,
	}

	result := CheckDedup(ev, state)

	if !result.ShouldProcess {
		t.Error("expected ShouldProcess=true for higher seq")
	}
	if result.IsDuplicate {
		t.Error("expected IsDuplicate=false for higher seq")
	}
}

func TestCheckDedup_LowerSeq(t *testing.T) {
	state := NewDeviceState("DEV-1", "P-1", "D-01", "F-01")
	state.LastSeq = 5
	state.LastEvent = "heartbeat"

	ev := TelemetryEvent{
		DeviceID: "DEV-1",
		Event:    "heartbeat",
		Seq:      3,
	}

	result := CheckDedup(ev, state)

	if result.ShouldProcess {
		t.Error("expected ShouldProcess=false for lower seq")
	}
	if !result.IsStale {
		t.Error("expected IsStale=true for lower seq")
	}
}

func TestDeviceState_RecordEvent(t *testing.T) {
	state := NewDeviceState("DEV-1", "P-1", "D-01", "F-01")

	ev := EventRecord{
		Event:     "heartbeat",
		Energized: true,
		Seq:       1,
	}

	state.RecordEvent(ev)

	if state.LastSeq != 1 {
		t.Errorf("expected LastSeq=1, got %d", state.LastSeq)
	}
	if state.LastEvent != "heartbeat" {
		t.Errorf("expected LastEvent=heartbeat, got %s", state.LastEvent)
	}
	if !state.Energized {
		t.Error("expected Energized=true")
	}
	if len(state.EventHistory) != 1 {
		t.Errorf("expected 1 event in history, got %d", len(state.EventHistory))
	}
}

func TestDeviceState_DarkSince(t *testing.T) {
	state := NewDeviceState("DEV-1", "P-1", "D-01", "F-01")

	if state.IsDark() {
		t.Error("expected not dark initially")
	}

	state.RecordDarkSince(100)
	if !state.IsDark() {
		t.Error("expected dark after RecordDarkSince")
	}

	state.ClearDarkSince()
	if state.IsDark() {
		t.Error("expected not dark after ClearDarkSince")
	}
}

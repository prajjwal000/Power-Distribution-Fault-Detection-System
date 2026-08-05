package simulator

import (
	"fmt"
	"testing"
)

func TestInitializeNextEmitTimesWarmupSpread(t *testing.T) {
	st := NewSimulatorState()
	for i := 0; i < 100; i++ {
		id := fmt.Sprintf("D-%03d", i)
		st.Devices[id] = &DeviceState{DeviceID: id, PoleID: "P-" + id, Energized: true}
	}
	st.Devices["D-dark"] = &DeviceState{DeviceID: "D-dark", PoleID: "P-dark", Energized: false}

	clock := NewClock(30)
	te := NewTelemetryEngine(st, clock, func(TelemetryEvent) {})
	te.initializeNextEmitTimes()

	now := clock.NowSim()
	distinct := make(map[int64]struct{})
	for id, dev := range st.Devices {
		if !dev.Energized {
			if dev.NextEmitSim != 0 {
				t.Errorf("dark device %s scheduled at %d, want 0", id, dev.NextEmitSim)
			}
			continue
		}
		if dev.NextEmitSim < now || dev.NextEmitSim > now+WarmupWindowSecs {
			t.Errorf("device %s first emit at %d, outside warm-up window [%d, %d]",
				id, dev.NextEmitSim, now, now+WarmupWindowSecs)
		}
		distinct[dev.NextEmitSim] = struct{}{}
	}

	if len(distinct) < 2 {
		t.Errorf("warm-up emitted %d distinct times, want spread across the window", len(distinct))
	}
}

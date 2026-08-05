package simulator

import (
	"testing"
	"time"

	"power-fault-detector/internal/model"
)

func strp(s string) *string { return &s }

// buildLineState builds P-1 -> P-2 -> P-4 with devices on every pole:
// D-1 and D-2 on firmware that sends power_lost, D-4 on silent fw 1.2.
func buildLineState() *SimulatorState {
	st := NewSimulatorState()
	st.Children["P-1"] = []string{"P-2"}
	st.Children["P-2"] = []string{"P-4"}
	st.Children["P-4"] = []string{}
	st.Parents["P-2"] = "P-1"
	st.Parents["P-4"] = "P-2"
	st.TransformerByID = make(map[string]*model.Transformer)
	st.FeederByID = make(map[string]*model.Feeder)

	st.PoleByID["P-1"] = &model.Pole{ID: "P-1", DeviceID: strp("D-1")}
	st.PoleByID["P-2"] = &model.Pole{ID: "P-2", DeviceID: strp("D-2")}
	st.PoleByID["P-4"] = &model.Pole{ID: "P-4", DeviceID: strp("D-4")}

	st.Devices["D-1"] = &DeviceState{DeviceID: "D-1", PoleID: "P-1", Firmware: "1.4.2", Energized: true, RadioDelaySecs: 2}
	st.Devices["D-2"] = &DeviceState{DeviceID: "D-2", PoleID: "P-2", Firmware: "1.4.2", Energized: true, RadioDelaySecs: 10, ClockSkewSecs: 5}
	st.Devices["D-4"] = &DeviceState{DeviceID: "D-4", PoleID: "P-4", Firmware: "1.2.0", Energized: true}
	return st
}

func TestDeliveryQueueDueOrder(t *testing.T) {
	q := NewDeliveryQueue()
	q.Schedule(30, TelemetryEvent{PoleID: "C"})
	q.Schedule(10, TelemetryEvent{PoleID: "A"})
	q.Schedule(20, TelemetryEvent{PoleID: "B"})

	if got := q.Len(); got != 3 {
		t.Fatalf("Len() = %d, want 3", got)
	}
	if due := q.Due(15); len(due) != 1 || due[0].PoleID != "A" {
		t.Fatalf("Due(15) = %+v, want only A", due)
	}
	due := q.Due(100)
	if len(due) != 2 || due[0].PoleID != "B" || due[1].PoleID != "C" {
		t.Fatalf("Due(100) = %+v, want [B, C] in sim order", due)
	}
	if got := q.Len(); got != 0 {
		t.Fatalf("Len() after drain = %d, want 0", got)
	}
}

func TestInjectFaultSchedulesPowerLostWithRadioDelay(t *testing.T) {
	old := powerLostDeliveryRate
	powerLostDeliveryRate = 1.0
	defer func() { powerLostDeliveryRate = old }()

	st := buildLineState()
	clock := NewClock(30)
	te := NewTelemetryEngine(st, clock, func(TelemetryEvent) {})

	fault := te.InjectFault("P-1", "P-2")
	if len(fault.AffectedSet) != 2 {
		t.Fatalf("affected = %v, want [P-2, P-4]", fault.AffectedSet)
	}

	if got := te.queue.Len(); got != 1 {
		t.Fatalf("queued %d events, want 1 (D-2 emits, D-4 silent on fw 1.2)", got)
	}
	if due := te.queue.Due(fault.StartSim + 9); len(due) != 0 {
		t.Fatalf("power_lost due before radio delay elapsed: %+v", due)
	}
	due := te.queue.Due(fault.StartSim + 10)
	if len(due) != 1 {
		t.Fatalf("due at radio delay = %+v, want exactly 1 event", due)
	}
	evt := due[0]
	if evt.Event != "power_lost" || evt.PoleID != "P-2" || evt.Energized || evt.Seq != 1 {
		t.Errorf("unexpected event: %+v", evt)
	}

	ts, err := time.Parse("2006-01-02T15:04:05.000Z", evt.Ts)
	if err != nil {
		t.Fatalf("unparseable ts %q: %v", evt.Ts, err)
	}
	if ts.Before(time.Now().Add(-time.Minute)) {
		t.Errorf("ts %v is before simulator boot", ts)
	}

	if st.Devices["D-4"].Energized {
		t.Error("fw 1.2 device must go dark even though it stays silent")
	}
	if !st.Devices["D-1"].Energized {
		t.Error("upstream device must stay energized")
	}
}

func TestTickSchedulesHeartbeatWithRadioDelay(t *testing.T) {
	st := NewSimulatorState()
	dev := &DeviceState{DeviceID: "D-1", PoleID: "P-1", Firmware: "1.4.2", Energized: true, RadioDelaySecs: 7}
	st.Devices["D-1"] = dev
	st.PoleByID["P-1"] = &model.Pole{ID: "P-1", DeviceID: strp("D-1")}

	clock := NewClock(1)
	clock.SetSimTime(1000)
	te := NewTelemetryEngine(st, clock, func(TelemetryEvent) {})

	dev.NextEmitSim = 999
	te.tick()

	if dev.Seq != 1 {
		t.Fatalf("seq = %d, want 1", dev.Seq)
	}
	if due := te.queue.Due(1005); len(due) != 0 {
		t.Fatalf("heartbeat due before radio delay: %+v", due)
	}
	due := te.queue.Due(1006)
	if len(due) != 1 || due[0].Event != "heartbeat" {
		t.Fatalf("due at radio delay = %+v, want 1 heartbeat", due)
	}
	if dev.NextEmitSim <= 1000 {
		t.Errorf("next emit %d not rescheduled past now", dev.NextEmitSim)
	}
}

func TestDispatchDeliversDueEvents(t *testing.T) {
	st := NewSimulatorState()
	clock := NewClock(30)
	received := make(chan TelemetryEvent, 8)
	te := NewTelemetryEngine(st, clock, func(e TelemetryEvent) { received <- e })

	want := TelemetryEvent{DeviceID: "D-1", PoleID: "P-1", Event: "heartbeat", Energized: true, Seq: 1}
	te.queue.Schedule(clock.NowSim(), want)
	te.Start()
	defer te.Stop()

	select {
	case got := <-received:
		if got != want {
			t.Errorf("delivered %+v, want %+v", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("due event was not delivered within 2s")
	}
}

func TestPauseHoldsDelivery(t *testing.T) {
	st := NewSimulatorState()
	clock := NewClock(30)
	received := make(chan TelemetryEvent, 8)
	te := NewTelemetryEngine(st, clock, func(e TelemetryEvent) { received <- e })

	clock.Pause()
	te.queue.Schedule(clock.NowSim(), TelemetryEvent{DeviceID: "D-1", PoleID: "P-1", Event: "heartbeat"})
	te.Start()
	defer te.Stop()

	select {
	case <-received:
		t.Fatal("event delivered while clock paused")
	case <-time.After(300 * time.Millisecond):
	}

	clock.Resume()
	select {
	case <-received:
	case <-time.After(2 * time.Second):
		t.Fatal("event not delivered after resume")
	}
}

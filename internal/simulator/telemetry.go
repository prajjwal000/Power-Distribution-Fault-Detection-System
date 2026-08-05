package simulator

import (
	"container/heap"
	"math/rand"
	"sync"
	"time"
)

type TelemetryEmitter func(event TelemetryEvent)

const HeartbeatIntervalSecs = 900
const HeartbeatJitterSecs = 45

// WarmupWindowSecs bounds when each device sends its first heartbeat after
// simulator boot. Spreading first check-ins across this window establishes
// the energized baseline at the API quickly without a startup burst, and
// without fabricating boot/power_restored events for devices that never
// actually lost power.
const WarmupWindowSecs = 120

type deviceHeap []*DeviceState

func (h deviceHeap) Len() int           { return len(h) }
func (h deviceHeap) Less(i, j int) bool { return h[i].NextEmitSim < h[j].NextEmitSim }
func (h deviceHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *deviceHeap) Push(x any)        { *h = append(*h, x.(*DeviceState)) }
func (h *deviceHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[0 : n-1]
	return item
}

type TelemetryEngine struct {
	st    *SimulatorState
	clock *Clock
	emit  TelemetryEmitter
	mu    sync.Mutex
}

func NewTelemetryEngine(st *SimulatorState, clock *Clock, emit TelemetryEmitter) *TelemetryEngine {
	return &TelemetryEngine{st: st, clock: clock, emit: emit}
}

func (te *TelemetryEngine) Start() {
	te.mu.Lock()
	te.initializeNextEmitTimes()
	te.mu.Unlock()
	go te.run()
}

func (te *TelemetryEngine) Stop() {
	te.mu.Lock()
	defer te.mu.Unlock()
}

func (te *TelemetryEngine) initializeNextEmitTimes() {
	now := te.clock.NowSim()
	for _, dev := range te.st.Devices {
		if !dev.Energized {
			continue
		}
		dev.NextEmitSim = now + int64(rand.Float64()*WarmupWindowSecs)
	}
}

func (te *TelemetryEngine) run() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		te.tick()
	}
}

func (te *TelemetryEngine) tick() {
	simNow := te.clock.NowSim()

	te.mu.Lock()
	defer te.mu.Unlock()

	h := make(deviceHeap, 0)
	for _, dev := range te.st.Devices {
		if dev.Energized && dev.NextEmitSim > 0 {
			heap.Push(&h, dev)
		}
	}

	for h.Len() > 0 && h[0].NextEmitSim <= simNow {
		dev := heap.Pop(&h).(*DeviceState)
		if !dev.Energized {
			continue
		}
		if dev.NextEmitSim > simNow {
			heap.Push(&h, dev)
			break
		}
		te.emitEventLocked(dev, simNow)

		jitter := int64(HeartbeatJitterSecs * 2 * rand.Float64())
		dev.NextEmitSim = simNow + int64(HeartbeatIntervalSecs) - int64(HeartbeatJitterSecs) + jitter
		heap.Push(&h, dev)
	}
}

func (te *TelemetryEngine) emitEventLocked(dev *DeviceState, simNow int64) {
	dev.Seq++
	evSim := dev.NextEmitSim
	ts := te.clock.TsForSim(evSim, dev.ClockSkewSecs)
	evt := TelemetryEvent{
		DeviceID:  dev.DeviceID,
		PoleID:    dev.PoleID,
		Event:     "heartbeat",
		Energized: true,
		Ts:        ts,
		Seq:       dev.Seq,
		BatteryMV: dev.BatteryMV,
		RSSI:      dev.RSSI,
		Fw:        dev.Firmware,
	}
	te.emit(evt)
}

func (te *TelemetryEngine) InjectFault(parentID, childID string) *Fault {
	te.mu.Lock()
	defer te.mu.Unlock()

	simNow := te.clock.NowSim()
	affected := te.st.AffectedPolesForSpan(childID)

	fault := &Fault{
		ID:          "fault-" + childID,
		Span:        parentID + "->" + childID,
		AffectedSet: affected,
		StartSim:    simNow,
	}
	te.st.ActiveFaults[fault.ID] = fault

	for _, poleID := range affected {
		pole := te.st.PoleByID[poleID]
		if pole == nil || pole.DeviceID == nil {
			continue
		}
		dev, ok := te.st.Devices[*pole.DeviceID]
		if !ok {
			continue
		}
		dev.Energized = false
		dev.NextEmitSim = 0

		if !dev.WillEmitPowerLost() {
			continue
		}

		evSim := simNow + dev.RadioDelaySecs
		dev.NextEmitSim = evSim
		dev.Seq++
		evt := TelemetryEvent{
			DeviceID:  dev.DeviceID,
			PoleID:    dev.PoleID,
			Event:     "power_lost",
			Energized: false,
			Ts:        te.clock.TsForSim(evSim, dev.ClockSkewSecs),
			Seq:       dev.Seq,
			BatteryMV: dev.BatteryMV,
			RSSI:      dev.RSSI,
			Fw:        dev.Firmware,
		}
		te.emit(evt)
	}

	return fault
}

func (te *TelemetryEngine) ListFaults() []*Fault {
	te.mu.Lock()
	defer te.mu.Unlock()
	faults := make([]*Fault, 0, len(te.st.ActiveFaults))
	for _, f := range te.st.ActiveFaults {
		faults = append(faults, f)
	}
	return faults
}

func (te *TelemetryEngine) RepairFault(faultID string) error {
	te.mu.Lock()
	defer te.mu.Unlock()

	fault, ok := te.st.ActiveFaults[faultID]
	if !ok {
		return nil
	}
	te.repairDevices(fault.AffectedSet)
	delete(te.st.ActiveFaults, faultID)
	return nil
}

func (te *TelemetryEngine) RepairAll() {
	te.mu.Lock()
	defer te.mu.Unlock()

	allPoles := make([]string, 0, len(te.st.ActiveFaults))
	for _, f := range te.st.ActiveFaults {
		allPoles = append(allPoles, f.AffectedSet...)
	}
	te.st.ActiveFaults = make(map[string]*Fault)
	te.repairDevices(allPoles)
}

func (te *TelemetryEngine) repairDevices(poleIDs []string) {
	simNow := te.clock.NowSim()

	for _, poleID := range poleIDs {
		pole := te.st.PoleByID[poleID]
		if pole == nil || pole.DeviceID == nil {
			continue
		}
		dev, ok := te.st.Devices[*pole.DeviceID]
		if !ok {
			continue
		}
		if dev.Energized {
			continue
		}

		dev.Seq = 0
		dev.Seq++
		te.emit(TelemetryEvent{
			DeviceID:  dev.DeviceID,
			PoleID:    dev.PoleID,
			Event:     "boot",
			Energized: false,
			Ts:        te.clock.TsForSim(simNow, dev.ClockSkewSecs),
			Seq:       dev.Seq,
			BatteryMV: dev.BatteryMV,
			RSSI:      dev.RSSI,
			Fw:        dev.Firmware,
		})

		dev.Seq++
		restoreSim := simNow + dev.RadioDelaySecs + 1
		te.emit(TelemetryEvent{
			DeviceID:  dev.DeviceID,
			PoleID:    dev.PoleID,
			Event:     "power_restored",
			Energized: true,
			Ts:        te.clock.TsForSim(restoreSim, dev.ClockSkewSecs),
			Seq:       dev.Seq,
			BatteryMV: dev.BatteryMV,
			RSSI:      dev.RSSI,
			Fw:        dev.Firmware,
		})

		dev.Energized = true
		jitter := int64(HeartbeatJitterSecs * 2 * rand.Float64())
		dev.NextEmitSim = simNow + int64(HeartbeatIntervalSecs) - int64(HeartbeatJitterSecs) + jitter
	}
}



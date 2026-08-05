package simulator

import (
	"container/heap"
	"fmt"
	"math/rand"
	"strconv"
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

const (
	deliveryWorkers  = 8
	dispatchInterval = 50 * time.Millisecond
	schedulerTick    = 100 * time.Millisecond
)

// powerLostDeliveryRate is the fraction of dying-message attempts that make
// it through (capacitor reserve). A var, not a const, so tests can pin it.
var powerLostDeliveryRate = 0.70

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
	st         *SimulatorState
	clock      *Clock
	emit       TelemetryEmitter
	queue      *DeliveryQueue
	rng        *rand.Rand // guarded by mu

	jobs       chan TelemetryEvent
	stopCh     chan struct{}
	faultSeq   int         // guarded by mu

	schedWg    sync.WaitGroup // run + dispatch
	workerWg   sync.WaitGroup // delivery workers

	mu sync.Mutex
}

func NewTelemetryEngine(st *SimulatorState, clock *Clock, emit TelemetryEmitter) *TelemetryEngine {
	return &TelemetryEngine{
		st:     st,
		clock:  clock,
		emit:   emit,
		queue:  NewDeliveryQueue(),
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
		jobs:   make(chan TelemetryEvent, 4096),
		stopCh: make(chan struct{}),
	}
}

func (te *TelemetryEngine) Start() {
	te.mu.Lock()
	te.initializeNextEmitTimes()
	te.mu.Unlock()

	te.schedWg.Add(2)
	go te.run()
	go te.dispatch()

	te.workerWg.Add(deliveryWorkers)
	for i := 0; i < deliveryWorkers; i++ {
		go te.worker()
	}
}

func (te *TelemetryEngine) Stop() {
	close(te.stopCh)
	te.schedWg.Wait()
	close(te.jobs)
	te.workerWg.Wait()
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

// run fires due heartbeats on the scheduler tick.
func (te *TelemetryEngine) run() {
	defer te.schedWg.Done()
	ticker := time.NewTicker(schedulerTick)
	defer ticker.Stop()

	for {
		select {
		case <-te.stopCh:
			return
		case <-ticker.C:
			te.tick()
		}
	}
}

// dispatch moves due events from the delivery queue to the worker pool.
// Delivery is keyed on sim time, so a paused clock delivers nothing.
func (te *TelemetryEngine) dispatch() {
	defer te.schedWg.Done()
	ticker := time.NewTicker(dispatchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-te.stopCh:
			return
		case <-ticker.C:
			if te.clock.IsPaused() {
				continue
			}
			for _, evt := range te.queue.Due(te.clock.NowSim()) {
				select {
				case te.jobs <- evt:
				case <-te.stopCh:
					return
				}
			}
		}
	}
}

// worker delivers events to the fan-out emitter (API ingest + SSE broadcast).
// Delivery happens here, off the engine lock, so a slow API can only slow
// delivery — never heartbeat scheduling or fault injection.
func (te *TelemetryEngine) worker() {
	defer te.workerWg.Done()
	for evt := range te.jobs {
		te.emit(evt)
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
		te.scheduleHeartbeatLocked(dev)

		jitter := int64(HeartbeatJitterSecs * 2 * rand.Float64())
		dev.NextEmitSim = simNow + int64(HeartbeatIntervalSecs) - int64(HeartbeatJitterSecs) + jitter
		heap.Push(&h, dev)
	}
}

func (te *TelemetryEngine) scheduleHeartbeatLocked(dev *DeviceState) {
	dev.Seq++
	evSim := dev.NextEmitSim
	evt := TelemetryEvent{
		DeviceID:  dev.DeviceID,
		PoleID:    dev.PoleID,
		Event:     "heartbeat",
		Energized: true,
		Ts:        te.clock.TsForSim(evSim, dev.ClockSkewSecs),
		Seq:       dev.Seq,
		BatteryMV: dev.BatteryMV,
		RSSI:      dev.RSSI,
		Fw:        dev.Firmware,
	}
	te.queue.Schedule(evSim+dev.RadioDelaySecs, evt)
}

// willEmitPowerLost decides whether a device's dying power_lost attempt
// succeeds. Callers must hold mu (guards rng).
func (te *TelemetryEngine) willEmitPowerLost(dev *DeviceState) bool {
	if !dev.FirmwareSendsPowerLost() {
		return false
	}
	return te.rng.Float64() < powerLostDeliveryRate
}

// injectFaultCommon does the shared work of darkening affected poles and
// queuing power_lost events. Caller must hold mu.
func (te *TelemetryEngine) injectFaultCommon(faultType, target string, affected []string) *Fault {
	simNow := te.clock.NowSim()

	te.faultSeq++
	fault := &Fault{
		ID:          "fault-" + strconv.FormatInt(int64(te.faultSeq), 10),
		Type:        faultType,
		Target:      target,
		AffectedSet: affected,
		Affected:    len(affected),
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
		if !dev.Energized {
			continue
		}
		dev.Energized = false
		dev.NextEmitSim = 0

		if !te.willEmitPowerLost(dev) {
			continue
		}

		dev.Seq++
		evt := TelemetryEvent{
			DeviceID:  dev.DeviceID,
			PoleID:    dev.PoleID,
			Event:     "power_lost",
			Energized: false,
			Ts:        te.clock.TsForSim(simNow, dev.ClockSkewSecs),
			Seq:       dev.Seq,
			BatteryMV: dev.BatteryMV,
			RSSI:      dev.RSSI,
			Fw:        dev.Firmware,
		}
		te.queue.Schedule(simNow+dev.RadioDelaySecs, evt)
	}

	return fault
}

func (te *TelemetryEngine) InjectFault(parentID, childID string) *Fault {
	te.mu.Lock()
	defer te.mu.Unlock()

	if te.st.Parents[childID] != parentID {
		return nil
	}
	affected := te.st.AffectedPolesForSpan(childID)
	return te.injectFaultCommon("span", parentID+"->"+childID, affected)
}

func (te *TelemetryEngine) InjectDT(dtID string) *Fault {
	te.mu.Lock()
	defer te.mu.Unlock()

	// Validate DT exists
	if _, ok := te.st.TransformerByID[dtID]; !ok {
		return nil
	}
	affected := te.st.AffectedPolesForDT(dtID)
	return te.injectFaultCommon("dt", dtID, affected)
}

func (te *TelemetryEngine) InjectFeeder(feederID string) *Fault {
	te.mu.Lock()
	defer te.mu.Unlock()

	// Validate feeder exists
	if _, ok := te.st.FeederByID[feederID]; !ok {
		return nil
	}
	affected := te.st.AffectedPolesForFeeder(feederID)
	return te.injectFaultCommon("feeder", feederID, affected)
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
		return fmt.Errorf("fault %s not found", faultID)
	}
	te.repairDevices(fault.AffectedSet, faultID)
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
	te.repairDevices(allPoles, "")
}

func (te *TelemetryEngine) repairDevices(poleIDs []string, excludeFaultID string) {
	simNow := te.clock.NowSim()

	// Determine which poles are still covered by other active faults.
	stillDark := make(map[string]bool)
	for _, f := range te.st.ActiveFaults {
		if f.ID == excludeFaultID {
			continue
		}
		for _, p := range f.AffectedSet {
			stillDark[p] = true
		}
	}

	for _, poleID := range poleIDs {
		if stillDark[poleID] {
			continue // still dark due to another fault
		}
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
		te.queue.Schedule(simNow+dev.RadioDelaySecs, TelemetryEvent{
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
		// Per spec: "typically within 20 seconds" — stagger restoration across devices
		stagger := int64(rand.Float64() * 20)
		restoreSim := simNow + 1 + stagger
		te.queue.Schedule(restoreSim+dev.RadioDelaySecs, TelemetryEvent{
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

// InjectDeviceDeath makes `count` devices stop heartbeating (simulates a dead modem
// while power is still on). If deviceID is specified, targets that device;
// otherwise picks random energized devices. The device stays energized but its
// NextEmitSim is set to 0 so it won't send heartbeats.
func (te *TelemetryEngine) InjectDeviceDeath(deviceID string, count int) error {
	te.mu.Lock()
	defer te.mu.Unlock()

	var targets []*DeviceState
	if deviceID != "" {
		if dev, ok := te.st.Devices[deviceID]; ok && dev.Energized {
			targets = append(targets, dev)
		}
	} else {
		for _, dev := range te.st.Devices {
			if dev.Energized && dev.NextEmitSim > 0 {
				targets = append(targets, dev)
			}
		}
	}

	if len(targets) == 0 {
		return fmt.Errorf("no energized devices available")
	}

	for i := 0; i < count && i < len(targets); i++ {
		dev := targets[i]
		dev.NextEmitSim = 0 // stop heartbeating
	}
	return nil
}

// InjectDuplicateEvent re-emits a recent event from the specified device (or a
// random one) with the same seq, simulating at-least-once delivery.
func (te *TelemetryEngine) InjectDuplicateEvent(deviceID string, count int) error {
	te.mu.Lock()
	defer te.mu.Unlock()

	var dev *DeviceState
	if deviceID != "" {
		var ok bool
		dev, ok = te.st.Devices[deviceID]
		if !ok {
			return fmt.Errorf("device not found: %s", deviceID)
		}
	} else {
		for _, d := range te.st.Devices {
			if d.Seq > 0 {
				dev = d
				break
			}
		}
	}

	if dev == nil || dev.Seq == 0 {
		return fmt.Errorf("no device with recent events")
	}

	for i := 0; i < count; i++ {
		evt := TelemetryEvent{
			DeviceID:  dev.DeviceID,
			PoleID:    dev.PoleID,
			Event:     "heartbeat",
			Energized: dev.Energized,
			Ts:        te.clock.TsForSim(te.clock.NowSim(), dev.ClockSkewSecs),
			Seq:       dev.Seq,
			BatteryMV: dev.BatteryMV,
			RSSI:      dev.RSSI,
			Fw:        dev.Firmware,
		}
		te.queue.Schedule(te.clock.NowSim()+dev.RadioDelaySecs, evt)
	}
	return nil
}

// InjectStaleReplay emits an old power_lost event with a stale timestamp and
// sequence, simulating the 6-hour retry behavior from devices that were offline.
func (te *TelemetryEngine) InjectStaleReplay(deviceID string, count int) error {
	te.mu.Lock()
	defer te.mu.Unlock()

	var dev *DeviceState
	if deviceID != "" {
		var ok bool
		dev, ok = te.st.Devices[deviceID]
		if !ok {
			return fmt.Errorf("device not found: %s", deviceID)
		}
	} else {
		for _, d := range te.st.Devices {
			if !d.Energized {
				dev = d
				break
			}
		}
	}

	if dev == nil || dev.Seq == 0 {
		return fmt.Errorf("no suitable device for stale replay")
	}

	staleSim := te.clock.NowSim() - 7200
	for i := 0; i < count; i++ {
		evt := TelemetryEvent{
			DeviceID:  dev.DeviceID,
			PoleID:    dev.PoleID,
			Event:     "power_lost",
			Energized: false,
			Ts:        te.clock.TsForSim(staleSim, dev.ClockSkewSecs),
			Seq:       dev.Seq,
			BatteryMV: dev.BatteryMV,
			RSSI:      dev.RSSI,
			Fw:        dev.Firmware,
		}
		te.queue.Schedule(te.clock.NowSim()+dev.RadioDelaySecs, evt)
	}
	return nil
}

package ingestor

import (
	"log"
	"sync"
	"time"

	"power-fault-detector/internal/detect"
)

type Config struct {
	DetectionWindowSecs int
}

func DefaultConfig() Config {
	return Config{
		DetectionWindowSecs: 60,
	}
}

type Ingestor struct {
	mu         sync.RWMutex
	devices    map[string]*DeviceIngestState
	topology   *detect.TopologyIndex
	buffer     *TemporalBuffer
	config     Config
	stats      IngestStats
}

type IngestStats struct {
	TotalEvents   uint64
	Deduplicated  uint64
	StaleDropped  uint64
	Heartbeats    uint64
	PowerLost     uint64
	PowerRestored uint64
	Boots         uint64
}

func NewIngestor(topology *detect.TopologyIndex, detectChan chan<- detect.DetectionJob, cfg Config) *Ingestor {
	flushDelay := time.Duration(cfg.DetectionWindowSecs) * time.Second
	return &Ingestor{
		devices:  make(map[string]*DeviceIngestState),
		topology: topology,
		buffer:   NewTemporalBuffer(flushDelay, detectChan),
		config:   cfg,
	}
}

func (ing *Ingestor) ProcessEvent(ev TelemetryEvent) {
	ing.mu.Lock()
	defer ing.mu.Unlock()

	pole, ok := ing.topology.DeviceToPole[ev.DeviceID]
	if !ok {
		ing.stats.TotalEvents++
		return
	}

	state, exists := ing.devices[ev.DeviceID]
	if !exists {
		state = NewDeviceState(ev.DeviceID, pole.ID, pole.DTID, pole.FeederID)
		ing.devices[ev.DeviceID] = state
	}

	dedup := CheckDedup(ev, state)
	if dedup.IsDuplicate {
		ing.stats.Deduplicated++
		return
	}
	if dedup.IsStale {
		ing.stats.StaleDropped++
		return
	}

	record := EventRecord{
		Event:      ev.Event,
		Energized:  ev.Energized,
		Reported:   ev.Reported,
		Ts:         ev.Ts,
		Seq:        ev.Seq,
		BatteryMV:  ev.BatteryMV,
		RSSI:       ev.RSSI,
		Fw:         ev.Fw,
		ReceivedAt: time.Now(),
		PoleID:     pole.ID,
		DeviceID:   ev.DeviceID,
	}

	state.RecordEvent(record)
	ing.stats.TotalEvents++

	switch ev.Event {
	case "heartbeat":
		ing.stats.Heartbeats++
		if state.IsDark() {
			state.ClearDarkSince()
			ing.buffer.HandleRestoration(record, pole.DTID)
		}
	case "power_lost":
		ing.stats.PowerLost++
		if !state.IsDark() {
			darkSim := int64(0)
			state.RecordDarkSince(darkSim)
			ing.buffer.AddEvent(record, pole.DTID, true)
		}
	case "boot":
		ing.stats.Boots++
	case "power_restored":
		ing.stats.PowerRestored++
		if state.IsDark() {
			state.ClearDarkSince()
			ing.buffer.HandleRestoration(record, pole.DTID)
		}
	}
}

func (ing *Ingestor) GetStats() IngestStats {
	ing.mu.RLock()
	defer ing.mu.RUnlock()
	return ing.stats
}

func (ing *Ingestor) GetDeviceState(deviceID string) *DeviceIngestState {
	ing.mu.RLock()
	defer ing.mu.RUnlock()
	return ing.devices[deviceID]
}

func (ing *Ingestor) GetTopology() *detect.TopologyIndex {
	return ing.topology
}

func (ing *Ingestor) GetDeviceStates() map[string]detect.DeviceStateRef {
	ing.mu.RLock()
	defer ing.mu.RUnlock()
	// Return a copy to avoid race conditions
	result := make(map[string]detect.DeviceStateRef, len(ing.devices))
	for k, v := range ing.devices {
		result[k] = v
	}
	return result
}

func (ing *Ingestor) StartStatsLogger() {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			stats := ing.GetStats()
			if stats.TotalEvents == 0 {
				continue
			}
			log.Printf("[ingestor] events=%d dedup=%d stale=%d hb=%d lost=%d restored=%d boot=%d pending_buf=%d",
				stats.TotalEvents, stats.Deduplicated, stats.StaleDropped,
				stats.Heartbeats, stats.PowerLost, stats.PowerRestored, stats.Boots,
				ing.buffer.PendingCount())
		}
	}()
}

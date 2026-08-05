package ingestor

import "time"

type EventRecord struct {
	Event      string    `json:"event"`
	Energized  bool      `json:"energized"`
	Reported   bool      `json:"reported"`
	Ts         string    `json:"ts"`
	Seq        int64     `json:"seq"`
	BatteryMV  int       `json:"battery_mv"`
	RSSI       int       `json:"rssi"`
	Fw         string    `json:"fw"`
	ReceivedAt time.Time `json:"received_at"`
	PoleID     string    `json:"pole_id"`
	DeviceID   string    `json:"device_id"`
}

type DeviceIngestState struct {
	DeviceID     string
	PoleID       string
	DTID         string
	FeederID     string
	LastSeq      int64
	Energized    bool
	LastEvent    string
	DarkSinceSim *int64
	EventHistory []EventRecord
}

const maxHistory = 20

func NewDeviceState(deviceID, poleID, dtID, feederID string) *DeviceIngestState {
	return &DeviceIngestState{
		DeviceID:     deviceID,
		PoleID:       poleID,
		DTID:         dtID,
		FeederID:     feederID,
		Energized:    true,
		EventHistory: make([]EventRecord, 0, maxHistory),
	}
}

func (d *DeviceIngestState) RecordEvent(ev EventRecord) {
	d.LastSeq = ev.Seq
	d.LastEvent = ev.Event
	d.Energized = ev.Energized

	d.EventHistory = append(d.EventHistory, ev)
	if len(d.EventHistory) > maxHistory {
		d.EventHistory = d.EventHistory[1:]
	}
}

func (d *DeviceIngestState) RecordDarkSince(simSecs int64) {
	d.DarkSinceSim = &simSecs
}

func (d *DeviceIngestState) ClearDarkSince() {
	d.DarkSinceSim = nil
}

func (d *DeviceIngestState) IsDark() bool {
	return d.DarkSinceSim != nil
}

// GetDeviceID returns the device ID (implements DeviceStateRef)
func (d *DeviceIngestState) GetDeviceID() string {
	return d.DeviceID
}

// IsEnergized returns whether the device is currently energized (implements DeviceStateRef)
func (d *DeviceIngestState) IsEnergized() bool {
	return d.Energized
}

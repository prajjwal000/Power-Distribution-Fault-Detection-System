package ingestor

type TelemetryEvent struct {
	DeviceID  string `json:"device_id"`
	PoleID    string `json:"pole_id"`
	Event     string `json:"event"`
	Energized bool   `json:"energized"`
	Reported  bool   `json:"reported"`
	Ts        string `json:"ts"`
	Seq       int64  `json:"seq"`
	BatteryMV int    `json:"battery_mv"`
	RSSI      int    `json:"rssi"`
	Fw        string `json:"fw"`
}

type DedupResult struct {
	IsDuplicate bool
	IsStale     bool
	ShouldProcess bool
}

func CheckDedup(ev TelemetryEvent, state *DeviceIngestState) DedupResult {
	if state == nil {
		return DedupResult{ShouldProcess: true}
	}

	if ev.Seq < state.LastSeq {
		return DedupResult{IsDuplicate: true, IsStale: true}
	}

	if ev.Seq == state.LastSeq && ev.Event == state.LastEvent {
		return DedupResult{IsDuplicate: true}
	}

	return DedupResult{ShouldProcess: true}
}

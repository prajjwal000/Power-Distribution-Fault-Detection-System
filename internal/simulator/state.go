package simulator

import (
	"math/rand"
	"power-fault-detector/internal/model"
)

type DeviceState struct {
	PoleID         string
	DeviceID       string
	Firmware       string
	BatteryMV      int
	RSSI           int
	Seq            int64
	Energized      bool
	ClockSkewSecs  int64
	RadioDelaySecs int64
	NextEmitSim    int64
}

func (d *DeviceState) WillEmitPowerLost() bool {
	if len(d.Firmware) >= 3 && d.Firmware[0] == '1' && d.Firmware[1] == '.' && d.Firmware[2] == '2' {
		return false
	}
	return rand.Float64() < 0.70
}

type TelemetryEvent struct {
	DeviceID  string `json:"device_id"`
	PoleID    string `json:"pole_id"`
	Event     string `json:"event"`
	Energized bool   `json:"energized"`
	Ts        string `json:"ts"`
	Seq       int64  `json:"seq"`
	BatteryMV int    `json:"battery_mv"`
	RSSI      int    `json:"rssi"`
	Fw        string `json:"fw"`
}

type SimulatorState struct {
	PoleByID     map[string]*model.Pole
	PoleByDevice map[string]*model.Pole
	Devices      map[string]*DeviceState
	Substations  []model.Substation
	Feeders      []model.Feeder
	Transformers []model.Transformer
	GTPoles      []model.GTPole
	Children     map[string][]string
	Parents      map[string]string
	ActiveFaults map[string]*Fault
}

type Fault struct {
	ID          string
	Span        string
	AffectedSet []string
	StartSim    int64
}

func NewSimulatorState() *SimulatorState {
	return &SimulatorState{
		PoleByID:     make(map[string]*model.Pole),
		PoleByDevice: make(map[string]*model.Pole),
		Devices:      make(map[string]*DeviceState),
		Children:     make(map[string][]string),
		Parents:      make(map[string]string),
		ActiveFaults: make(map[string]*Fault),
	}
}

func (st *SimulatorState) Descendants(poleID string) []string {
	var out []string
	stack := []string{poleID}
	visited := make(map[string]bool)
	for len(stack) > 0 {
		curr := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if visited[curr] {
			continue
		}
		visited[curr] = true
		if curr != poleID {
			out = append(out, curr)
		}
		for _, child := range st.Children[curr] {
			if !visited[child] {
				stack = append(stack, child)
			}
		}
	}
	return out
}

func (st *SimulatorState) AffectedPolesForSpan(childPoleID string) []string {
	desc := st.Descendants(childPoleID)
	return append([]string{childPoleID}, desc...)
}

func (st *SimulatorState) AffectedPolesForDT(dtID string) []string {
	var out []string
	for _, p := range st.PoleByID {
		if p.DTID == dtID {
			out = append(out, p.ID)
		}
	}
	return out
}

func (st *SimulatorState) AffectedPolesForFeeder(feederID string) []string {
	var out []string
	for _, p := range st.PoleByID {
		if p.FeederID == feederID {
			out = append(out, p.ID)
		}
	}
	return out
}

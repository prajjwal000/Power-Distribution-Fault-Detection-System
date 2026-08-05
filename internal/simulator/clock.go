package simulator

import (
	"fmt"
	"sync"
	"time"
)

type Clock struct {
	mu            sync.RWMutex
	multiplier    int
	bootWallTime  time.Time
	bootSimTime   int64
	paused        bool
	pauseSimTime  int64
	pauseWallTime time.Time
}

func NewClock(multiplier int) *Clock {
	if multiplier < 1 {
		multiplier = 1
	}
	return &Clock{
		multiplier:   multiplier,
		bootWallTime: time.Now(),
		bootSimTime:  0,
		paused:       false,
	}
}

func (c *Clock) NowSim() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.nowSimRLocked()
}

func (c *Clock) nowSimRLocked() int64 {
	if c.paused {
		return c.pauseSimTime
	}
	wallElapsed := time.Since(c.bootWallTime)
	simElapsed := wallElapsed * time.Duration(c.multiplier)
	return c.bootSimTime + int64(simElapsed.Seconds())
}

// SetMultiplier changes the sim rate without jumping sim time: the current
// sim instant is rebased so the new rate applies only from now on.
func (c *Clock) SetMultiplier(m int) {
	if m < 1 {
		m = 1
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.paused {
		c.bootSimTime = c.nowSimRLocked()
		c.bootWallTime = time.Now()
	}
	c.multiplier = m
}

func (c *Clock) GetMultiplier() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.multiplier
}

func (c *Clock) Pause() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.paused {
		c.pauseSimTime = c.nowSimRLocked()
		c.pauseWallTime = time.Now()
		c.paused = true
	}
}

func (c *Clock) Resume() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.paused {
		c.bootSimTime = c.pauseSimTime
		c.bootWallTime = time.Now()
		c.paused = false
	}
}

func (c *Clock) SetSimTime(t int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.paused {
		c.pauseSimTime = t
		c.pauseWallTime = time.Now()
	} else {
		c.bootSimTime = t
		c.bootWallTime = time.Now()
	}
}

// WallForSim maps a sim instant to the wall-clock time at which it occurs.
// While paused, the frozen sim instant stays anchored to the moment of pause.
func (c *Clock) WallForSim(simSecs int64) time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	mult := time.Duration(c.multiplier)
	if mult < 1 {
		mult = 1
	}
	if c.paused {
		return c.pauseWallTime.Add(time.Duration(simSecs-c.pauseSimTime) * time.Second / mult)
	}
	return c.bootWallTime.Add(time.Duration(simSecs-c.bootSimTime) * time.Second / mult)
}

// TsForSim renders the device-visible timestamp for an event that happened at
// simSecs, as reported by a device whose local clock drifts skewSecs from the
// true wall clock. Skew is applied in the wall domain: two devices that go
// dark at the same sim instant report timestamps up to 180 real seconds apart.
// This is the only place sim time becomes a telemetry `ts`.
func (c *Clock) TsForSim(simSecs, skewSecs int64) string {
	wall := c.WallForSim(simSecs).Add(time.Duration(skewSecs) * time.Second)
	return wall.UTC().Format("2006-01-02T15:04:05.000Z")
}

type ClockResponse struct {
	SimTime    string `json:"sim_time"`
	Multiplier int    `json:"multiplier"`
	WallTime   string `json:"wall_time"`
	Paused     bool   `json:"paused"`
}

func (c *Clock) Response() ClockResponse {
	sim := c.NowSim()
	h := (sim / 3600) % 24
	m := (sim / 60) % 60
	s := sim % 60
	c.mu.RLock()
	paused := c.paused
	mult := c.multiplier
	c.mu.RUnlock()
	return ClockResponse{
		SimTime:    fmt.Sprintf("%02d:%02d:%02d", h, m, s),
		Multiplier: mult,
		WallTime:   time.Now().UTC().Format(time.RFC3339),
		Paused:     paused,
	}
}

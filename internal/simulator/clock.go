package simulator

import (
	"fmt"
	"sync"
	"time"
)

type Clock struct {
	mu           sync.RWMutex
	multiplier   int
	bootWallTime time.Time
	bootSimTime  int64
	paused       bool
	pauseSimTime int64
}

func NewClock(multiplier int) *Clock {
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

func (c *Clock) SetMultiplier(m int) {
	c.mu.Lock()
	defer c.mu.Unlock()
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
		c.paused = true
	}
}

func (c *Clock) Resume() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.paused {
		wallOffset := time.Duration(c.pauseSimTime-c.bootSimTime) * time.Second / time.Duration(c.multiplier)
		c.bootWallTime = time.Now().Add(-wallOffset)
		c.paused = false
	}
}

func (c *Clock) SetSimTime(t int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.paused {
		c.pauseSimTime = t
	} else {
		c.bootSimTime = t
		c.bootWallTime = time.Now()
	}
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

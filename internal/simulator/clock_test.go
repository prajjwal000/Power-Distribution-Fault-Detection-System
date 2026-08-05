package simulator

import (
	"math"
	"testing"
	"time"
)

func TestSetMultiplierDoesNotJumpSimTime(t *testing.T) {
	c := NewClock(30)
	c.SetSimTime(10_000)

	c.SetMultiplier(60)

	got := c.NowSim()
	if math.Abs(float64(got-10_000)) > 2 {
		t.Errorf("sim time after multiplier change = %d, want ~10000 (no jump)", got)
	}
}

func TestSetMultiplierAppliesNewRate(t *testing.T) {
	c := NewClock(1)
	c.SetSimTime(0)
	time.Sleep(60 * time.Millisecond)

	before := c.NowSim()
	c.SetMultiplier(1000)
	time.Sleep(60 * time.Millisecond)

	after := c.NowSim()
	if after-before < 20 {
		t.Errorf("sim advanced %d secs after rate increase, want a large jump", after-before)
	}
}

func TestWallForSimRoundTrip(t *testing.T) {
	c := NewClock(30)
	c.SetSimTime(3000)

	nowWall := time.Now()
	got := c.WallForSim(3000)
	if got.Sub(nowWall).Abs() > 500*time.Millisecond {
		t.Errorf("WallForSim(current sim) = %v, want ~%v", got, nowWall)
	}

	// 300 sim seconds in the past at 30x = 10 wall seconds ago.
	past := c.WallForSim(2700)
	want := nowWall.Add(-10 * time.Second)
	if past.Sub(want).Abs() > 500*time.Millisecond {
		t.Errorf("WallForSim(2700) = %v, want ~%v", past, want)
	}
}

func TestTsForSimAppliesDeviceSkew(t *testing.T) {
	c := NewClock(30)
	c.SetSimTime(3000)

	base, err := time.Parse("2006-01-02T15:04:05.000Z", c.TsForSim(3000, 0))
	if err != nil {
		t.Fatalf("parse ts: %v", err)
	}
	skewed, err := time.Parse("2006-01-02T15:04:05.000Z", c.TsForSim(3000, 90))
	if err != nil {
		t.Fatalf("parse ts: %v", err)
	}

	diff := skewed.Sub(base)
	if diff < 89*time.Second || diff > 91*time.Second {
		t.Errorf("skew delta = %v, want ~90s", diff)
	}
}

func TestTsForSimNeverBeforeBoot(t *testing.T) {
	boot := time.Now()
	c := NewClock(30)

	ts, err := time.Parse("2006-01-02T15:04:05.000Z", c.TsForSim(0, 0))
	if err != nil {
		t.Fatalf("parse ts: %v", err)
	}
	if ts.Before(boot.Add(-time.Second)) {
		t.Errorf("ts %v is before simulator boot %v", ts, boot)
	}
}

func TestPauseFreezesSimAndAnchorsWall(t *testing.T) {
	c := NewClock(30)
	c.SetSimTime(500)
	c.Pause()

	time.Sleep(30 * time.Millisecond)
	if got := c.NowSim(); got != 500 {
		t.Errorf("paused sim = %d, want 500", got)
	}

	pauseWall := time.Now()
	got := c.WallForSim(500)
	if got.Sub(pauseWall).Abs() > 500*time.Millisecond {
		t.Errorf("WallForSim(paused sim) = %v, want ~%v", got, pauseWall)
	}

	c.Resume()
	if got := c.NowSim(); got < 500 {
		t.Errorf("resumed sim = %d, want >= 500", got)
	}
}

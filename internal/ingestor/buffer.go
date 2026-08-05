package ingestor

import (
	"log"
	"sync"
	"time"

	"power-fault-detector/internal/detect"
)

type DTBuffer struct {
	DTID        string
	DarkPoles   map[string]time.Time
	LitPoles    map[string]time.Time
	Events      []EventRecord
	FirstDarkAt time.Time
	Timer       *time.Timer
}

type TemporalBuffer struct {
	mu         sync.Mutex
	pending    map[string]*DTBuffer
	flushDelay time.Duration
	detectChan chan<- detect.DetectionJob
}

func NewTemporalBuffer(flushDelay time.Duration, detectChan chan<- detect.DetectionJob) *TemporalBuffer {
	return &TemporalBuffer{
		pending:    make(map[string]*DTBuffer),
		flushDelay: flushDelay,
		detectChan: detectChan,
	}
}

func (tb *TemporalBuffer) AddEvent(ev EventRecord, dtID string, isDarkEvent bool) {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	buf, exists := tb.pending[dtID]
	if !exists {
		buf = &DTBuffer{
			DTID:      dtID,
			DarkPoles: make(map[string]time.Time),
			LitPoles:  make(map[string]time.Time),
			Events:    make([]EventRecord, 0),
		}
		tb.pending[dtID] = buf
	}

	buf.Events = append(buf.Events, ev)

	if isDarkEvent {
		now := time.Now()
		if _, exists := buf.DarkPoles[ev.PoleID]; !exists {
			buf.DarkPoles[ev.PoleID] = now
		}
		if buf.FirstDarkAt.IsZero() {
			buf.FirstDarkAt = now
			buf.Timer = time.AfterFunc(tb.flushDelay, func() {
				tb.flush(dtID)
			})
		}
	} else {
		buf.LitPoles[ev.PoleID] = time.Now()
		if buf.Timer != nil && len(buf.DarkPoles) == 0 {
			buf.Timer.Stop()
			buf.Timer = nil
			delete(tb.pending, dtID)
		}
	}
}

func (tb *TemporalBuffer) HandleRestoration(ev EventRecord, dtID string) {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	buf, exists := tb.pending[dtID]
	if !exists {
		return
	}

	delete(buf.DarkPoles, ev.PoleID)

	if len(buf.DarkPoles) == 0 && buf.Timer != nil {
		buf.Timer.Stop()
		buf.Timer = nil
		delete(tb.pending, dtID)
		log.Printf("[buffer] DT %s: all poles restored, cancelling detection", dtID)
	}
}

func (tb *TemporalBuffer) flush(dtID string) {
	tb.mu.Lock()
	buf, exists := tb.pending[dtID]
	if !exists {
		tb.mu.Unlock()
		return
	}
	delete(tb.pending, dtID)
	tb.mu.Unlock()

	darkPoles := make([]string, 0, len(buf.DarkPoles))
	for poleID := range buf.DarkPoles {
		darkPoles = append(darkPoles, poleID)
	}

	litPoles := make([]string, 0, len(buf.LitPoles))
	for poleID := range buf.LitPoles {
		litPoles = append(litPoles, poleID)
	}

	events := make([]detect.EventRecord, len(buf.Events))
	for i, ev := range buf.Events {
		events[i] = detect.EventRecord{
			Event:      ev.Event,
			Energized:  ev.Energized,
			Reported:   ev.Reported,
			Ts:         ev.Ts,
			Seq:        ev.Seq,
			BatteryMV:  ev.BatteryMV,
			RSSI:       ev.RSSI,
			Fw:         ev.Fw,
			ReceivedAt: ev.ReceivedAt,
			PoleID:     ev.PoleID,
			DeviceID:   ev.DeviceID,
		}
	}

	job := detect.DetectionJob{
		DTID:       dtID,
		DarkPoles:  darkPoles,
		LitPoles:   litPoles,
		Events:     events,
		DetectedAt: time.Now(),
	}

	select {
	case tb.detectChan <- job:
		log.Printf("[buffer] DT %s: flushed %d dark, %d lit poles to detection", dtID, len(darkPoles), len(litPoles))
	default:
		log.Printf("[buffer] DT %s: detection channel full, dropping job", dtID)
	}
}

func (tb *TemporalBuffer) PendingCount() int {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	return len(tb.pending)
}

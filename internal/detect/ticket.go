package detect

import (
	"time"
)

type TicketStatus string

const (
	TicketActive   TicketStatus = "active"
	TicketVerified TicketStatus = "verified"
	TicketResolved TicketStatus = "resolved"
)

type TicketSeverity string

const (
	SeverityCritical TicketSeverity = "critical"
	SeverityMajor    TicketSeverity = "major"
	SeverityMinor    TicketSeverity = "minor"
)

type TicketScope string

const (
	ScopeSpan   TicketScope = "span"
	ScopeDT     TicketScope = "dt"
	ScopeFeeder TicketScope = "feeder"
)

type Location struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type Ticket struct {
	ID            string         `json:"id"`
	Status        TicketStatus   `json:"status"`
	Severity      TicketSeverity `json:"severity"`
	Scope         TicketScope    `json:"scope"`
	TargetID      string         `json:"target_id"`
	DTID          string         `json:"dt_id"`
	FeederID      string         `json:"feeder_id"`
	AffectedCount int            `json:"affected_count"`
	AffectedPoles []string       `json:"affected_poles"`
	Confidence    float64        `json:"confidence"`
	Evidence      []EventRecord  `json:"evidence"`
	DetectedAt    time.Time      `json:"detected_at"`
	VerifiedAt    *time.Time     `json:"verified_at,omitempty"`
	ResolvedAt    *time.Time     `json:"resolved_at,omitempty"`
	PINCode       *string        `json:"pincode,omitempty"`
	Location      *Location      `json:"location,omitempty"`
	IsRefined     bool           `json:"is_refined"`
}

type DetectionJob struct {
	DTID       string
	DarkPoles  []string
	LitPoles   []string
	Events     []EventRecord
	DetectedAt time.Time
}

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

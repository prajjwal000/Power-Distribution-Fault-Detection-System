package detect

import (
	"log"
	"sync"
	"time"
)

type Engine struct {
	mu           sync.RWMutex
	topo         *TopologyIndex
	tickets      map[string]*Ticket
	jobChan      chan DetectionJob
	broadcaster  *Broadcaster
	stopCh       chan struct{}
	ticketSeq    int
	ingestor     IngestorRef
}

type IngestorRef interface {
	GetDeviceStates() map[string]DeviceStateRef
}

type DeviceStateRef interface {
	GetDeviceID() string
	IsEnergized() bool
}

func NewEngine(topo *TopologyIndex) *Engine {
	return &Engine{
		topo:       topo,
		tickets:    make(map[string]*Ticket),
		jobChan:    make(chan DetectionJob, 1024),
		broadcaster: NewBroadcaster(),
		stopCh:     make(chan struct{}),
	}
}

func (e *Engine) SetIngestor(ing IngestorRef) {
	e.ingestor = ing
}

func (e *Engine) JobChannel() chan<- DetectionJob {
	return e.jobChan
}

func (e *Engine) SetBroadcaster(b *Broadcaster) {
	e.broadcaster = b
}

func (e *Engine) Start() {
	go e.processLoop()
	go e.refinementLoop()
	go e.verificationLoop()
	log.Println("[detect] engine started")
}

func (e *Engine) Stop() {
	close(e.stopCh)
}

func (e *Engine) processLoop() {
	for {
		select {
		case job := <-e.jobChan:
			e.processJob(job)
		case <-e.stopCh:
			return
		}
	}
}

func (e *Engine) processJob(job DetectionJob) {
	candidates := Localize(job, e.topo)

	for _, c := range candidates {
		e.mu.Lock()
		ticket := e.createTicket(c, job)
		e.tickets[ticket.ID] = ticket
		e.mu.Unlock()

		log.Printf("[detect] ticket %s: %s fault at %s (confidence %.2f) DT=%s affected=%d poles",
			ticket.ID, ticket.Scope, ticket.TargetID, ticket.Confidence, ticket.DTID, ticket.AffectedCount)

		e.broadcaster.Broadcast(TicketUpdate{
			Type:   "ticket_created",
			Ticket: *ticket,
		})
	}
}

func (e *Engine) createTicket(c FaultCandidate, job DetectionJob) *Ticket {
	e.ticketSeq++
	ticketID := "T-" + time.Now().Format("20060102") + "-" + string(rune('A'+e.ticketSeq%26))

	var severity TicketSeverity
	switch {
	case c.Confidence >= 0.8:
		severity = SeverityCritical
	case c.Confidence >= 0.5:
		severity = SeverityMajor
	default:
		severity = SeverityMinor
	}

	var feederID string
	if len(c.AffectedPoles) > 0 {
		if p, ok := e.topo.PoleByID[c.AffectedPoles[0]]; ok {
			feederID = p.FeederID
		}
	}

	return &Ticket{
		ID:            ticketID,
		Status:        TicketActive,
		Severity:      severity,
		Scope:         c.Scope,
		TargetID:      c.TargetID,
		DTID:          job.DTID,
		FeederID:      feederID,
		AffectedCount: len(c.AffectedPoles),
		AffectedPoles: c.AffectedPoles,
		Confidence:    c.Confidence,
		Evidence:      job.Events,
		DetectedAt:    job.DetectedAt,
		Location:      c.Location,
	}
}

func (e *Engine) refinementLoop() {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			e.refineTickets()
		case <-e.stopCh:
			return
		}
	}
}

func (e *Engine) refineTickets() {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, ticket := range e.tickets {
		if ticket.Status != TicketActive || ticket.IsRefined {
			continue
		}

		if ticket.Scope == ScopeSpan && e.topo.HasKnownTopology(ticket.DTID) {
			ticket.Confidence = min(ticket.Confidence+0.15, 0.99)
			ticket.IsRefined = true
			log.Printf("[detect] ticket %s: refined confidence to %.2f", ticket.ID, ticket.Confidence)

			e.broadcaster.Broadcast(TicketUpdate{
				Type:   "ticket_refined",
				Ticket: *ticket,
			})
		}
	}
}

func (e *Engine) verificationLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			e.verifyTickets()
		case <-e.stopCh:
			return
		}
	}
}

func (e *Engine) verifyTickets() {
	if e.ingestor == nil {
		log.Printf("[detect] verifyTickets: ingestor is nil")
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	deviceStates := e.ingestor.GetDeviceStates()
	log.Printf("[detect] verifyTickets: got %d device states", len(deviceStates))
	if len(deviceStates) == 0 {
		return
	}

	energizedDevices := make(map[string]bool)
	for _, state := range deviceStates {
		energizedDevices[state.GetDeviceID()] = state.IsEnergized()
	}
	log.Printf("[detect] verifyTickets: energized count = %d", countEnergized(energizedDevices))

	for _, ticket := range e.tickets {
		if ticket.Status != TicketActive {
			continue
		}

		allRestored := true
		missingCount := 0
		notEnergizedCount := 0
		for _, pid := range ticket.AffectedPoles {
			if p, ok := e.topo.PoleByID[pid]; ok && p.DeviceID != nil {
				deviceID := *p.DeviceID
				if energized, ok := energizedDevices[deviceID]; ok && !energized {
					allRestored = false
					notEnergizedCount++
					break
				}
				if !ok {
					allRestored = false
					missingCount++
					break
				}
			}
		}
		log.Printf("[detect] verifyTickets: ticket %s - allRestored=%v missing=%d notEnergized=%d", ticket.ID, allRestored, missingCount, notEnergizedCount)

		if allRestored {
			now := time.Now()
			ticket.Status = TicketVerified
			ticket.VerifiedAt = &now
			log.Printf("[detect] ticket %s: auto-verified (all %d poles restored)", ticket.ID, len(ticket.AffectedPoles))

			e.broadcaster.Broadcast(TicketUpdate{
				Type:   "ticket_verified",
				Ticket: *ticket,
			})
		}
	}
}

func countEnergized(m map[string]bool) int {
	count := 0
	for _, v := range m {
		if v {
			count++
		}
	}
	return count
}

func (e *Engine) GetTickets() []*Ticket {
	e.mu.RLock()
	defer e.mu.RUnlock()
	tickets := make([]*Ticket, 0, len(e.tickets))
	for _, t := range e.tickets {
		tickets = append(tickets, t)
	}
	return tickets
}

func (e *Engine) GetTicket(id string) *Ticket {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.tickets[id]
}

func (e *Engine) Broadcaster() *Broadcaster {
	return e.broadcaster
}
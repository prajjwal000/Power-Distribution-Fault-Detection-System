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
}

func NewEngine(topo *TopologyIndex) *Engine {
	return &Engine{
		topo:        topo,
		tickets:     make(map[string]*Ticket),
		jobChan:     make(chan DetectionJob, 1024),
		broadcaster: NewBroadcaster(),
		stopCh:      make(chan struct{}),
	}
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

func (e *Engine) HandleRestoration(deviceID string, poleID string, dtID string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, ticket := range e.tickets {
		if ticket.DTID != dtID || ticket.Status != TicketActive {
			continue
		}

		restored := true
		for _, pid := range ticket.AffectedPoles {
			if pid == poleID {
				continue
			}
			if p, ok := e.topo.PoleByID[pid]; ok && p.DeviceID != nil {
				if _, ok := e.topo.DeviceToPole[*p.DeviceID]; ok {
					restored = false
					break
				}
			}
		}

		if restored {
			now := time.Now()
			ticket.Status = TicketVerified
			ticket.VerifiedAt = &now
			log.Printf("[detect] ticket %s: auto-verified (all poles restored)", ticket.ID)

			e.broadcaster.Broadcast(TicketUpdate{
				Type:   "ticket_verified",
				Ticket: *ticket,
			})
		}
	}
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

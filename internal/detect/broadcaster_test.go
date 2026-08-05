package detect

import (
	"testing"
	"time"
)

func TestBroadcaster_SubscribeUnsubscribe(t *testing.T) {
	b := NewBroadcaster()

	ch := b.Subscribe()
	if b.SubscriberCount() != 1 {
		t.Errorf("expected 1 subscriber, got %d", b.SubscriberCount())
	}

	b.Unsubscribe(ch)
	if b.SubscriberCount() != 0 {
		t.Errorf("expected 0 subscribers, got %d", b.SubscriberCount())
	}
}

func TestBroadcaster_Broadcast(t *testing.T) {
	b := NewBroadcaster()

	ch := b.Subscribe()

	ticket := Ticket{
		ID:     "T-001",
		Status: TicketActive,
		DTID:   "D-01",
	}

	update := TicketUpdate{
		Type:   "ticket_created",
		Ticket: ticket,
	}

	b.Broadcast(update)

	select {
	case received := <-ch:
		if received.Type != "ticket_created" {
			t.Errorf("expected type ticket_created, got %s", received.Type)
		}
		if received.Ticket.ID != "T-001" {
			t.Errorf("expected ticket ID T-001, got %s", received.Ticket.ID)
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for broadcast")
	}
}

func TestBroadcaster_MultipleSubscribers(t *testing.T) {
	b := NewBroadcaster()

	ch1 := b.Subscribe()
	ch2 := b.Subscribe()

	ticket := Ticket{ID: "T-002"}
	update := TicketUpdate{Type: "ticket_created", Ticket: ticket}

	b.Broadcast(update)

	timeout := time.After(time.Second)
	for _, ch := range []chan TicketUpdate{ch1, ch2} {
		select {
		case received := <-ch:
			if received.Ticket.ID != "T-002" {
				t.Errorf("expected ticket ID T-002, got %s", received.Ticket.ID)
			}
		case <-timeout:
			t.Error("timeout waiting for broadcast")
		}
	}

	b.Unsubscribe(ch1)
	b.Unsubscribe(ch2)
}

func TestBroadcaster_DropsOnSlowConsumer(t *testing.T) {
	b := NewBroadcaster()

	ch := b.Subscribe()

	for i := 0; i < 300; i++ {
		ticket := Ticket{ID: "T-FILL"}
		update := TicketUpdate{Type: "ticket_created", Ticket: ticket}
		b.Broadcast(update)
	}

	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Error("timeout waiting for broadcast")
	}
}

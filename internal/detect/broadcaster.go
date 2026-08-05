package detect

import (
	"log"
	"sync"
)

type TicketUpdate struct {
	Type   string `json:"type"`
	Ticket Ticket `json:"ticket"`
}

type Broadcaster struct {
	mu          sync.RWMutex
	subscribers map[chan TicketUpdate]struct{}
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		subscribers: make(map[chan TicketUpdate]struct{}),
	}
}

func (b *Broadcaster) Subscribe() chan TicketUpdate {
	ch := make(chan TicketUpdate, 256)
	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *Broadcaster) Unsubscribe(ch chan TicketUpdate) {
	b.mu.Lock()
	delete(b.subscribers, ch)
	b.mu.Unlock()
	close(ch)
}

func (b *Broadcaster) Broadcast(update TicketUpdate) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for ch := range b.subscribers {
		select {
		case ch <- update:
		default:
			log.Printf("[broadcaster] subscriber buffer full, dropping update")
		}
	}
}

func (b *Broadcaster) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers)
}

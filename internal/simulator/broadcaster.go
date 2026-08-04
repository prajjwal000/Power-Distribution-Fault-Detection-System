package simulator

import "sync"

const subscriberBufferSize = 256

type Broadcaster struct {
	mu          sync.RWMutex
	subscribers map[chan TelemetryEvent]struct{}
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		subscribers: make(map[chan TelemetryEvent]struct{}),
	}
}

func (b *Broadcaster) Subscribe() (<-chan TelemetryEvent, func()) {
	ch := make(chan TelemetryEvent, subscriberBufferSize)
	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()

	cancel := func() {
		b.mu.Lock()
		delete(b.subscribers, ch)
		close(ch)
		b.mu.Unlock()
	}

	return ch, cancel
}

func (b *Broadcaster) Publish(event TelemetryEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for ch := range b.subscribers {
		select {
		case ch <- event:
		default:
			// slow consumer — drop this event for this subscriber
		}
	}
}

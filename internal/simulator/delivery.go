package simulator

import (
	"container/heap"
	"sync"
)

// pendingEvent is a telemetry event scheduled for delivery at a sim instant.
type pendingEvent struct {
	deliverAtSim int64
	event        TelemetryEvent
}

type pendingHeap []*pendingEvent

func (h pendingHeap) Len() int           { return len(h) }
func (h pendingHeap) Less(i, j int) bool { return h[i].deliverAtSim < h[j].deliverAtSim }
func (h pendingHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *pendingHeap) Push(x any)        { *h = append(*h, x.(*pendingEvent)) }
func (h *pendingHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	*h = old[:n-1]
	return item
}

// DeliveryQueue is the single path through which every telemetry event —
// heartbeats, fault telemetry, restoration, noise — reaches the API and the
// SSE broadcaster. Keying delivery on sim time has two consequences: pausing
// the clock freezes delivery, and per-device radio delay offsets produce the
// same out-of-order arrivals a real network would (a downstream device with
// good radio can be delivered before the fault pole itself).
type DeliveryQueue struct {
	mu    sync.Mutex
	items pendingHeap
}

func NewDeliveryQueue() *DeliveryQueue {
	return &DeliveryQueue{}
}

func (q *DeliveryQueue) Schedule(deliverAtSim int64, event TelemetryEvent) {
	q.mu.Lock()
	heap.Push(&q.items, &pendingEvent{deliverAtSim: deliverAtSim, event: event})
	q.mu.Unlock()
}

// Due pops every event whose delivery time has arrived, in sim-time order.
func (q *DeliveryQueue) Due(nowSim int64) []TelemetryEvent {
	q.mu.Lock()
	defer q.mu.Unlock()
	var due []TelemetryEvent
	for q.items.Len() > 0 && q.items[0].deliverAtSim <= nowSim {
		item := heap.Pop(&q.items).(*pendingEvent)
		due = append(due, item.event)
	}
	return due
}

func (q *DeliveryQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.items.Len()
}

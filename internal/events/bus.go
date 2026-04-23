package events

import (
	"sync"
)

// Event is a single SSE event.
type Event struct {
	Topic string
	Type  string
	Data  any
}

// Bus is a lightweight in-process pub/sub event bus.
type Bus struct {
	mu   sync.RWMutex
	subs map[string][]chan Event
}

// NewBus creates a new EventBus.
func NewBus() *Bus {
	return &Bus{
		subs: make(map[string][]chan Event),
	}
}

// Subscribe creates a channel that receives events for the given topics.
func (b *Bus) Subscribe(topics ...string) chan Event {
	ch := make(chan Event, 16)
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, t := range topics {
		b.subs[t] = append(b.subs[t], ch)
	}
	return ch
}

// Unsubscribe removes a channel from all topics and closes it.
func (b *Bus) Unsubscribe(ch chan Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for t, subs := range b.subs {
		for i, s := range subs {
			if s == ch {
				b.subs[t] = append(subs[:i], subs[i+1:]...)
				break
			}
		}
		if len(b.subs[t]) == 0 {
			delete(b.subs, t)
		}
	}
	close(ch)
}

// Publish sends an event to all subscribers of the given topic.
func (b *Bus) Publish(topic string, ev Event) {
	b.mu.RLock()
	subs := make([]chan Event, len(b.subs[topic]))
	copy(subs, b.subs[topic])
	b.mu.RUnlock()

	for _, ch := range subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

// Close shuts down the bus and closes all subscriber channels.
func (b *Bus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, subs := range b.subs {
		for _, ch := range subs {
			close(ch)
		}
	}
	b.subs = make(map[string][]chan Event)
}

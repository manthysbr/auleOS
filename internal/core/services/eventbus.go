package services

import (
	"log/slog"
	"sync"
)

type EventType string

const (
	EventTypeStatus     EventType = "status"
	EventTypeLog        EventType = "log"
	EventTypeSubAgent   EventType = "sub_agent"
	EventTypeNewMessage EventType = "new_message"
)

type Event struct {
	JobID     string
	Type      EventType
	Data      string // JSON payload or raw text
	Timestamp int64
}

type Subscription struct {
	id     string
	events chan Event
}

type EventBus struct {
	logger   *slog.Logger
	mu       sync.RWMutex
	subs     map[string][]chan Event // Key: JobID
	globalCh []chan Event            // Subscribers that receive ALL events
}

func NewEventBus(logger *slog.Logger) *EventBus {
	return &EventBus{
		logger: logger,
		subs:   make(map[string][]chan Event),
	}
}

// SubscribeGlobal returns a channel that receives all events (broadcast + job events).
// Useful for the frontend to receive proactive agent messages without knowing job IDs.
func (b *EventBus) SubscribeGlobal() (<-chan Event, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan Event, 100)
	b.globalCh = append(b.globalCh, ch)

	unsub := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		for i, sub := range b.globalCh {
			if sub == ch {
				close(ch)
				b.globalCh = append(b.globalCh[:i], b.globalCh[i+1:]...)
				break
			}
		}
	}

	return ch, unsub
}

// Subscribe returns a channel that receives events for a specific job
func (b *EventBus) Subscribe(jobID string) (<-chan Event, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan Event, 100) // Buffer to prevent blocking publisher
	b.subs[jobID] = append(b.subs[jobID], ch)

	// Unsubscribe function
	unsub := func() {
		b.mu.Lock()
		defer b.mu.Unlock()

		subscribers := b.subs[jobID]
		for i, sub := range subscribers {
			if sub == ch {
				// Close channel
				close(ch)
				// Remove from slice
				b.subs[jobID] = append(subscribers[:i], subscribers[i+1:]...)
				break
			}
		}
		if len(b.subs[jobID]) == 0 {
			delete(b.subs, jobID)
		}
	}

	return ch, unsub
}

// Publish sends an event to all subscribers of the job AND global subscribers
func (b *EventBus) Publish(e Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Send to job-specific subscribers
	subscribers, ok := b.subs[e.JobID]
	if ok {
		for _, ch := range subscribers {
			select {
			case ch <- e:
			default:
				b.logger.Warn("event bus channel full, dropping event", "job_id", e.JobID)
			}
		}
	}

	// Send to global subscribers (broadcast channel + all events)
	for _, ch := range b.globalCh {
		select {
		case ch <- e:
		default:
			b.logger.Warn("event bus global channel full, dropping event", "job_id", e.JobID)
		}
	}
}

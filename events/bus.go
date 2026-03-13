package events

import (
	"context"
	"sync"
)

// HandlerFn is called for each published event of the subscribed type.
type HandlerFn func(ctx context.Context, event Event)

// Bus is the event bus interface (in-process now; NATS-compatible later).
type Bus interface {
	Publish(ctx context.Context, event Event) error
	Subscribe(eventType string, handler HandlerFn)
}

// InProcessBus is an in-memory pub/sub implementation.
type InProcessBus struct {
	mu       sync.RWMutex
	handlers map[string][]HandlerFn
}

// NewInProcessBus returns a new in-process event bus.
func NewInProcessBus() *InProcessBus {
	return &InProcessBus{
		handlers: make(map[string][]HandlerFn),
	}
}

// Publish delivers the event to all subscribers of that type.
func (b *InProcessBus) Publish(ctx context.Context, event Event) error {
	b.mu.RLock()
	fns := b.handlers[event.Type]
	b.mu.RUnlock()
	for _, fn := range fns {
		fn(ctx, event)
	}
	return nil
}

// Subscribe registers a handler for the given event type.
func (b *InProcessBus) Subscribe(eventType string, handler HandlerFn) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventType] = append(b.handlers[eventType], handler)
}

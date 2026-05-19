package server

import (
	"sync"

	"jute-dash/internal/store"
)

type EventBroker struct {
	mu          sync.Mutex
	subscribers map[chan store.ConversationEvent]struct{}
}

func NewEventBroker() *EventBroker {
	return &EventBroker{subscribers: map[chan store.ConversationEvent]struct{}{}}
}

func (b *EventBroker) Subscribe() chan store.ConversationEvent {
	ch := make(chan store.ConversationEvent, 32)
	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *EventBroker) Unsubscribe(ch chan store.ConversationEvent) {
	b.mu.Lock()
	delete(b.subscribers, ch)
	close(ch)
	b.mu.Unlock()
}

func (b *EventBroker) Publish(event store.ConversationEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

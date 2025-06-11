// events.go - Event system for cross-service communication
package events

import (
	"context"
	"sync"

	"github.com/charmbracelet/log"
)

// EventType represents different types of events in the system
type EventType string

const (
	// OAuth events
	OAuthGoogleRefreshed EventType = "oauth.google.refreshed"
	OAuthTokenExpired    EventType = "oauth.token.expired"
	
	// Holon events
	HolonClientRefreshed EventType = "holon.client.refreshed"
)

// Event represents a system event
type Event struct {
	Type      EventType              `json:"type"`
	Data      map[string]interface{} `json:"data"`
	Timestamp int64                  `json:"timestamp"`
}

// EventHandler is a function that handles events
type EventHandler func(ctx context.Context, event Event) error

// EventBus manages event subscriptions and publishing
type EventBus struct {
	mu           sync.RWMutex
	subscribers  map[EventType][]EventHandler
	logger       *log.Logger
}

// NewEventBus creates a new event bus
func NewEventBus(logger *log.Logger) *EventBus {
	return &EventBus{
		subscribers: make(map[EventType][]EventHandler),
		logger:      logger,
	}
}

// Subscribe adds an event handler for a specific event type
func (eb *EventBus) Subscribe(eventType EventType, handler EventHandler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	
	eb.subscribers[eventType] = append(eb.subscribers[eventType], handler)
	
	if eb.logger != nil {
		eb.logger.Debug("Event handler subscribed", "event_type", eventType)
	}
}

// Publish sends an event to all registered handlers
func (eb *EventBus) Publish(ctx context.Context, event Event) {
	eb.mu.RLock()
	handlers := eb.subscribers[event.Type]
	eb.mu.RUnlock()
	
	if eb.logger != nil {
		eb.logger.Debug("Publishing event", "event_type", event.Type, "handlers_count", len(handlers))
	}
	
	// Execute handlers concurrently
	var wg sync.WaitGroup
	for _, handler := range handlers {
		wg.Add(1)
		go func(h EventHandler) {
			defer wg.Done()
			if err := h(ctx, event); err != nil {
				if eb.logger != nil {
					eb.logger.Error("Event handler failed", "event_type", event.Type, "error", err)
				}
			}
		}(handler)
	}
	
	wg.Wait()
}

// Unsubscribe removes all handlers for a specific event type
func (eb *EventBus) Unsubscribe(eventType EventType) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	
	delete(eb.subscribers, eventType)
	
	if eb.logger != nil {
		eb.logger.Debug("Event handlers unsubscribed", "event_type", eventType)
	}
}

// GetSubscriberCount returns the number of handlers for an event type
func (eb *EventBus) GetSubscriberCount(eventType EventType) int {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	
	return len(eb.subscribers[eventType])
}
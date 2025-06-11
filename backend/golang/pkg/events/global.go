// global.go - Global event bus instance
package events

import (
	"sync"

	"github.com/charmbracelet/log"
)

var (
	globalEventBus *EventBus
	globalOnce     sync.Once
)

// GetGlobalEventBus returns the global event bus instance
func GetGlobalEventBus() *EventBus {
	globalOnce.Do(func() {
		// Initialize with a basic logger, can be updated later
		globalEventBus = NewEventBus(log.Default())
	})
	return globalEventBus
}

// SetGlobalEventBusLogger updates the logger for the global event bus
func SetGlobalEventBusLogger(logger *log.Logger) {
	bus := GetGlobalEventBus()
	bus.logger = logger
}
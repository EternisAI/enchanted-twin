// microscheduler_test_helpers.go provides test helper functions and types for microscheduler testing.
// This file contains common utilities used across all microscheduler test files.
package microscheduler

import (
	"encoding/json"
	"io"
	"time"

	"github.com/charmbracelet/log"
)

// createTestLogger creates a logger that only shows errors to keep test output clean.
func createTestLogger() *log.Logger {
	return log.NewWithOptions(io.Discard, log.Options{
		Level: log.ErrorLevel,
	})
}

// Helper function to create a simple stateless task.
func createStatelessTask(name string, priority Priority, duration time.Duration, compute func(resource interface{}) (interface{}, error)) Task {
	return Task{
		Name:         name,
		Priority:     priority,
		Duration:     duration,
		InitialState: &NoOpTaskState{},
		Compute: func(resource interface{}, state TaskState, interrupt *InterruptContext, interruptChan <-chan struct{}) (interface{}, error) {
			return compute(resource)
		},
	}
}

// Helper function to create an interruptible stateless task.
func createInterruptibleTask(name string, priority Priority, compute func(resource interface{}, interrupt <-chan struct{}) (interface{}, error)) Task {
	return Task{
		Name:         name,
		Priority:     priority,
		InitialState: &NoOpTaskState{},
		Compute: func(resource interface{}, state TaskState, interrupt *InterruptContext, interruptChan <-chan struct{}) (interface{}, error) {
			return compute(resource, interruptChan)
		},
	}
}

// Helper function to create a stateful task.
func createStatefulTask(name string, priority Priority, initialState TaskState, compute func(resource interface{}, state TaskState, interrupt *InterruptContext, interruptChan <-chan struct{}) (interface{}, error)) Task {
	return Task{
		Name:         name,
		Priority:     priority,
		InitialState: initialState,
		Compute:      compute,
	}
}

// SimpleTaskState is a simple implementation of TaskState for testing.
type SimpleTaskState struct {
	Counter int    `json:"counter"`
	Message string `json:"message"`
}

func (s *SimpleTaskState) Save() ([]byte, error) {
	return json.Marshal(s)
}

func (s *SimpleTaskState) Restore(data []byte) error {
	return json.Unmarshal(data, s)
}

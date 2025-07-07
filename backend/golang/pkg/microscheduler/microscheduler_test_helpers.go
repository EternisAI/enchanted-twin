package microscheduler

import (
	"encoding/json"
	"io"
	"time"

	"github.com/charmbracelet/log"
)

func createTestLogger() *log.Logger {
	return log.NewWithOptions(io.Discard, log.Options{
		Level: log.ErrorLevel,
	})
}

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

func createStatefulTask(name string, priority Priority, initialState TaskState, compute func(resource interface{}, state TaskState, interrupt *InterruptContext, interruptChan <-chan struct{}) (interface{}, error)) Task {
	return Task{
		Name:         name,
		Priority:     priority,
		InitialState: initialState,
		Compute:      compute,
	}
}

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
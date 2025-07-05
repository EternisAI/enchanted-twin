// microscheduler_stateful_test.go contains tests for stateful task execution:
// - Stateful task execution with state persistence
// - State restoration from saved data
// - State restoration error handling
// - Mixed stateful and stateless task scenarios
package microscheduler

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStatefulTaskExecution(t *testing.T) {
	executor := NewTaskExecutor(1, createTestLogger())
	defer executor.Shutdown()

	ctx := context.Background()
	initialState := &SimpleTaskState{Counter: 0, Message: "initial"}

	task := createStatefulTask("Stateful task", Background, initialState, func(resource interface{}, state TaskState, interrupt *InterruptContext, interruptChan <-chan struct{}) (interface{}, error) {
		simpleState, ok := state.(*SimpleTaskState)
		if !ok {
			return nil, fmt.Errorf("invalid state type")
		}

		// Simulate some work that modifies state
		for i := 0; i < 10; i++ {
			time.Sleep(5 * time.Millisecond)
			simpleState.Counter++
		}

		simpleState.Message = "completed"
		return fmt.Sprintf("Task completed with counter: %d", simpleState.Counter), nil
	})

	result, err := executor.Execute(ctx, task, Background)
	if err != nil {
		t.Errorf("Stateful task failed: %v", err)
	}

	expectedResult := "Task completed with counter: 10"
	if result != expectedResult {
		t.Errorf("Expected '%s', got '%v'", expectedResult, result)
	}
}

func TestStatefulTaskStateRestoration(t *testing.T) {
	executor := NewTaskExecutor(1, createTestLogger())
	defer executor.Shutdown()

	ctx := context.Background()

	// Create initial state
	initialState := &SimpleTaskState{Counter: 5, Message: "restored"}

	// Serialize the state to simulate a rescheduled task
	stateData, err := initialState.Save()
	if err != nil {
		t.Fatalf("Failed to serialize state: %v", err)
	}

	// Create a task with saved state data
	task := Task{
		Name:           "State restoration task",
		Priority:       Background,
		InitialState:   &SimpleTaskState{}, // Empty initial state
		SavedStateData: stateData,          // Previously saved state
		Compute: func(resource interface{}, state TaskState, interrupt *InterruptContext, interruptChan <-chan struct{}) (interface{}, error) {
			simpleState, ok := state.(*SimpleTaskState)
			if !ok {
				return nil, fmt.Errorf("invalid state type")
			}

			// State should be restored to Counter: 5, Message: "restored"
			simpleState.Counter += 3
			simpleState.Message = "restored and updated"

			return fmt.Sprintf("Restored counter: %d, message: %s", simpleState.Counter, simpleState.Message), nil
		},
	}

	result, err := executor.Execute(ctx, task, Background)
	if err != nil {
		t.Errorf("State restoration task failed: %v", err)
	}

	expectedResult := "Restored counter: 8, message: restored and updated"
	if result != expectedResult {
		t.Errorf("Expected '%s', got '%v'", expectedResult, result)
	}
}

func TestStatefulTaskStateRestorationError(t *testing.T) {
	executor := NewTaskExecutor(1, createTestLogger())
	defer executor.Shutdown()

	ctx := context.Background()

	// Create a task with invalid saved state data
	task := Task{
		Name:           "Invalid state restoration task",
		Priority:       Background,
		InitialState:   &SimpleTaskState{},
		SavedStateData: []byte("invalid json"), // Invalid JSON
		Compute: func(resource interface{}, state TaskState, interrupt *InterruptContext, interruptChan <-chan struct{}) (interface{}, error) {
			return "Should not reach here", nil
		},
	}

	result, err := executor.Execute(ctx, task, Background)
	if err == nil {
		t.Error("Expected error for invalid state restoration")
	}

	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}

	if !strings.Contains(err.Error(), "failed to restore state") {
		t.Errorf("Expected 'failed to restore state' error, got: %v", err)
	}
}

func TestMixedStatefulAndStatelessTasks(t *testing.T) {
	executor := NewTaskExecutor(1, createTestLogger())
	defer executor.Shutdown()

	ctx := context.Background()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var results []string

	// Stateless task
	wg.Add(1)
	go func() {
		defer wg.Done()
		task := createStatelessTask("Stateless Task", Background, 30*time.Millisecond, func(resource interface{}) (interface{}, error) {
			return "Stateless result", nil
		})

		result, err := executor.Execute(ctx, task, Background)
		if err != nil {
			t.Errorf("Stateless task failed: %v", err)
		}

		mu.Lock()
		results = append(results, fmt.Sprintf("Stateless Task: %v", result))
		mu.Unlock()
	}()

	// Stateful task
	wg.Add(1)
	go func() {
		defer wg.Done()
		initialState := &SimpleTaskState{Counter: 0, Message: "initial"}
		task := createStatefulTask("Stateful Task", Background, initialState, func(resource interface{}, state TaskState, interrupt *InterruptContext, interruptChan <-chan struct{}) (interface{}, error) {
			simpleState, ok := state.(*SimpleTaskState)
			if !ok {
				return nil, fmt.Errorf("expected SimpleTaskState, got %T", state)
			}
			simpleState.Counter = 3
			simpleState.Message = "stateful"
			return fmt.Sprintf("Stateful result: %d", simpleState.Counter), nil
		})

		result, err := executor.Execute(ctx, task, Background)
		if err != nil {
			t.Errorf("Stateful task failed: %v", err)
		}

		mu.Lock()
		results = append(results, fmt.Sprintf("Stateful Task: %v", result))
		mu.Unlock()
	}()

	wg.Wait()

	mu.Lock()
	defer mu.Unlock()

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	t.Logf("Mixed task execution results: %v", results)

	// Verify both task types completed successfully
	statelessFound := false
	statefulFound := false
	for _, result := range results {
		if strings.Contains(result, "Stateless Task: Stateless result") {
			statelessFound = true
		}
		if strings.Contains(result, "Stateful Task: Stateful result: 3") {
			statefulFound = true
		}
	}

	if !statelessFound {
		t.Error("Stateless task result not found")
	}
	if !statefulFound {
		t.Error("Stateful task result not found")
	}
}

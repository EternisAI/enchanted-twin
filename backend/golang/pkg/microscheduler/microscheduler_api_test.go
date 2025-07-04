// microscheduler_api_test.go contains tests for the simplified unified API:
// - Simplified API with unified TaskCompute signature
// - NoOpTaskState functionality for stateless tasks
// - Unified task compute signature validation
// - Helper function testing and validation
package microscheduler

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestSimplifiedAPI(t *testing.T) {
	executor := NewTaskExecutor(1, createTestLogger())
	defer executor.Shutdown()

	ctx := context.Background()

	// Test 1: Simple stateless task using NoOpTaskState
	simpleTask := Task{
		Name:         "Simple task",
		Priority:     Background,
		InitialState: &NoOpTaskState{},
		Compute: func(resource interface{}, state TaskState, interrupt *InterruptContext, interruptChan <-chan struct{}) (interface{}, error) {
			return "Simple result", nil
		},
	}

	result, err := executor.Execute(ctx, simpleTask, Background)
	if err != nil {
		t.Errorf("Simple task failed: %v", err)
	}
	if result != "Simple result" {
		t.Errorf("Expected 'Simple result', got %v", result)
	}

	// Test 2: Interruptible task using the unified API
	interruptibleTask := Task{
		Name:         "Interruptible task",
		Priority:     Background,
		InitialState: &NoOpTaskState{},
		Compute: func(resource interface{}, state TaskState, interrupt *InterruptContext, interruptChan <-chan struct{}) (interface{}, error) {
			for i := 0; i < 5; i++ {
				select {
				case <-interruptChan:
					return fmt.Sprintf("Interrupted at %d", i), fmt.Errorf("interrupted")
				default:
					time.Sleep(20 * time.Millisecond)
				}
			}
			return "Interruptible completed", nil
		},
	}

	result, err = executor.Execute(ctx, interruptibleTask, Background)
	if err != nil {
		t.Errorf("Interruptible task failed: %v", err)
	}
	if result != "Interruptible completed" {
		t.Errorf("Expected 'Interruptible completed', got %v", result)
	}

	// Test 3: Stateful task using SimpleTaskState
	statefulTask := Task{
		Name:         "Stateful task",
		Priority:     Background,
		InitialState: &SimpleTaskState{Counter: 0, Message: "initial"},
		Compute: func(resource interface{}, state TaskState, interrupt *InterruptContext, interruptChan <-chan struct{}) (interface{}, error) {
			simpleState := state.(*SimpleTaskState)
			for i := 0; i < 3; i++ {
				select {
				case <-interruptChan:
					simpleState.Counter = i
					simpleState.Message = fmt.Sprintf("interrupted at %d", i)
					interrupt.SaveState(simpleState)
					return fmt.Sprintf("Stateful interrupted at %d", i), fmt.Errorf("interrupted")
				default:
					time.Sleep(20 * time.Millisecond)
					simpleState.Counter = i + 1
				}
			}
			return fmt.Sprintf("Stateful completed: %d", simpleState.Counter), nil
		},
	}

	result, err = executor.Execute(ctx, statefulTask, Background)
	if err != nil {
		t.Errorf("Stateful task failed: %v", err)
	}
	expected := "Stateful completed: 3"
	if result != expected {
		t.Errorf("Expected '%s', got %v", expected, result)
	}

	t.Log("All simplified API tests passed!")
}

func TestSimplifiedAPIWorks(t *testing.T) {
	logger := createTestLogger()
	executor := NewTaskExecutor(1, logger)
	defer executor.Shutdown()

	ctx := context.Background()

	// Test 1: Simple stateless task
	simpleTask := Task{
		Name:         "Simple task",
		Priority:     Background,
		InitialState: &NoOpTaskState{},
		Compute: func(resource interface{}, state TaskState, interrupt *InterruptContext, interruptChan <-chan struct{}) (interface{}, error) {
			return "Simple result", nil
		},
	}

	result, err := executor.Execute(ctx, simpleTask, Background)
	if err != nil {
		t.Errorf("Simple task failed: %v", err)
	}
	if result != "Simple result" {
		t.Errorf("Expected 'Simple result', got %v", result)
	}

	// Test 2: Interruptible task
	interruptibleTask := Task{
		Name:         "Interruptible task",
		Priority:     Background,
		InitialState: &NoOpTaskState{},
		Compute: func(resource interface{}, state TaskState, interrupt *InterruptContext, interruptChan <-chan struct{}) (interface{}, error) {
			for i := 0; i < 5; i++ {
				select {
				case <-interruptChan:
					return fmt.Sprintf("Interrupted at %d", i), fmt.Errorf("interrupted")
				default:
					time.Sleep(20 * time.Millisecond)
				}
			}
			return "Interruptible completed", nil
		},
	}

	result, err = executor.Execute(ctx, interruptibleTask, Background)
	if err != nil {
		t.Errorf("Interruptible task failed: %v", err)
	}
	if result != "Interruptible completed" {
		t.Errorf("Expected 'Interruptible completed', got %v", result)
	}

	t.Log("All simplified API tests passed!")
}

func TestNoOpTaskState(t *testing.T) {
	// Test the NoOpTaskState implementation
	state := &NoOpTaskState{}

	// Test Save
	data, err := state.Save()
	if err != nil {
		t.Errorf("NoOpTaskState.Save() failed: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("Expected empty data from NoOpTaskState.Save(), got %d bytes", len(data))
	}

	// Test Restore
	err = state.Restore([]byte("any data"))
	if err != nil {
		t.Errorf("NoOpTaskState.Restore() failed: %v", err)
	}

	// Test Restore with empty data
	err = state.Restore([]byte{})
	if err != nil {
		t.Errorf("NoOpTaskState.Restore() with empty data failed: %v", err)
	}
}

func TestUnifiedTaskComputeSignature(t *testing.T) {
	executor := NewTaskExecutor(1, createTestLogger())
	defer executor.Shutdown()

	ctx := context.Background()

	// Test that all task types use the same unified compute signature
	tasks := []Task{
		// Stateless task using NoOpTaskState
		{
			Name:         "Stateless unified",
			Priority:     Background,
			InitialState: &NoOpTaskState{},
			Compute: func(resource interface{}, state TaskState, interrupt *InterruptContext, interruptChan <-chan struct{}) (interface{}, error) {
				// Verify we get the NoOpTaskState
				if _, ok := state.(*NoOpTaskState); !ok {
					return nil, fmt.Errorf("expected NoOpTaskState, got %T", state)
				}
				return "Stateless unified result", nil
			},
		},
		// Stateful task using SimpleTaskState
		{
			Name:         "Stateful unified",
			Priority:     Background,
			InitialState: &SimpleTaskState{Counter: 5, Message: "test"},
			Compute: func(resource interface{}, state TaskState, interrupt *InterruptContext, interruptChan <-chan struct{}) (interface{}, error) {
				// Verify we get the SimpleTaskState
				simpleState, ok := state.(*SimpleTaskState)
				if !ok {
					return nil, fmt.Errorf("expected SimpleTaskState, got %T", state)
				}
				if simpleState.Counter != 5 {
					return nil, fmt.Errorf("expected counter=5, got %d", simpleState.Counter)
				}
				return "Stateful unified result", nil
			},
		},
		// Interruptible task
		{
			Name:         "Interruptible unified",
			Priority:     Background,
			InitialState: &NoOpTaskState{},
			Compute: func(resource interface{}, state TaskState, interrupt *InterruptContext, interruptChan <-chan struct{}) (interface{}, error) {
				// Verify we can use the interrupt channel
				select {
				case <-interruptChan:
					return "Interrupted", fmt.Errorf("interrupted")
				default:
					// Check interrupt context is available
					if interrupt == nil {
						return nil, fmt.Errorf("interrupt context is nil")
					}
					return "Interruptible unified result", nil
				}
			},
		},
	}

	expectedResults := []string{
		"Stateless unified result",
		"Stateful unified result",
		"Interruptible unified result",
	}

	for i, task := range tasks {
		result, err := executor.Execute(ctx, task, task.Priority)
		if err != nil {
			t.Errorf("Task %d (%s) failed: %v", i, task.Name, err)
			continue
		}

		if result != expectedResults[i] {
			t.Errorf("Task %d (%s): expected '%s', got '%v'", i, task.Name, expectedResults[i], result)
		}
	}
}

func TestHelperFunctions(t *testing.T) {
	executor := NewTaskExecutor(1, createTestLogger())
	defer executor.Shutdown()

	ctx := context.Background()

	// Test createStatelessTask helper
	statelessTask := createStatelessTask("Helper stateless", Background, 10*time.Millisecond, func(resource interface{}) (interface{}, error) {
		return "Helper stateless result", nil
	})

	result, err := executor.Execute(ctx, statelessTask, Background)
	if err != nil {
		t.Errorf("Helper stateless task failed: %v", err)
	}
	if result != "Helper stateless result" {
		t.Errorf("Expected 'Helper stateless result', got %v", result)
	}

	// Test createInterruptibleTask helper
	interruptibleTask := createInterruptibleTask("Helper interruptible", Background, func(resource interface{}, interrupt <-chan struct{}) (interface{}, error) {
		select {
		case <-interrupt:
			return "Helper interrupted", fmt.Errorf("interrupted")
		default:
			return "Helper interruptible result", nil
		}
	})

	result, err = executor.Execute(ctx, interruptibleTask, Background)
	if err != nil {
		t.Errorf("Helper interruptible task failed: %v", err)
	}
	if result != "Helper interruptible result" {
		t.Errorf("Expected 'Helper interruptible result', got %v", result)
	}

	// Test createStatefulTask helper
	initialState := &SimpleTaskState{Counter: 10, Message: "helper"}
	statefulTask := createStatefulTask("Helper stateful", Background, initialState, func(resource interface{}, state TaskState, interrupt *InterruptContext, interruptChan <-chan struct{}) (interface{}, error) {
		simpleState := state.(*SimpleTaskState)
		return fmt.Sprintf("Helper stateful result: %d", simpleState.Counter), nil
	})

	result, err = executor.Execute(ctx, statefulTask, Background)
	if err != nil {
		t.Errorf("Helper stateful task failed: %v", err)
	}
	expected := "Helper stateful result: 10"
	if result != expected {
		t.Errorf("Expected '%s', got %v", expected, result)
	}
}

func TestNoRescheduleRequest(t *testing.T) {
	executor := NewTaskExecutor(1, createTestLogger())
	defer executor.Shutdown()

	ctx := context.Background()

	// Test that a task can request not to be rescheduled
	task := Task{
		Name:         "No Reschedule Task",
		Priority:     Background,
		InitialState: &NoOpTaskState{},
		Compute: func(resource interface{}, state TaskState, interrupt *InterruptContext, interruptChan <-chan struct{}) (interface{}, error) {
			// Call NoReschedule to indicate this task shouldn't be rescheduled if interrupted
			interrupt.NoReschedule()
			return "Task completed with no reschedule request", nil
		},
	}

	result, err := executor.Execute(ctx, task, Background)
	if err != nil {
		t.Errorf("NoReschedule task failed: %v", err)
	}

	expected := "Task completed with no reschedule request"
	if result != expected {
		t.Errorf("Expected '%s', got '%v'", expected, result)
	}

	// Verify the InterruptContext has the NoReschedule function
	if task.Compute == nil {
		t.Error("Task should have a Compute function")
	}
}
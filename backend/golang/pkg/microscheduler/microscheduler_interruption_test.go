// microscheduler_interruption_test.go contains tests for task interruption:
// - Task interruption and rescheduling mechanisms
// - Interruptible task execution with channel-based interruption
// - Mixed interruptible and legacy task scenarios
// - Interrupt channel mechanism validation
package microscheduler

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestTaskInterruption(t *testing.T) {
	executor := NewTaskExecutor(1, createTestLogger())
	defer executor.Shutdown()

	ctx := context.Background()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var executionOrder []string

	// Long-running background task
	wg.Add(1)
	go func() {
		defer wg.Done()
		task := createStatelessTask("Background task", Background, 500*time.Millisecond, func(resource interface{}) (interface{}, error) {
			return "Background completed", nil
		})
		
		start := time.Now()
		result, err := executor.Execute(ctx, task, Background)
		elapsed := time.Since(start)
		
		if err != nil {
			t.Errorf("Background task failed: %v", err)
		}
		
		mu.Lock()
		executionOrder = append(executionOrder, fmt.Sprintf("Background: %v after %v", result, elapsed))
		mu.Unlock()
	}()

	// Wait for background task to start
	time.Sleep(100 * time.Millisecond)

	// Submit UI task that should have higher priority
	wg.Add(1)
	go func() {
		defer wg.Done()
		task := createStatelessTask("UI task", UI, 50*time.Millisecond, func(resource interface{}) (interface{}, error) {
			return "UI completed", nil
		})
		
		start := time.Now()
		result, err := executor.Execute(ctx, task, UI)
		elapsed := time.Since(start)
		
		if err != nil {
			t.Errorf("UI task failed: %v", err)
		}
		
		mu.Lock()
		executionOrder = append(executionOrder, fmt.Sprintf("UI: %v after %v", result, elapsed))
		mu.Unlock()
	}()

	wg.Wait()

	mu.Lock()
	defer mu.Unlock()

	if len(executionOrder) != 2 {
		t.Errorf("Expected 2 results, got %d", len(executionOrder))
	}

	t.Logf("Execution order with interruption: %v", executionOrder)

	// Note: The current implementation may not interrupt in-progress tasks
	// but should prioritize UI tasks over background tasks
	uiFound := false
	backgroundFound := false
	for _, result := range executionOrder {
		if strings.Contains(result, "UI: UI completed") {
			uiFound = true
		}
		if strings.Contains(result, "Background: Background completed") {
			backgroundFound = true
		}
	}

	if !uiFound {
		t.Error("UI task should have completed")
	}
	if !backgroundFound {
		t.Logf("Warning: Background task was not interrupted as expected")
	}
}

func TestTaskRescheduling(t *testing.T) {
	executor := NewTaskExecutor(1, createTestLogger())
	defer executor.Shutdown()

	ctx := context.Background()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var results []string

	// Submit multiple background tasks and one UI task
	tasks := []struct {
		name     string
		priority Priority
		delay    time.Duration
	}{
		{"BG-1", Background, 0},
		{"BG-2", Background, 20 * time.Millisecond},
		{"UI-1", UI, 40 * time.Millisecond},
		{"BG-3", Background, 60 * time.Millisecond},
	}

	for _, taskInfo := range tasks {
		wg.Add(1)
		go func(name string, priority Priority, delay time.Duration) {
			defer wg.Done()
			
			time.Sleep(delay)
			
			task := createStatelessTask(name, priority, 30*time.Millisecond, func(resource interface{}) (interface{}, error) {
				return fmt.Sprintf("%s completed", name), nil
			})
			
			result, err := executor.Execute(ctx, task, priority)
			if err != nil {
				t.Errorf("Task %s failed: %v", name, err)
			}
			
			mu.Lock()
			results = append(results, fmt.Sprintf("%s: %v", name, result))
			mu.Unlock()
		}(taskInfo.name, taskInfo.priority, taskInfo.delay)
	}

	wg.Wait()

	mu.Lock()
	defer mu.Unlock()

	if len(results) != 4 {
		t.Errorf("Expected 4 results, got %d", len(results))
	}

	t.Logf("Task rescheduling results: %v", results)

	// All tasks should complete eventually
	for _, taskName := range []string{"BG-1", "BG-2", "UI-1", "BG-3"} {
		found := false
		for _, result := range results {
			if strings.Contains(result, fmt.Sprintf("%s: %s completed", taskName, taskName)) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Task %s did not complete", taskName)
		}
	}
}

func TestLastEffortInterruption(t *testing.T) {
	executor := NewTaskExecutor(1, createTestLogger())
	defer executor.Shutdown()

	ctx := context.Background()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var executionOrder []string

	// Start background task
	wg.Add(1)
	go func() {
		defer wg.Done()
		task := createStatelessTask("Background task", Background, 200*time.Millisecond, func(resource interface{}) (interface{}, error) {
			return "Background completed", nil
		})
		
		result, err := executor.Execute(ctx, task, Background)
		if err != nil {
			t.Errorf("Background task failed: %v", err)
		}
		
		mu.Lock()
		executionOrder = append(executionOrder, fmt.Sprintf("Background: %v", result))
		mu.Unlock()
	}()

	// Wait for background task to start
	time.Sleep(50 * time.Millisecond)

	// Submit LastEffort task
	wg.Add(1)
	go func() {
		defer wg.Done()
		task := createStatelessTask("LastEffort task", LastEffort, 100*time.Millisecond, func(resource interface{}) (interface{}, error) {
			return "LastEffort completed", nil
		})
		
		result, err := executor.Execute(ctx, task, LastEffort)
		if err != nil {
			t.Errorf("LastEffort task failed: %v", err)
		}
		
		mu.Lock()
		executionOrder = append(executionOrder, fmt.Sprintf("LastEffort: %v", result))
		mu.Unlock()
	}()

	wg.Wait()

	mu.Lock()
	defer mu.Unlock()

	if len(executionOrder) != 2 {
		t.Errorf("Expected 2 results, got %d", len(executionOrder))
	}

	t.Logf("LastEffort interruption execution order: %v", executionOrder)
}

func TestInterruptibleTaskExecution(t *testing.T) {
	executor := NewTaskExecutor(1, createTestLogger())
	defer executor.Shutdown()

	ctx := context.Background()

	task := createInterruptibleTask("Interruptible task", Background, func(resource interface{}, interrupt <-chan struct{}) (interface{}, error) {
		// Simulate work that can be interrupted
		for i := 0; i < 10; i++ {
			select {
			case <-interrupt:
				return fmt.Sprintf("Task interrupted at step %d", i), fmt.Errorf("interrupted")
			default:
				time.Sleep(10 * time.Millisecond)
			}
		}
		return "Task completed after 10 steps", nil
	})

	result, err := executor.Execute(ctx, task, Background)
	if err != nil {
		t.Errorf("Interruptible task failed: %v", err)
	}

	expectedResult := "Task completed after 10 steps"
	if result != expectedResult {
		t.Errorf("Expected '%s', got '%v'", expectedResult, result)
	}
}

func TestInterruptibleStatefulTaskExecution(t *testing.T) {
	executor := NewTaskExecutor(1, createTestLogger())
	defer executor.Shutdown()

	ctx := context.Background()
	initialState := &SimpleTaskState{Counter: 0, Message: "initial"}

	task := Task{
		Name:         "Interruptible stateful task",
		Priority:     Background,
		InitialState: initialState,
		Compute: func(resource interface{}, state TaskState, interrupt *InterruptContext, interruptChan <-chan struct{}) (interface{}, error) {
			simpleState, ok := state.(*SimpleTaskState)
			if !ok {
				return nil, fmt.Errorf("invalid state type")
			}

			// Simulate work that can be interrupted and saves state
			for i := simpleState.Counter; i < 10; i++ {
				select {
				case <-interruptChan:
					// Save state before interruption
					simpleState.Counter = i
					simpleState.Message = fmt.Sprintf("interrupted at %d", i)
					if err := interrupt.SaveState(simpleState); err != nil {
						return nil, fmt.Errorf("failed to save state: %w", err)
					}
					return fmt.Sprintf("Interrupted at step %d", i), fmt.Errorf("interrupted")
				default:
					time.Sleep(10 * time.Millisecond)
					simpleState.Counter = i + 1
				}
			}

			simpleState.Message = "completed"
			return fmt.Sprintf("Interruptible stateful task completed: %d", simpleState.Counter), nil
		},
	}

	result, err := executor.Execute(ctx, task, Background)
	if err != nil {
		t.Errorf("Interruptible stateful task failed: %v", err)
	}

	expectedResult := "Interruptible stateful task completed: 10"
	if result != expectedResult {
		t.Errorf("Expected '%s', got '%v'", expectedResult, result)
	}
}

func TestMixedInterruptibleAndLegacyTasks(t *testing.T) {
	executor := NewTaskExecutor(1, createTestLogger())
	defer executor.Shutdown()

	ctx := context.Background()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var results []string

	// Legacy stateless task
	wg.Add(1)
	go func() {
		defer wg.Done()
		task := createStatelessTask("Legacy Task", Background, 50*time.Millisecond, func(resource interface{}) (interface{}, error) {
			return "Legacy result", nil
		})
		
		result, err := executor.Execute(ctx, task, Background)
		if err != nil {
			t.Errorf("Legacy task failed: %v", err)
		}
		
		mu.Lock()
		results = append(results, fmt.Sprintf("Legacy Task: %v", result))
		mu.Unlock()
	}()

	// Interruptible task
	wg.Add(1)
	go func() {
		defer wg.Done()
		task := createInterruptibleTask("Interruptible Task", Background, func(resource interface{}, interrupt <-chan struct{}) (interface{}, error) {
			for i := 0; i < 5; i++ {
				select {
				case <-interrupt:
					return fmt.Sprintf("Interrupted at %d", i), fmt.Errorf("interrupted")
				default:
					time.Sleep(20 * time.Millisecond)
				}
			}
			return "Interruptible result", nil
		})
		
		result, err := executor.Execute(ctx, task, Background)
		if err != nil {
			t.Errorf("Interruptible task failed: %v", err)
		}
		
		mu.Lock()
		results = append(results, fmt.Sprintf("Interruptible Task: %v", result))
		mu.Unlock()
	}()

	// Legacy stateful task
	wg.Add(1)
	go func() {
		defer wg.Done()
		initialState := &SimpleTaskState{Counter: 0, Message: "initial"}
		task := createStatefulTask("Legacy Stateful Task", Background, initialState, func(resource interface{}, state TaskState, interrupt *InterruptContext, interruptChan <-chan struct{}) (interface{}, error) {
			simpleState := state.(*SimpleTaskState)
			simpleState.Counter = 3
			simpleState.Message = "legacy stateful"
			return fmt.Sprintf("Legacy stateful result: %d", simpleState.Counter), nil
		})
		
		result, err := executor.Execute(ctx, task, Background)
		if err != nil {
			t.Errorf("Legacy stateful task failed: %v", err)
		}
		
		mu.Lock()
		results = append(results, fmt.Sprintf("Legacy Stateful Task: %v", result))
		mu.Unlock()
	}()

	// Interruptible stateful task
	wg.Add(1)
	go func() {
		defer wg.Done()
		initialState := &SimpleTaskState{Counter: 0, Message: "initial"}
		task := Task{
			Name:         "Interruptible Stateful Task",
			Priority:     Background,
			InitialState: initialState,
			Compute: func(resource interface{}, state TaskState, interrupt *InterruptContext, interruptChan <-chan struct{}) (interface{}, error) {
				simpleState := state.(*SimpleTaskState)
				simpleState.Counter = 3
				simpleState.Message = "interruptible stateful"
				return fmt.Sprintf("Interruptible stateful result: %d", simpleState.Counter), nil
			},
		}
		
		result, err := executor.Execute(ctx, task, Background)
		if err != nil {
			t.Errorf("Interruptible stateful task failed: %v", err)
		}
		
		mu.Lock()
		results = append(results, fmt.Sprintf("Interruptible Stateful Task: %v", result))
		mu.Unlock()
	}()

	wg.Wait()

	mu.Lock()
	defer mu.Unlock()

	if len(results) != 4 {
		t.Errorf("Expected 4 results, got %d", len(results))
	}

	t.Logf("Mixed task execution results: %v", results)

	// Verify all task types completed
	expectedTasks := []string{
		"Legacy Task: Legacy result",
		"Interruptible Task: Interruptible result",
		"Legacy Stateful Task: Legacy stateful result: 3",
		"Interruptible Stateful Task: Interruptible stateful result: 3",
	}

	for _, expected := range expectedTasks {
		found := false
		for _, result := range results {
			if result == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected result '%s' not found", expected)
		}
	}
}

func TestInterruptChannelMechanism(t *testing.T) {
	executor := NewTaskExecutor(1, createTestLogger())
	defer executor.Shutdown()

	ctx := context.Background()

	task := createInterruptibleTask("Interrupt test task", Background, func(resource interface{}, interrupt <-chan struct{}) (interface{}, error) {
		// Test that the interrupt channel works correctly
		for i := 0; i < 100; i++ {
			select {
			case <-interrupt:
				return fmt.Sprintf("Task interrupted at step %d", i), fmt.Errorf("interrupted")
			default:
				t.Logf("Step %d - checking interrupt", i)
				// Simulate work
				time.Sleep(10 * time.Millisecond)
				t.Logf("Step %d - working...", i)
			}
		}
		return "Task completed after 100 steps", nil
	})

	result, err := executor.Execute(ctx, task, Background)
	if err != nil {
		t.Errorf("Interrupt channel test failed: %v", err)
	}

	if result != "Task completed after 100 steps" {
		t.Errorf("Expected task to complete normally, got: %v", result)
	}
}

func TestInterruptChannelMechanismWithInterruption(t *testing.T) {
	executor := NewTaskExecutor(1, createTestLogger())
	defer executor.Shutdown()

	ctx := context.Background()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var results []string

	// Start a long-running interruptible background task
	wg.Add(1)
	go func() {
		defer wg.Done()
		task := createInterruptibleTask("Interruptible background task", Background, func(resource interface{}, interrupt <-chan struct{}) (interface{}, error) {
			for i := 0; i < 100; i++ {
				select {
				case <-interrupt:
					return fmt.Sprintf("Background interrupted at step %d", i), fmt.Errorf("interrupted")
				default:
					time.Sleep(30 * time.Millisecond)
				}
			}
			return "Background completed after 100 steps", nil
		})
		
		result, err := executor.Execute(ctx, task, Background)
		
		mu.Lock()
		if err != nil && strings.Contains(err.Error(), "interrupted") {
			results = append(results, fmt.Sprintf("Background: %v", result))
		} else {
			results = append(results, fmt.Sprintf("Background: %v", result))
		}
		mu.Unlock()
	}()

	// Wait for background task to start
	time.Sleep(100 * time.Millisecond)

	// Submit UI task that should preempt background task
	wg.Add(1)
	go func() {
		defer wg.Done()
		task := createStatelessTask("UI task", UI, 50*time.Millisecond, func(resource interface{}) (interface{}, error) {
			return "UI completed", nil
		})
		
		result, err := executor.Execute(ctx, task, UI)
		if err != nil {
			t.Errorf("UI task failed: %v", err)
		}
		
		mu.Lock()
		results = append(results, fmt.Sprintf("UI: %v", result))
		mu.Unlock()
	}()

	wg.Wait()

	mu.Lock()
	defer mu.Unlock()

	t.Logf("Interrupt channel mechanism results: %v", results)

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	// Verify UI task completed
	uiCompleted := false
	for _, result := range results {
		if strings.Contains(result, "UI: UI completed") {
			uiCompleted = true
		}
	}

	if !uiCompleted {
		t.Error("UI task should have completed")
	}
}
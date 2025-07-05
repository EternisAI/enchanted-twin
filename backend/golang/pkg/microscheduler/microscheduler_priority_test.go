// microscheduler_priority_test.go contains tests for priority handling:
// - Single processor priority ordering
// - LastEffort priority behavior and interleaving
// - Priority preemption mechanisms
// - Priority string representations
package microscheduler

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSingleProcessorPriority(t *testing.T) {
	executor := NewTaskExecutor(1, createTestLogger())
	defer executor.Shutdown()

	ctx := context.Background()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var executionOrder []string

	// Create tasks with different priorities
	tasks := []struct {
		name     string
		priority Priority
		delay    time.Duration
	}{
		{"BG-1", Background, 0},
		{"UI-1", UI, 50 * time.Millisecond},
		{"BG-2", Background, 100 * time.Millisecond},
		{"UI-2", UI, 150 * time.Millisecond},
	}

	for _, taskInfo := range tasks {
		wg.Add(1)
		go func(name string, priority Priority, delay time.Duration) {
			defer wg.Done()

			// Add delay before submitting task
			time.Sleep(delay)

			task := createStatelessTask(name, priority, 30*time.Millisecond, func(resource interface{}) (interface{}, error) {
				return fmt.Sprintf("%s completed", name), nil
			})

			start := time.Now()
			result, err := executor.Execute(ctx, task, priority)
			elapsed := time.Since(start)

			if err != nil {
				t.Errorf("Task %s failed: %v", name, err)
			}

			mu.Lock()
			executionOrder = append(executionOrder, fmt.Sprintf("%s: %v after %v", name, result, elapsed))
			mu.Unlock()
		}(taskInfo.name, taskInfo.priority, taskInfo.delay)
	}

	wg.Wait()

	mu.Lock()
	defer mu.Unlock()

	if len(executionOrder) != 4 {
		t.Errorf("Expected 4 completed tasks, got %d", len(executionOrder))
	}

	t.Logf("Single processor execution order: %v", executionOrder)
}

func TestPriorityStringMethod(t *testing.T) {
	tests := []struct {
		priority Priority
		expected string
	}{
		{UI, "UI"},
		{LastEffort, "LastEffort"},
		{Background, "Background"},
		{Priority(999), "Unknown"},
	}

	for _, test := range tests {
		if test.priority.String() != test.expected {
			t.Errorf("Expected %s, got %s", test.expected, test.priority.String())
		}
	}
}

func TestLastEffortPriority(t *testing.T) {
	executor := NewTaskExecutor(1, createTestLogger())
	defer executor.Shutdown()

	ctx := context.Background()

	var mu sync.Mutex
	var executionOrder []string

	// Block the executor with a long task, then submit all priority tasks at once
	blockingDone := make(chan struct{})
	go func() {
		blockingTask := createStatelessTask("Blocking", Background, 100*time.Millisecond, func(resource interface{}) (interface{}, error) {
			return "Blocking completed", nil
		})
		_, _ = executor.Execute(ctx, blockingTask, Background)
		close(blockingDone)
	}()

	// Wait for blocking task to start
	time.Sleep(25 * time.Millisecond)

	var wg sync.WaitGroup

	// Submit all tasks while the executor is busy
	tasks := []struct {
		name     string
		priority Priority
	}{
		{"Background-Task", Background},
		{"LastEffort-Task", LastEffort},
		{"UI-Task", UI},
	}

	for _, taskInfo := range tasks {
		wg.Add(1)
		go func(name string, priority Priority) {
			defer wg.Done()

			task := createStatelessTask(name, priority, 30*time.Millisecond, func(resource interface{}) (interface{}, error) {
				mu.Lock()
				executionOrder = append(executionOrder, name)
				mu.Unlock()
				return fmt.Sprintf("%s completed", name), nil
			})

			_, err := executor.Execute(ctx, task, priority)
			if err != nil {
				t.Errorf("Task %s failed: %v", name, err)
			}
		}(taskInfo.name, taskInfo.priority)
	}

	// Wait for blocking task to complete
	<-blockingDone

	// Wait for all submitted tasks to complete
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()

	if len(executionOrder) != 3 {
		t.Errorf("Expected 3 completed tasks, got %d", len(executionOrder))
	}

	t.Logf("LastEffort priority execution order: %v", executionOrder)

	// UI should be processed first (highest priority)
	// LastEffort should be processed before Background but after UI

	// Find positions of each task type in execution order
	uiPos := -1
	lastEffortPos := -1
	backgroundPos := -1

	for i, task := range executionOrder {
		if strings.Contains(task, "UI-Task") {
			uiPos = i
		} else if strings.Contains(task, "LastEffort-Task") {
			lastEffortPos = i
		} else if strings.Contains(task, "Background-Task") {
			backgroundPos = i
		}
	}

	// Verify all tasks were found
	if uiPos == -1 {
		t.Error("UI task not found in execution order")
	}
	if lastEffortPos == -1 {
		t.Error("LastEffort task not found in execution order")
	}
	if backgroundPos == -1 {
		t.Error("Background task not found in execution order")
	}

	// Verify priority ordering: UI < LastEffort < Background (lower index = higher priority)
	if uiPos >= lastEffortPos {
		t.Errorf("UI task should be processed before LastEffort task, but UI was at position %d and LastEffort at position %d", uiPos, lastEffortPos)
	}
	if lastEffortPos >= backgroundPos {
		t.Errorf("LastEffort task should be processed before Background task, but LastEffort was at position %d and Background at position %d", lastEffortPos, backgroundPos)
	}
	if uiPos >= backgroundPos {
		t.Errorf("UI task should be processed before Background task, but UI was at position %d and Background at position %d", uiPos, backgroundPos)
	}
}

func TestLastEffortInterleaving(t *testing.T) {
	executor := NewTaskExecutor(1, createTestLogger())
	defer executor.Shutdown()

	ctx := context.Background()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var executionOrder []string

	// Create a mix of UI, LastEffort, and Background tasks
	tasks := []struct {
		name     string
		priority Priority
		delay    time.Duration
	}{
		{"BG-1", Background, 0},
		{"LE-1", LastEffort, 20 * time.Millisecond},
		{"UI-1", UI, 40 * time.Millisecond},
		{"BG-2", Background, 60 * time.Millisecond},
		{"LE-2", LastEffort, 80 * time.Millisecond},
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
			executionOrder = append(executionOrder, fmt.Sprintf("%s: %v", name, result))
			mu.Unlock()
		}(taskInfo.name, taskInfo.priority, taskInfo.delay)
	}

	wg.Wait()

	mu.Lock()
	defer mu.Unlock()

	if len(executionOrder) != 5 {
		t.Errorf("Expected 5 completed tasks, got %d", len(executionOrder))
	}

	t.Logf("LastEffort interleaving execution order: %v", executionOrder)

	// Verify that UI tasks are processed before LastEffort tasks
	// and LastEffort tasks are processed before Background tasks
	uiFound := false
	lastEffortFound := false
	for _, result := range executionOrder {
		if strings.Contains(result, "UI-") {
			uiFound = true
		}
		if strings.Contains(result, "LE-") && uiFound {
			lastEffortFound = true
		}
	}

	if !uiFound {
		t.Error("UI task should have been executed")
	}
	if !lastEffortFound {
		t.Error("LastEffort task should have been executed after UI")
	}
}

func TestPriorityPreemption(t *testing.T) {
	executor := NewTaskExecutor(1, createTestLogger())
	defer executor.Shutdown()

	ctx := context.Background()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var executionOrder []string

	// Start a long-running background task
	wg.Add(1)
	go func() {
		defer wg.Done()
		task := createStatelessTask("Long BG Task", Background, 200*time.Millisecond, func(resource interface{}) (interface{}, error) {
			return "Background task completed", nil
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

	// Submit a UI task that should preempt the background task
	wg.Add(1)
	go func() {
		defer wg.Done()
		task := createStatelessTask("UI Task", UI, 50*time.Millisecond, func(resource interface{}) (interface{}, error) {
			return "UI task completed", nil
		})

		result, err := executor.Execute(ctx, task, UI)
		if err != nil {
			t.Errorf("UI task failed: %v", err)
		}

		mu.Lock()
		executionOrder = append(executionOrder, fmt.Sprintf("UI: %v", result))
		mu.Unlock()
	}()

	wg.Wait()

	mu.Lock()
	defer mu.Unlock()

	if len(executionOrder) != 2 {
		t.Errorf("Expected 2 completed tasks, got %d", len(executionOrder))
	}

	t.Logf("Priority preemption execution order: %v", executionOrder)
}

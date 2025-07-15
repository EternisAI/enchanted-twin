// microscheduler_executor_context_test.go contains tests for context handling:
// - Context cancellation and timeout behavior
// - Orphaned task detection
// - Deadline handling
package microscheduler

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestContextCancellation(t *testing.T) {
	executor := NewTaskExecutor(2, createTestLogger())
	defer executor.Shutdown()

	ctx, cancel := context.WithCancel(context.Background())

	task := createStatelessTask("Long running task", Background, 1*time.Second, func(resource interface{}) (interface{}, error) {
		return "should not complete", nil
	})

	var wg sync.WaitGroup
	var result interface{}
	var err error

	wg.Add(1)
	go func() {
		defer wg.Done()
		result, err = executor.Execute(ctx, task, Background)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	wg.Wait()

	if err == nil {
		t.Error("Expected context cancellation error")
	}

	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}

	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}
}

func TestOrphanedTaskDetection(t *testing.T) {
	executor := NewTaskExecutor(1, createTestLogger())
	defer executor.Shutdown()

	ctx, cancel := context.WithCancel(context.Background())

	tasks := []Task{
		createStatelessTask("Task 1", Background, 200*time.Millisecond, func(resource interface{}) (interface{}, error) {
			return "Task 1 completed", nil
		}),
		createStatelessTask("Task 2 (will be orphaned)", Background, 200*time.Millisecond, func(resource interface{}) (interface{}, error) {
			return "Task 2 completed", nil
		}),
	}

	var wg sync.WaitGroup
	var results []interface{}
	var errors []error
	var mu sync.Mutex

	for _, task := range tasks {
		wg.Add(1)
		go func(t Task) {
			defer wg.Done()
			result, err := executor.Execute(ctx, t, t.Priority)
			mu.Lock()
			results = append(results, result)
			errors = append(errors, err)
			mu.Unlock()
		}(task)
	}

	time.Sleep(50 * time.Millisecond)
	cancel()

	wg.Wait()

	if len(errors) != 2 {
		t.Errorf("Expected 2 results, got %d", len(errors))
	}

	cancelledCount := 0
	for _, err := range errors {
		if err == context.Canceled {
			cancelledCount++
		}
	}

	if cancelledCount == 0 {
		t.Error("Expected at least one task to be canceled")
	}
}

func TestContextTimeout(t *testing.T) {
	executor := NewTaskExecutor(1, createTestLogger())
	defer executor.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	task := createStatelessTask("Timeout task", UI, 300*time.Millisecond, func(resource interface{}) (interface{}, error) {
		return "should timeout", nil
	})

	start := time.Now()
	result, err := executor.Execute(ctx, task, UI)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("Expected timeout error")
	}

	if err != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}

	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}

	if elapsed > 200*time.Millisecond {
		t.Errorf("Expected timeout to occur within 200ms, took %v", elapsed)
	}
}

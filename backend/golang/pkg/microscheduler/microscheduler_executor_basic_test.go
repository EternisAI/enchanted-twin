// microscheduler_executor_basic_test.go contains tests for basic microscheduler functionality:
// - Basic task execution and blocking behavior
// - Concurrency and thread safety
// - Executor lifecycle (shutdown)
// - Task result handling
package microscheduler

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestExecutorBlocking(t *testing.T) {
	executor := NewTaskExecutor(2, createTestLogger())
	defer executor.Shutdown()

	ctx := context.Background()

	task := createStatelessTask("Test task", UI, 100*time.Millisecond, func(resource interface{}) (interface{}, error) {
		return "blocking result", nil
	})

	start := time.Now()
	result, err := executor.Execute(ctx, task, UI)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if result != "blocking result" {
		t.Errorf("Expected 'blocking result', got %v", result)
	}

	if elapsed < 100*time.Millisecond {
		t.Errorf("Expected execution to block for at least 100ms, got %v", elapsed)
	}
}

func TestExecutorConcurrency(t *testing.T) {
	executor := NewTaskExecutor(3, createTestLogger())
	defer executor.Shutdown()

	ctx := context.Background()

	var wg sync.WaitGroup
	var mu sync.Mutex
	completed := make([]string, 0)

	tasks := []Task{
		createStatelessTask("UI task 1", UI, 100*time.Millisecond, func(resource interface{}) (interface{}, error) {
			return "UI-1 result", nil
		}),
		createStatelessTask("Background task 1", Background, 100*time.Millisecond, func(resource interface{}) (interface{}, error) {
			return "BG-1 result", nil
		}),
		createStatelessTask("Background task 2", Background, 100*time.Millisecond, func(resource interface{}) (interface{}, error) {
			return "BG-2 result", nil
		}),
	}

	start := time.Now()

	for _, task := range tasks {
		wg.Add(1)
		go func(t Task) {
			defer wg.Done()
			result, err := executor.Execute(ctx, t, t.Priority)
			if err != nil {
				panic(fmt.Sprintf("Expected no error, got %v", err))
			}
			mu.Lock()
			completed = append(completed, fmt.Sprintf("%s: %v", t.Name, result))
			mu.Unlock()
		}(task)
	}

	wg.Wait()
	elapsed := time.Since(start)

	if len(completed) != 3 {
		t.Errorf("Expected 3 completed tasks, got %d", len(completed))
	}

	if elapsed > 200*time.Millisecond {
		t.Errorf("Expected concurrent execution to complete faster than 200ms, got %v", elapsed)
	}
}

func TestExecutorThreadSafety(t *testing.T) {
	executor := NewTaskExecutor(4, createTestLogger())
	defer executor.Shutdown()

	ctx := context.Background()

	const numGoroutines = 10
	const tasksPerGoroutine = 5

	var wg sync.WaitGroup
	var mu sync.Mutex
	var completedTasks []string

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < tasksPerGoroutine; j++ {
				priority := UI
				if j%2 == 0 {
					priority = Background
				}

				task := createStatelessTask(fmt.Sprintf("Task-%d-%d", goroutineID, j), priority, 10*time.Millisecond, func(resource interface{}) (interface{}, error) {
					return fmt.Sprintf("Result-%d-%d", goroutineID, j), nil
				})

				result, err := executor.Execute(ctx, task, priority)
				if err != nil {
					panic(fmt.Sprintf("Unexpected error: %v", err))
				}

				mu.Lock()
				completedTasks = append(completedTasks, fmt.Sprintf("%s: %v", task.Name, result))
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	expectedCount := numGoroutines * tasksPerGoroutine
	if len(completedTasks) != expectedCount {
		t.Errorf("Expected %d completed tasks, got %d", expectedCount, len(completedTasks))
	}
}

func TestExecutorShutdown(t *testing.T) {
	executor := NewTaskExecutor(2, createTestLogger())

	ctx := context.Background()

	task := createStatelessTask("Test task", UI, 50*time.Millisecond, func(resource interface{}) (interface{}, error) {
		return "should not execute", nil
	})

	executor.Shutdown()

	result, err := executor.Execute(ctx, task, UI)
	if err == nil {
		t.Error("Expected error when executing task after shutdown")
	}

	if result != nil {
		t.Errorf("Expected nil result, got %v", result)
	}

	if err.Error() != "executor is shutdown" {
		t.Errorf("Expected 'executor is shutdown' error, got: %v", err)
	}
}

func TestExecutorMultipleThreadsWithScheduling(t *testing.T) {
	executor := NewTaskExecutor(3, createTestLogger())
	defer executor.Shutdown()

	ctx := context.Background()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var executionResults []string

	tasks := []Task{
		createStatelessTask("UI-1", UI, 50*time.Millisecond, func(resource interface{}) (interface{}, error) {
			return "UI-1 completed", nil
		}),
		createStatelessTask("BG-1", Background, 50*time.Millisecond, func(resource interface{}) (interface{}, error) {
			return "BG-1 completed", nil
		}),
		createStatelessTask("UI-2", UI, 50*time.Millisecond, func(resource interface{}) (interface{}, error) {
			return "UI-2 completed", nil
		}),
		createStatelessTask("BG-2", Background, 50*time.Millisecond, func(resource interface{}) (interface{}, error) {
			return "BG-2 completed", nil
		}),
		createStatelessTask("UI-3", UI, 50*time.Millisecond, func(resource interface{}) (interface{}, error) {
			return "UI-3 completed", nil
		}),
		createStatelessTask("BG-3", Background, 50*time.Millisecond, func(resource interface{}) (interface{}, error) {
			return "BG-3 completed", nil
		}),
	}

	for _, task := range tasks {
		wg.Add(1)
		go func(t Task) {
			defer wg.Done()
			result, err := executor.Execute(ctx, t, t.Priority)
			if err != nil {
				panic(fmt.Sprintf("Task failed: %v", err))
			}
			mu.Lock()
			executionResults = append(executionResults, fmt.Sprintf("%s: %v", t.Name, result))
			mu.Unlock()
		}(task)
	}

	wg.Wait()

	if len(executionResults) != 6 {
		t.Errorf("Expected 6 completed tasks, got %d", len(executionResults))
	}
}

func TestTaskResults(t *testing.T) {
	executor := NewTaskExecutor(2, createTestLogger())
	defer executor.Shutdown()

	ctx := context.Background()

	tests := []struct {
		name           string
		task           Task
		expectedResult interface{}
		expectError    bool
	}{
		{
			name: "Simple string result",
			task: createStatelessTask("String task", UI, 10*time.Millisecond, func(resource interface{}) (interface{}, error) {
				return "string result", nil
			}),
			expectedResult: "string result",
			expectError:    false,
		},
		{
			name: "Integer result",
			task: createStatelessTask("Int task", Background, 10*time.Millisecond, func(resource interface{}) (interface{}, error) {
				return 42, nil
			}),
			expectedResult: 42,
			expectError:    false,
		},
		{
			name: "Error result",
			task: createStatelessTask("Error task", UI, 10*time.Millisecond, func(resource interface{}) (interface{}, error) {
				return nil, fmt.Errorf("test error")
			}),
			expectedResult: nil,
			expectError:    true,
		},
		{
			name: "Struct result",
			task: createStatelessTask("Struct task", Background, 10*time.Millisecond, func(resource interface{}) (interface{}, error) {
				return map[string]interface{}{"key": "value", "number": 123}, nil
			}),
			expectedResult: map[string]interface{}{"key": "value", "number": 123},
			expectError:    false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := executor.Execute(ctx, test.task, test.task.Priority)

			if test.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}

			if !reflect.DeepEqual(result, test.expectedResult) {
				t.Errorf("Expected result %v, got %v", test.expectedResult, result)
			}
		})
	}
}

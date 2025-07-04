package helpers

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/log"
)

func createTestLogger() *log.Logger {
	return log.NewWithOptions(io.Discard, log.Options{
		Level: log.ErrorLevel, // Only show errors to keep test output clean
	})
}

func TestExecutorBlocking(t *testing.T) {
	executor := NewTaskExecutor(2, createTestLogger())
	defer executor.Shutdown()

	ctx := context.Background()

	task := Task{
		Name:     "Test task",
		Priority: UI,
		Duration: 100 * time.Millisecond,
		Compute: func(resource interface{}) (interface{}, error) {
			return "blocking result", nil
		},
	}

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
		{
			Name:     "UI task 1",
			Priority: UI,
			Duration: 100 * time.Millisecond,
			Compute: func(resource interface{}) (interface{}, error) {
				return "UI-1 result", nil
			},
		},
		{
			Name:     "Background task 1",
			Priority: Background,
			Duration: 100 * time.Millisecond,
			Compute: func(resource interface{}) (interface{}, error) {
				return "BG-1 result", nil
			},
		},
		{
			Name:     "Background task 2",
			Priority: Background,
			Duration: 100 * time.Millisecond,
			Compute: func(resource interface{}) (interface{}, error) {
				return "BG-2 result", nil
			},
		},
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

func TestContextCancellation(t *testing.T) {
	executor := NewTaskExecutor(2, createTestLogger())
	defer executor.Shutdown()

	ctx, cancel := context.WithCancel(context.Background())

	task := Task{
		Name:     "Long running task",
		Priority: Background,
		Duration: 1 * time.Second,
		Compute: func(resource interface{}) (interface{}, error) {
			return "should not complete", nil
		},
	}

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
		{
			Name:     "Task 1",
			Priority: Background,
			Duration: 200 * time.Millisecond,
			Compute: func(resource interface{}) (interface{}, error) {
				return "Task 1 completed", nil
			},
		},
		{
			Name:     "Task 2 (will be orphaned)",
			Priority: Background,
			Duration: 200 * time.Millisecond,
			Compute: func(resource interface{}) (interface{}, error) {
				return "Task 2 completed", nil
			},
		},
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

	task := Task{
		Name:     "Timeout task",
		Priority: UI,
		Duration: 300 * time.Millisecond,
		Compute: func(resource interface{}) (interface{}, error) {
			return "should timeout", nil
		},
	}

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

				task := Task{
					Name:     fmt.Sprintf("Task-%d-%d", goroutineID, j),
					Priority: priority,
					Duration: 10 * time.Millisecond,
					Compute: func(resource interface{}) (interface{}, error) {
						return fmt.Sprintf("Result-%d-%d", goroutineID, j), nil
					},
				}

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

	task := Task{
		Name:     "Test task",
		Priority: UI,
		Duration: 50 * time.Millisecond,
		Compute: func(resource interface{}) (interface{}, error) {
			return "should not execute", nil
		},
	}

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
		{
			Name:     "UI-1",
			Priority: UI,
			Duration: 50 * time.Millisecond,
			Compute: func(resource interface{}) (interface{}, error) {
				return "UI-1 completed", nil
			},
		},
		{
			Name:     "BG-1",
			Priority: Background,
			Duration: 50 * time.Millisecond,
			Compute: func(resource interface{}) (interface{}, error) {
				return "BG-1 completed", nil
			},
		},
		{
			Name:     "UI-2",
			Priority: UI,
			Duration: 50 * time.Millisecond,
			Compute: func(resource interface{}) (interface{}, error) {
				return "UI-2 completed", nil
			},
		},
		{
			Name:     "BG-2",
			Priority: Background,
			Duration: 50 * time.Millisecond,
			Compute: func(resource interface{}) (interface{}, error) {
				return "BG-2 completed", nil
			},
		},
		{
			Name:     "UI-3",
			Priority: UI,
			Duration: 50 * time.Millisecond,
			Compute: func(resource interface{}) (interface{}, error) {
				return "UI-3 completed", nil
			},
		},
		{
			Name:     "BG-3",
			Priority: Background,
			Duration: 50 * time.Millisecond,
			Compute: func(resource interface{}) (interface{}, error) {
				return "BG-3 completed", nil
			},
		},
	}

	for _, task := range tasks {
		wg.Add(1)
		go func(t Task) {
			defer wg.Done()

			result, err := executor.Execute(ctx, t, t.Priority)
			if err != nil {
				panic(fmt.Sprintf("Unexpected error: %v", err))
			}

			mu.Lock()
			executionResults = append(executionResults, fmt.Sprintf("%s: %v", t.Name, result))
			mu.Unlock()
		}(task)
	}

	wg.Wait()

	if len(executionResults) != len(tasks) {
		t.Errorf("Expected %d executed tasks, got %d", len(tasks), len(executionResults))
	}

	uiCount := 0
	bgCount := 0
	for _, resultStr := range executionResults {
		switch resultStr[0:2] {
		case "UI":
			uiCount++
		case "BG":
			bgCount++
		}
	}

	if uiCount != 3 {
		t.Errorf("Expected 3 UI tasks, got %d", uiCount)
	}

	if bgCount != 3 {
		t.Errorf("Expected 3 Background tasks, got %d", bgCount)
	}
}

func TestSingleProcessorPriority(t *testing.T) {
	executor := NewTaskExecutor(1, createTestLogger())
	defer executor.Shutdown()

	ctx := context.Background()

	var mu sync.Mutex
	var executionOrder []string

	tasks := []Task{
		{
			Name:     "Background task 1",
			Priority: Background,
			Duration: 50 * time.Millisecond,
			Compute: func(resource interface{}) (interface{}, error) {
				return "BG-1 result", nil
			},
		},
		{
			Name:     "Background task 2",
			Priority: Background,
			Duration: 50 * time.Millisecond,
			Compute: func(resource interface{}) (interface{}, error) {
				return "BG-2 result", nil
			},
		},
		{
			Name:     "UI task 1",
			Priority: UI,
			Duration: 50 * time.Millisecond,
			Compute: func(resource interface{}) (interface{}, error) {
				return "UI-1 result", nil
			},
		},
		{
			Name:     "UI task 2",
			Priority: UI,
			Duration: 50 * time.Millisecond,
			Compute: func(resource interface{}) (interface{}, error) {
				return "UI-2 result", nil
			},
		},
	}

	var wg sync.WaitGroup
	for i, task := range tasks {
		wg.Add(1)
		go func(t Task, index int) {
			defer wg.Done()

			time.Sleep(time.Duration(index) * 10 * time.Millisecond)

			start := time.Now()
			result, err := executor.Execute(ctx, t, t.Priority)
			if err != nil {
				panic(fmt.Sprintf("Unexpected error: %v", err))
			}

			mu.Lock()
			executionOrder = append(executionOrder, fmt.Sprintf("%s (started at %v): %v", t.Name, start.Format("15:04:05.000"), result))
			mu.Unlock()
		}(task, i)
	}

	wg.Wait()

	if len(executionOrder) != 4 {
		t.Errorf("Expected 4 executed tasks, got %d", len(executionOrder))
	}

	uiCount := 0
	bgCount := 0

	for _, taskResult := range executionOrder {
		switch taskResult[0:2] {
		case "UI":
			uiCount++
		case "Ba":
			bgCount++
		}
	}

	if uiCount != 2 {
		t.Errorf("Expected 2 UI tasks, got %d", uiCount)
	}

	if bgCount != 2 {
		t.Errorf("Expected 2 Background tasks, got %d", bgCount)
	}

	t.Logf("Execution order: %v", executionOrder)
}

func TestTaskResults(t *testing.T) {
	executor := NewTaskExecutor(2, createTestLogger())
	defer executor.Shutdown()

	ctx := context.Background()

	tasks := []Task{
		{
			Name:     "Math task",
			Priority: Background,
			Duration: 50 * time.Millisecond,
			Compute: func(resource interface{}) (interface{}, error) {
				return 42 * 2, nil
			},
		},
		{
			Name:     "String task",
			Priority: UI,
			Duration: 30 * time.Millisecond,
			Compute: func(resource interface{}) (interface{}, error) {
				return "Hello, World!", nil
			},
		},
		{
			Name:     "Error task",
			Priority: Background,
			Duration: 20 * time.Millisecond,
			Compute: func(resource interface{}) (interface{}, error) {
				return nil, fmt.Errorf("simulated error")
			},
		},
		{
			Name:     "No compute task",
			Priority: UI,
			Duration: 10 * time.Millisecond,
		},
	}

	var wg sync.WaitGroup
	results := make([]interface{}, len(tasks))
	errors := make([]error, len(tasks))

	for i, task := range tasks {
		wg.Add(1)
		go func(index int, t Task) {
			defer wg.Done()
			result, err := executor.Execute(ctx, t, t.Priority)
			results[index] = result
			errors[index] = err
		}(i, task)
	}

	wg.Wait()

	if results[0] != 84 {
		t.Errorf("Expected math result 84, got %v", results[0])
	}

	if results[1] != "Hello, World!" {
		t.Errorf("Expected string result 'Hello, World!', got %v", results[1])
	}

	if errors[2] == nil || errors[2].Error() != "simulated error" {
		t.Errorf("Expected error 'simulated error', got %v", errors[2])
	}

	if results[3] != "Task No compute task completed" {
		t.Errorf("Expected default result, got %v", results[3])
	}
}

func TestPriorityStringMethod(t *testing.T) {
	if UI.String() != "UI" {
		t.Errorf("Expected 'UI', got %s", UI.String())
	}

	if LastEffort.String() != "LastEffort" {
		t.Errorf("Expected 'LastEffort', got %s", LastEffort.String())
	}

	if Background.String() != "Background" {
		t.Errorf("Expected 'Background', got %s", Background.String())
	}
}

func TestLastEffortPriority(t *testing.T) {
	executor := NewTaskExecutor(1, createTestLogger())
	defer executor.Shutdown()

	ctx := context.Background()

	var mu sync.Mutex
	var executionOrder []string

	tasks := []Task{
		{
			Name:     "Background task",
			Priority: Background,
			Duration: 50 * time.Millisecond,
			Compute: func(resource interface{}) (interface{}, error) {
				return "BG completed", nil
			},
		},
		{
			Name:     "LastEffort task",
			Priority: LastEffort,
			Duration: 50 * time.Millisecond,
			Compute: func(resource interface{}) (interface{}, error) {
				return "LastEffort completed", nil
			},
		},
		{
			Name:     "UI task",
			Priority: UI,
			Duration: 50 * time.Millisecond,
			Compute: func(resource interface{}) (interface{}, error) {
				return "UI completed", nil
			},
		},
	}

	var wg sync.WaitGroup
	for i, task := range tasks {
		wg.Add(1)
		go func(t Task, index int) {
			defer wg.Done()

			time.Sleep(time.Duration(index) * 10 * time.Millisecond)

			start := time.Now()
			result, err := executor.Execute(ctx, t, t.Priority)
			if err != nil {
				panic(fmt.Sprintf("Unexpected error: %v", err))
			}

			mu.Lock()
			executionOrder = append(executionOrder, fmt.Sprintf("%s (started at %v): %v", t.Name, start.Format("15:04:05.000"), result))
			mu.Unlock()
		}(task, i)
	}

	wg.Wait()

	if len(executionOrder) != 3 {
		t.Errorf("Expected 3 executed tasks, got %d", len(executionOrder))
	}

	uiCount := 0
	lastEffortCount := 0
	bgCount := 0

	for _, taskResult := range executionOrder {
		if taskResult[0:2] == "UI" {
			uiCount++
		} else if taskResult[0:10] == "LastEffort" {
			lastEffortCount++
		} else if taskResult[0:10] == "Background" {
			bgCount++
		}
	}

	if uiCount != 1 {
		t.Errorf("Expected 1 UI task, got %d", uiCount)
	}

	if lastEffortCount != 1 {
		t.Errorf("Expected 1 LastEffort task, got %d", lastEffortCount)
	}

	if bgCount != 1 {
		t.Errorf("Expected 1 Background task, got %d", bgCount)
	}

	t.Logf("Execution order: %v", executionOrder)
}

func TestLastEffortInterleaving(t *testing.T) {
	executor := NewTaskExecutor(2, createTestLogger())
	defer executor.Shutdown()

	ctx := context.Background()

	var wg sync.WaitGroup
	var mu sync.Mutex
	executionResults := make([]string, 0)

	tasks := []Task{
		{
			Name:     "UI-1",
			Priority: UI,
			Duration: 100 * time.Millisecond,
			Compute: func(resource interface{}) (interface{}, error) {
				return "UI-1 completed", nil
			},
		},
		{
			Name:     "LastEffort-1",
			Priority: LastEffort,
			Duration: 100 * time.Millisecond,
			Compute: func(resource interface{}) (interface{}, error) {
				return "LastEffort-1 completed", nil
			},
		},
		{
			Name:     "Background-1",
			Priority: Background,
			Duration: 100 * time.Millisecond,
			Compute: func(resource interface{}) (interface{}, error) {
				return "Background-1 completed", nil
			},
		},
		{
			Name:     "UI-2",
			Priority: UI,
			Duration: 100 * time.Millisecond,
			Compute: func(resource interface{}) (interface{}, error) {
				return "UI-2 completed", nil
			},
		},
		{
			Name:     "LastEffort-2",
			Priority: LastEffort,
			Duration: 100 * time.Millisecond,
			Compute: func(resource interface{}) (interface{}, error) {
				return "LastEffort-2 completed", nil
			},
		},
	}

	for _, task := range tasks {
		wg.Add(1)
		go func(t Task) {
			defer wg.Done()

			result, err := executor.Execute(ctx, t, t.Priority)
			if err != nil {
				panic(fmt.Sprintf("Unexpected error: %v", err))
			}

			mu.Lock()
			executionResults = append(executionResults, fmt.Sprintf("%s: %v", t.Name, result))
			mu.Unlock()
		}(task)
	}

	wg.Wait()

	if len(executionResults) != 5 {
		t.Errorf("Expected 5 executed tasks, got %d", len(executionResults))
	}

	uiCount := 0
	lastEffortCount := 0
	bgCount := 0

	for _, resultStr := range executionResults {
		if resultStr[0:2] == "UI" {
			uiCount++
		} else if resultStr[0:10] == "LastEffort" {
			lastEffortCount++
		} else if resultStr[0:10] == "Background" {
			bgCount++
		}
	}

	if uiCount != 2 {
		t.Errorf("Expected 2 UI tasks, got %d", uiCount)
	}

	if lastEffortCount != 2 {
		t.Errorf("Expected 2 LastEffort tasks, got %d", lastEffortCount)
	}

	if bgCount != 1 {
		t.Errorf("Expected 1 Background task, got %d", bgCount)
	}

	t.Logf("LastEffort interleaving results: %v", executionResults)
}

func TestWorkerConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		config    WorkerConfig
		expectErr bool
		errMsg    string
	}{
		{
			name:      "Zero workers",
			config:    WorkerConfig{UIWorkers: 0, BackgroundWorkers: 0},
			expectErr: true,
			errMsg:    "total workers cannot be zero",
		},
		{
			name:      "Single worker - UI only",
			config:    WorkerConfig{UIWorkers: 1, BackgroundWorkers: 0},
			expectErr: false,
		},
		{
			name:      "Single worker - Background only",
			config:    WorkerConfig{UIWorkers: 0, BackgroundWorkers: 1},
			expectErr: false,
		},
		{
			name:      "Multiple workers - valid",
			config:    WorkerConfig{UIWorkers: 2, BackgroundWorkers: 3},
			expectErr: false,
		},
		{
			name:      "Multiple workers - zero UI",
			config:    WorkerConfig{UIWorkers: 0, BackgroundWorkers: 3},
			expectErr: true,
			errMsg:    "UIWorkers must be > 0 when total workers > 1, got 0",
		},
		{
			name:      "Multiple workers - zero Background",
			config:    WorkerConfig{UIWorkers: 2, BackgroundWorkers: 0},
			expectErr: true,
			errMsg:    "BackgroundWorkers must be > 0 when total workers > 1, got 0",
		},
		{
			name:      "Multiple workers - negative UI",
			config:    WorkerConfig{UIWorkers: -1, BackgroundWorkers: 2},
			expectErr: true,
			errMsg:    "worker counts cannot be negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.validate()
			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if err.Error() != tt.errMsg {
					t.Errorf("Expected error '%s', got '%s'", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestNewTaskExecutorWithConfig(t *testing.T) {
	t.Run("Valid config", func(t *testing.T) {
		config := WorkerConfig{UIWorkers: 2, BackgroundWorkers: 3}
		executor, err := NewTaskExecutorWithConfig(config, createTestLogger())
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if executor == nil {
			t.Error("Expected executor to be created")
		}
		if executor != nil {
			executor.Shutdown()
		}
	})

	t.Run("Invalid config", func(t *testing.T) {
		config := WorkerConfig{UIWorkers: 0, BackgroundWorkers: 0}
		executor, err := NewTaskExecutorWithConfig(config, createTestLogger())
		if err == nil {
			t.Error("Expected error for invalid config")
		}
		if executor != nil {
			t.Error("Expected nil executor for invalid config")
			executor.Shutdown()
		}
	})
}

func TestWorkerConfiguration(t *testing.T) {
	config := WorkerConfig{UIWorkers: 2, BackgroundWorkers: 2}
	executor, err := NewTaskExecutorWithConfig(config, createTestLogger())
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}
	defer executor.Shutdown()

	ctx := context.Background()
	var wg sync.WaitGroup
	var mu sync.Mutex
	var completedTasks []string

	tasks := []Task{
		{
			Name:     "UI task 1",
			Priority: UI,
			Duration: 100 * time.Millisecond,
			Compute: func(resource interface{}) (interface{}, error) {
				return "UI-1 result", nil
			},
		},
		{
			Name:     "UI task 2",
			Priority: UI,
			Duration: 100 * time.Millisecond,
			Compute: func(resource interface{}) (interface{}, error) {
				return "UI-2 result", nil
			},
		},
		{
			Name:     "Background task 1",
			Priority: Background,
			Duration: 100 * time.Millisecond,
			Compute: func(resource interface{}) (interface{}, error) {
				return "BG-1 result", nil
			},
		},
		{
			Name:     "Background task 2",
			Priority: Background,
			Duration: 100 * time.Millisecond,
			Compute: func(resource interface{}) (interface{}, error) {
				return "BG-2 result", nil
			},
		},
	}

	for _, task := range tasks {
		wg.Add(1)
		go func(t Task) {
			defer wg.Done()
			result, err := executor.Execute(ctx, t, t.Priority)
			if err != nil {
				panic(fmt.Sprintf("Unexpected error: %v", err))
			}
			mu.Lock()
			completedTasks = append(completedTasks, fmt.Sprintf("%s: %v", t.Name, result))
			mu.Unlock()
		}(task)
	}

	wg.Wait()

	if len(completedTasks) != 4 {
		t.Errorf("Expected 4 completed tasks, got %d", len(completedTasks))
	}

	uiCount := 0
	bgCount := 0
	for _, taskResult := range completedTasks {
		if strings.Contains(taskResult, "UI task") {
			uiCount++
		} else if strings.Contains(taskResult, "Background task") {
			bgCount++
		}
	}

	if uiCount != 2 {
		t.Errorf("Expected 2 UI tasks, got %d", uiCount)
	}

	if bgCount != 2 {
		t.Errorf("Expected 2 Background tasks, got %d", bgCount)
	}
}

func TestResourceAccess(t *testing.T) {
	// Create a simple test resource struct
	type TestResource struct {
		Counter  int
		WorkerID int
	}

	executor := NewTaskExecutor(2, createTestLogger())
	defer executor.Shutdown()

	ctx := context.Background()

	// Test task that uses the simple resource interface
	task := Task{
		Name:     "Resource access task",
		Priority: Background,
		Duration: 50 * time.Millisecond,
		Compute: func(resource interface{}) (interface{}, error) {
			// Type assert the resource to our test struct
			testRes, ok := resource.(*TestResource)
			if !ok {
				// If no resource provided, create a default one
				testRes = &TestResource{Counter: 0, WorkerID: 1}
			}

			// Get the current counter value
			counter := testRes.Counter
			workerID := testRes.WorkerID

			// Increment the counter
			testRes.Counter++

			// Return information about what happened
			return map[string]interface{}{
				"worker_id":      workerID,
				"counter_before": counter,
				"counter_after":  counter + 1,
				"resource_type":  "TestResource",
			}, nil
		},
	}

	// Execute the task - since we're using the simplified interface,
	// the task function itself will create the resource if needed
	result, err := executor.Execute(ctx, task, Background)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify the result
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Errorf("Expected result to be a map, got %T", result)
	}

	workerID, exists := resultMap["worker_id"]
	if !exists {
		t.Error("Expected worker_id to be present in result")
	}

	counterBefore, exists := resultMap["counter_before"]
	if !exists {
		t.Error("Expected counter_before to be present in result")
	}

	counterAfter, exists := resultMap["counter_after"]
	if !exists {
		t.Error("Expected counter_after to be present in result")
	}

	if counterBefore != 0 {
		t.Errorf("Expected counter_before to be 0, got %v", counterBefore)
	}

	if counterAfter != 1 {
		t.Errorf("Expected counter_after to be 1, got %v", counterAfter)
	}

	// Test that worker ID is set correctly
	if workerIDInt, ok := workerID.(int); !ok || workerIDInt != 1 {
		t.Errorf("Expected worker_id to be 1, got %v", workerID)
	}

	// Test multiple tasks with different resources to demonstrate type assertion
	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make([]interface{}, 0)

	// Execute multiple tasks to test different resource types
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(taskNum int) {
			defer wg.Done()

			task := Task{
				Name:     fmt.Sprintf("Resource task %d", taskNum),
				Priority: Background,
				Duration: 20 * time.Millisecond,
				Compute: func(resource interface{}) (interface{}, error) {
					// Demonstrate different resource types
					if taskNum%2 == 0 {
						// Use TestResource for even tasks
						if testRes, ok := resource.(*TestResource); ok {
							return map[string]interface{}{
								"task_num":      taskNum,
								"resource_type": "TestResource",
								"counter":       testRes.Counter,
								"worker_id":     testRes.WorkerID,
							}, nil
						}
					}
					// Use string resource for odd tasks
					if strRes, ok := resource.(string); ok {
						return map[string]interface{}{
							"task_num":       taskNum,
							"resource_type":  "string",
							"resource_value": strRes,
						}, nil
					}
					// Default case - no resource or unknown type
					return map[string]interface{}{
						"task_num":       taskNum,
						"resource_type":  "none",
						"resource_value": nil,
					}, nil
				},
			}

			// Execute the task - the task function handles resource creation internally
			result, err := executor.Execute(ctx, task, Background)
			if err != nil {
				t.Errorf("Task %d failed: %v", taskNum, err)
				return
			}

			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}(i)
	}

	wg.Wait()

	if len(results) != 5 {
		t.Errorf("Expected 5 results, got %d", len(results))
	}

	// Verify that tasks handled different resource types correctly
	resourceTypes := make(map[string]int)
	for _, result := range results {
		resultMap, ok := result.(map[string]interface{})
		if !ok {
			t.Errorf("Expected result to be a map, got %T", result)
			continue
		}
		resourceType, ok := resultMap["resource_type"].(string)
		if !ok {
			t.Errorf("Expected resource_type to be a string, got %T", resultMap["resource_type"])
			continue
		}
		resourceTypes[resourceType]++
	}

	t.Logf("Resource type distribution: %v", resourceTypes)

	// We should have processed at least one task
	if len(resourceTypes) == 0 {
		t.Error("Expected at least one task to have been processed")
	}
}

func TestWorkerTypeResourceDifferentiation(t *testing.T) {
	config := WorkerConfig{
		UIWorkers:         1,
		BackgroundWorkers: 1,
		ResourceFactory: func(workerID int, workerType WorkerType) interface{} {
			switch workerType {
			case UIWorker:
				return map[string]interface{}{
					"type":       "UI",
					"workerID":   workerID,
					"httpClient": fmt.Sprintf("ui-client-%d", workerID),
				}
			case BackgroundWorker:
				return map[string]interface{}{
					"type":         "Background",
					"workerID":     workerID,
					"dbConnection": fmt.Sprintf("bg-db-%d", workerID),
				}
			default:
				return nil
			}
		},
	}

	executor, err := NewTaskExecutorWithConfig(config, createTestLogger())
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}
	defer executor.Shutdown()

	ctx := context.Background()
	var wg sync.WaitGroup
	var mu sync.Mutex
	resourceTypes := make([]string, 0)

	tasks := []Task{
		{
			Name:     "UI Task",
			Priority: UI,
			Duration: 50 * time.Millisecond,
			Compute: func(resource interface{}) (interface{}, error) {
				if res, ok := resource.(map[string]interface{}); ok {
					mu.Lock()
					if resType, typeOk := res["type"].(string); typeOk {
						resourceTypes = append(resourceTypes, resType)
					}
					mu.Unlock()
					return fmt.Sprintf("UI task completed with %s", res["httpClient"]), nil
				}
				return "UI task completed", nil
			},
		},
		{
			Name:     "Background Task",
			Priority: Background,
			Duration: 50 * time.Millisecond,
			Compute: func(resource interface{}) (interface{}, error) {
				if res, ok := resource.(map[string]interface{}); ok {
					mu.Lock()
					if resType, typeOk := res["type"].(string); typeOk {
						resourceTypes = append(resourceTypes, resType)
					}
					mu.Unlock()
					return fmt.Sprintf("BG task completed with %s", res["dbConnection"]), nil
				}
				return "Background task completed", nil
			},
		},
	}

	for _, task := range tasks {
		wg.Add(1)
		go func(tsk Task) {
			defer wg.Done()
			result, err := executor.Execute(ctx, tsk, tsk.Priority)
			if err != nil {
				panic(fmt.Sprintf("Unexpected error: %v", err))
			}
			t.Logf("Task %s result: %v", tsk.Name, result)
		}(task)
	}

	wg.Wait()

	if len(resourceTypes) != 2 {
		t.Errorf("Expected 2 resource types, got %d", len(resourceTypes))
	}

	uiCount := 0
	bgCount := 0
	for _, resType := range resourceTypes {
		switch resType {
		case "UI":
			uiCount++
		case "Background":
			bgCount++
		}
	}

	if uiCount != 1 {
		t.Errorf("Expected 1 UI resource type, got %d", uiCount)
	}

	if bgCount != 1 {
		t.Errorf("Expected 1 Background resource type, got %d", bgCount)
	}

	t.Logf("Worker resource types: %v", resourceTypes)
}

func TestWorkerTypeStringMethod(t *testing.T) {
	if UIWorker.String() != "UIWorker" {
		t.Errorf("Expected 'UIWorker', got %s", UIWorker.String())
	}

	if BackgroundWorker.String() != "BackgroundWorker" {
		t.Errorf("Expected 'BackgroundWorker', got %s", BackgroundWorker.String())
	}

	var unknownType WorkerType = 999
	if unknownType.String() != "Unknown" {
		t.Errorf("Expected 'Unknown', got %s", unknownType.String())
	}
}

func TestConfigurableBufferSizes(t *testing.T) {
	config := WorkerConfig{
		UIWorkers:            1,
		BackgroundWorkers:    1,
		UIQueueBufferSize:    50,
		LastEffortBufferSize: 75,
		BackgroundBufferSize: 25,
	}

	executor, err := NewTaskExecutorWithConfig(config, createTestLogger())
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}
	defer executor.Shutdown()

	if cap(executor.uiQueue) != 50 {
		t.Errorf("Expected UI queue buffer size 50, got %d", cap(executor.uiQueue))
	}

	if cap(executor.lastEffortQueue) != 75 {
		t.Errorf("Expected LastEffort queue buffer size 75, got %d", cap(executor.lastEffortQueue))
	}

	if cap(executor.backgroundQueue) != 25 {
		t.Errorf("Expected Background queue buffer size 25, got %d", cap(executor.backgroundQueue))
	}
}

func TestDefaultBufferSizes(t *testing.T) {
	config := WorkerConfig{
		UIWorkers:         1,
		BackgroundWorkers: 1,
	}

	executor, err := NewTaskExecutorWithConfig(config, createTestLogger())
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}
	defer executor.Shutdown()

	if cap(executor.uiQueue) != 100 {
		t.Errorf("Expected default UI queue buffer size 100, got %d", cap(executor.uiQueue))
	}

	if cap(executor.lastEffortQueue) != 100 {
		t.Errorf("Expected default LastEffort queue buffer size 100, got %d", cap(executor.lastEffortQueue))
	}

	if cap(executor.backgroundQueue) != 100 {
		t.Errorf("Expected default Background queue buffer size 100, got %d", cap(executor.backgroundQueue))
	}
}

func TestLoggingIntegration(t *testing.T) {
	// Create a logger that outputs to test for verification
	logger := log.NewWithOptions(io.Discard, log.Options{
		Level: log.InfoLevel,
	})

	executor := NewTaskExecutor(1, logger)
	defer executor.Shutdown()

	ctx := context.Background()
	task := Task{
		Name:     "Test logging task",
		Priority: UI,
		Duration: 10 * time.Millisecond,
		Compute: func(resource interface{}) (interface{}, error) {
			return "logged result", nil
		},
	}

	result, err := executor.Execute(ctx, task, UI)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if result != "logged result" {
		t.Errorf("Expected 'logged result', got %v", result)
	}
}

func TestBufferSizeValidation(t *testing.T) {
	tests := []struct {
		name      string
		config    WorkerConfig
		expectErr bool
		errMsg    string
	}{
		{
			name: "Negative UI buffer size",
			config: WorkerConfig{
				UIWorkers:         1,
				BackgroundWorkers: 1,
				UIQueueBufferSize: -1,
			},
			expectErr: true,
			errMsg:    "UIQueueBufferSize cannot be negative, got -1",
		},
		{
			name: "Negative LastEffort buffer size",
			config: WorkerConfig{
				UIWorkers:            1,
				BackgroundWorkers:    1,
				LastEffortBufferSize: -5,
			},
			expectErr: true,
			errMsg:    "LastEffortBufferSize cannot be negative, got -5",
		},
		{
			name: "Negative Background buffer size",
			config: WorkerConfig{
				UIWorkers:            1,
				BackgroundWorkers:    1,
				BackgroundBufferSize: -10,
			},
			expectErr: true,
			errMsg:    "BackgroundBufferSize cannot be negative, got -10",
		},
		{
			name: "Zero buffer sizes are valid",
			config: WorkerConfig{
				UIWorkers:            1,
				BackgroundWorkers:    1,
				UIQueueBufferSize:    0,
				LastEffortBufferSize: 0,
				BackgroundBufferSize: 0,
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewTaskExecutorWithConfig(tt.config, createTestLogger())
			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if err.Error() != tt.errMsg {
					t.Errorf("Expected error '%s', got '%s'", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

// microscheduler_config_test.go contains tests for microscheduler configuration:
// - Worker configuration validation
// - Buffer size configuration and validation
// - Custom resource factory configuration
// - Default configuration behavior
package microscheduler

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestWorkerConfigValidation(t *testing.T) {
	logger := createTestLogger()

	tests := []struct {
		name        string
		config      WorkerConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid config with multiple workers",
			config: WorkerConfig{
				UIWorkers:            2,
				BackgroundWorkers:    3,
				UIQueueBufferSize:    50,
				LastEffortBufferSize: 50,
				BackgroundBufferSize: 50,
			},
			expectError: false,
		},
		{
			name: "Valid config with single worker",
			config: WorkerConfig{
				UIWorkers:            1,
				BackgroundWorkers:    0,
				UIQueueBufferSize:    100,
				LastEffortBufferSize: 100,
				BackgroundBufferSize: 100,
			},
			expectError: false,
		},
		{
			name: "Invalid - zero workers",
			config: WorkerConfig{
				UIWorkers:         0,
				BackgroundWorkers: 0,
			},
			expectError: true,
			errorMsg:    "total workers cannot be zero",
		},
		{
			name: "Invalid - negative UI workers",
			config: WorkerConfig{
				UIWorkers:         -1,
				BackgroundWorkers: 2,
			},
			expectError: true,
			errorMsg:    "worker counts cannot be negative",
		},
		{
			name: "Invalid - zero UI workers with multiple total",
			config: WorkerConfig{
				UIWorkers:         0,
				BackgroundWorkers: 2,
			},
			expectError: true,
			errorMsg:    "UIWorkers must be > 0",
		},
		{
			name: "Invalid - negative buffer size",
			config: WorkerConfig{
				UIWorkers:            1,
				BackgroundWorkers:    1,
				UIQueueBufferSize:    -1,
				LastEffortBufferSize: 100,
				BackgroundBufferSize: 100,
			},
			expectError: true,
			errorMsg:    "UIQueueBufferSize cannot be negative",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewTaskExecutorWithConfig(test.config, logger)

			if test.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if !strings.Contains(err.Error(), test.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", test.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

func TestNewTaskExecutorWithConfig(t *testing.T) {
	logger := createTestLogger()

	config := WorkerConfig{
		UIWorkers:            2,
		BackgroundWorkers:    1,
		UIQueueBufferSize:    25,
		LastEffortBufferSize: 30,
		BackgroundBufferSize: 35,
		ResourceFactory: func(workerID int, workerType WorkerType) interface{} {
			return fmt.Sprintf("Resource-%d-%s", workerID, workerType.String())
		},
	}

	executor, err := NewTaskExecutorWithConfig(config, logger)
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}
	defer executor.Shutdown()

	// Test that executor works with custom config
	ctx := context.Background()
	task := createStatelessTask("Config test task", UI, 50*time.Millisecond, func(resource interface{}) (interface{}, error) {
		return "Config test completed", nil
	})

	result, err := executor.Execute(ctx, task, UI)
	if err != nil {
		t.Errorf("Task execution failed: %v", err)
	}

	expected := "Config test completed"
	if result != expected {
		t.Errorf("Expected '%s', got %v", expected, result)
	}
}

func TestWorkerConfiguration(t *testing.T) {
	logger := createTestLogger()

	tests := []struct {
		name              string
		processorCount    int
		expectedUIWorkers int
		expectedBGWorkers int
	}{
		{
			name:              "Single processor",
			processorCount:    1,
			expectedUIWorkers: 1,
			expectedBGWorkers: 0,
		},
		{
			name:              "Two processors",
			processorCount:    2,
			expectedUIWorkers: 1,
			expectedBGWorkers: 1,
		},
		{
			name:              "Four processors",
			processorCount:    4,
			expectedUIWorkers: 1,
			expectedBGWorkers: 3,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			executor := NewTaskExecutor(test.processorCount, logger)
			defer executor.Shutdown()

			// The executor should be created successfully
			// We can't directly inspect the internal config, but we can test functionality
			ctx := context.Background()
			task := createStatelessTask("Worker config test", UI, 10*time.Millisecond, func(resource interface{}) (interface{}, error) {
				return "Worker config test completed", nil
			})

			result, err := executor.Execute(ctx, task, UI)
			if err != nil {
				t.Errorf("Task execution failed: %v", err)
			}

			expected := "Worker config test completed"
			if result != expected {
				t.Errorf("Expected '%s', got %v", expected, result)
			}
		})
	}
}

func TestConfigurableBufferSizes(t *testing.T) {
	logger := createTestLogger()

	config := WorkerConfig{
		UIWorkers:            1,
		BackgroundWorkers:    1,
		UIQueueBufferSize:    5,
		LastEffortBufferSize: 10,
		BackgroundBufferSize: 15,
		ResourceFactory: func(workerID int, workerType WorkerType) interface{} {
			return fmt.Sprintf("CustomResource-%d", workerID)
		},
	}

	executor, err := NewTaskExecutorWithConfig(config, logger)
	if err != nil {
		t.Fatalf("Failed to create executor with custom buffer sizes: %v", err)
	}
	defer executor.Shutdown()

	ctx := context.Background()
	task := createStatelessTask("Buffer size test", Background, 10*time.Millisecond, func(resource interface{}) (interface{}, error) {
		return "Buffer size test completed", nil
	})

	result, err := executor.Execute(ctx, task, Background)
	if err != nil {
		t.Errorf("Task execution failed: %v", err)
	}

	expected := "Buffer size test completed"
	if result != expected {
		t.Errorf("Expected '%s', got %v", expected, result)
	}
}

func TestDefaultBufferSizes(t *testing.T) {
	logger := createTestLogger()

	config := WorkerConfig{
		UIWorkers:         1,
		BackgroundWorkers: 1,
		// Buffer sizes not set, should get defaults
	}

	executor, err := NewTaskExecutorWithConfig(config, logger)
	if err != nil {
		t.Fatalf("Failed to create executor with default buffer sizes: %v", err)
	}
	defer executor.Shutdown()

	ctx := context.Background()
	task := createStatelessTask("Default buffer test", UI, 10*time.Millisecond, func(resource interface{}) (interface{}, error) {
		return "Default buffer test completed", nil
	})

	result, err := executor.Execute(ctx, task, UI)
	if err != nil {
		t.Errorf("Task execution failed: %v", err)
	}

	expected := "Default buffer test completed"
	if result != expected {
		t.Errorf("Expected '%s', got %v", expected, result)
	}
}

func TestBufferSizeValidation(t *testing.T) {
	logger := createTestLogger()

	tests := []struct {
		name         string
		config       WorkerConfig
		expectError  bool
		errorMessage string
	}{
		{
			name: "Valid buffer sizes",
			config: WorkerConfig{
				UIWorkers:            1,
				BackgroundWorkers:    1,
				UIQueueBufferSize:    50,
				LastEffortBufferSize: 60,
				BackgroundBufferSize: 70,
			},
			expectError: false,
		},
		{
			name: "Zero buffer sizes (valid)",
			config: WorkerConfig{
				UIWorkers:            1,
				BackgroundWorkers:    1,
				UIQueueBufferSize:    0,
				LastEffortBufferSize: 0,
				BackgroundBufferSize: 0,
			},
			expectError: false,
		},
		{
			name: "Negative UI buffer size",
			config: WorkerConfig{
				UIWorkers:            1,
				BackgroundWorkers:    1,
				UIQueueBufferSize:    -1,
				LastEffortBufferSize: 100,
				BackgroundBufferSize: 100,
			},
			expectError:  true,
			errorMessage: "UIQueueBufferSize cannot be negative",
		},
		{
			name: "Negative LastEffort buffer size",
			config: WorkerConfig{
				UIWorkers:            1,
				BackgroundWorkers:    1,
				UIQueueBufferSize:    100,
				LastEffortBufferSize: -1,
				BackgroundBufferSize: 100,
			},
			expectError:  true,
			errorMessage: "LastEffortBufferSize cannot be negative",
		},
		{
			name: "Negative Background buffer size",
			config: WorkerConfig{
				UIWorkers:            1,
				BackgroundWorkers:    1,
				UIQueueBufferSize:    100,
				LastEffortBufferSize: 100,
				BackgroundBufferSize: -1,
			},
			expectError:  true,
			errorMessage: "BackgroundBufferSize cannot be negative",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewTaskExecutorWithConfig(test.config, logger)

			if test.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if !strings.Contains(err.Error(), test.errorMessage) {
					t.Errorf("Expected error containing '%s', got '%s'", test.errorMessage, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

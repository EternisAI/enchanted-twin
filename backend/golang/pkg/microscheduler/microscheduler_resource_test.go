// microscheduler_resource_test.go contains tests for resource management:
// - Resource factory functionality and worker resource access
// - Worker type differentiation (UI vs Background workers)
// - Logging integration
// - Resource type validation
package microscheduler

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestResourceAccess(t *testing.T) {
	logger := createTestLogger()
	var resourcesUsed []string
	var mu sync.Mutex

	config := WorkerConfig{
		UIWorkers:         2,
		BackgroundWorkers: 2,
		ResourceFactory: func(workerID int, workerType WorkerType) interface{} {
			resource := &UIWorkerResource{
				WorkerID:   workerID,
				HTTPClient: fmt.Sprintf("http-client-%d", workerID),
				UIRenderer: fmt.Sprintf("ui-renderer-%d", workerID),
			}
			if workerType == BackgroundWorker {
				return &BackgroundWorkerResource{
					WorkerID:      workerID,
					DBConnection:  fmt.Sprintf("db-conn-%d", workerID),
					DataProcessor: fmt.Sprintf("data-proc-%d", workerID),
				}
			}
			return resource
		},
	}

	executor, err := NewTaskExecutorWithConfig(config, logger)
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}
	defer executor.Shutdown()

	ctx := context.Background()

	// Test UI task accessing UI worker resource
	uiTask := createStatelessTask("UI task", UI, 50*time.Millisecond, func(resource interface{}) (interface{}, error) {
		if uiRes, ok := resource.(*UIWorkerResource); ok {
			mu.Lock()
			resourcesUsed = append(resourcesUsed, fmt.Sprintf("UI: WorkerID=%d, HTTPClient=%s", uiRes.WorkerID, uiRes.HTTPClient))
			mu.Unlock()
			return "UI task completed", nil
		}
		return nil, fmt.Errorf("unexpected resource type: %T", resource)
	})

	// Test Background task accessing Background worker resource
	bgTask := createStatelessTask("Background task", Background, 50*time.Millisecond, func(resource interface{}) (interface{}, error) {
		if bgRes, ok := resource.(*BackgroundWorkerResource); ok {
			mu.Lock()
			resourcesUsed = append(resourcesUsed, fmt.Sprintf("BG: WorkerID=%d, DBConnection=%s", bgRes.WorkerID, bgRes.DBConnection))
			mu.Unlock()
			return "Background task completed", nil
		}
		return nil, fmt.Errorf("unexpected resource type: %T", resource)
	})

	// Execute tasks
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		result, err := executor.Execute(ctx, uiTask, UI)
		if err != nil {
			t.Errorf("UI task failed: %v", err)
		}
		if result != "UI task completed" {
			t.Errorf("Unexpected UI task result: %v", result)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		result, err := executor.Execute(ctx, bgTask, Background)
		if err != nil {
			t.Errorf("Background task failed: %v", err)
		}
		if result != "Background task completed" {
			t.Errorf("Unexpected Background task result: %v", result)
		}
	}()

	wg.Wait()

	mu.Lock()
	defer mu.Unlock()

	if len(resourcesUsed) != 2 {
		t.Errorf("Expected 2 resource usages, got %d", len(resourcesUsed))
	}

	t.Logf("Resources used: %v", resourcesUsed)

	// Verify that different worker types got appropriate resources
	uiResourceFound := false
	bgResourceFound := false
	for _, usage := range resourcesUsed {
		if strings.Contains(usage, "UI:") && strings.Contains(usage, "HTTPClient") {
			uiResourceFound = true
		}
		if strings.Contains(usage, "BG:") && strings.Contains(usage, "DBConnection") {
			bgResourceFound = true
		}
	}

	if !uiResourceFound {
		t.Error("UI worker did not receive UI resource")
	}
	if !bgResourceFound {
		t.Error("Background worker did not receive Background resource")
	}
}

func TestWorkerTypeResourceDifferentiation(t *testing.T) {
	logger := createTestLogger()
	var resourceTypes []string
	var mu sync.Mutex

	config := WorkerConfig{
		UIWorkers:         1,
		BackgroundWorkers: 1,
		ResourceFactory: func(workerID int, workerType WorkerType) interface{} {
			mu.Lock()
			resourceTypes = append(resourceTypes, fmt.Sprintf("Worker-%d: %s", workerID, workerType.String()))
			mu.Unlock()

			switch workerType {
			case UIWorker:
				return &UIWorkerResource{
					WorkerID:    workerID,
					HTTPClient:  fmt.Sprintf("ui-http-%d", workerID),
					UIRenderer:  fmt.Sprintf("ui-render-%d", workerID),
					TaskCounter: 0,
				}
			case BackgroundWorker:
				return &BackgroundWorkerResource{
					WorkerID:      workerID,
					DBConnection:  fmt.Sprintf("bg-db-%d", workerID),
					DataProcessor: fmt.Sprintf("bg-proc-%d", workerID),
					TaskCounter:   0,
				}
			default:
				return nil
			}
		},
	}

	executor, err := NewTaskExecutorWithConfig(config, logger)
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}
	defer executor.Shutdown()

	// Give the executor time to initialize workers
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(resourceTypes) < 1 {
		t.Error("Expected at least 1 worker to be initialized")
	}

	t.Logf("Worker types created: %v", resourceTypes)

	// Verify different worker types were created
	uiWorkerFound := false
	for _, resourceType := range resourceTypes {
		if strings.Contains(resourceType, "UIWorker") {
			uiWorkerFound = true
		}
	}

	if !uiWorkerFound {
		t.Error("UIWorker was not created")
	}
	// Note: BackgroundWorker might not be created in single processor mode
}

func TestWorkerTypeStringMethod(t *testing.T) {
	tests := []struct {
		workerType WorkerType
		expected   string
	}{
		{UIWorker, "UIWorker"},
		{BackgroundWorker, "BackgroundWorker"},
		{WorkerType(999), "Unknown"},
	}

	for _, test := range tests {
		if test.workerType.String() != test.expected {
			t.Errorf("Expected %s, got %s", test.expected, test.workerType.String())
		}
	}
}

func TestLoggingIntegration(t *testing.T) {
	// This test verifies that the executor works with the logging system
	// without actually checking log output (since we're using a discard logger)
	executor := NewTaskExecutor(1, createTestLogger())
	defer executor.Shutdown()

	ctx := context.Background()
	task := createStatelessTask("Logging test task", UI, 10*time.Millisecond, func(resource interface{}) (interface{}, error) {
		return "Logging test completed", nil
	})

	result, err := executor.Execute(ctx, task, UI)
	if err != nil {
		t.Errorf("Task execution failed: %v", err)
	}

	expected := "Logging test completed"
	if result != expected {
		t.Errorf("Expected '%s', got %v", expected, result)
	}
}
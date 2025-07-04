# Microscheduler

A lightweight, priority-based task scheduler for Go applications with support for stateful task execution, interruption, and worker pooling.

## Overview

Microscheduler is a standalone micro library that provides:

- **Priority-based scheduling** (UI, LastEffort, Background)
- **Stateful task execution** with state persistence and restoration
- **Task interruption and rescheduling** with channel-based signaling
- **Worker pools** with configurable UI and background workers
- **Resource management** with custom resource factories
- **Context-aware execution** with cancellation and timeout support

## Key Features

### Priority Levels
- **UI**: Highest priority for user-facing operations
- **LastEffort**: Critical tasks that need immediate attention but aren't UI-blocking
- **Background**: Lowest priority for background processing

### Task Types
- **Stateless tasks**: Simple execution with no state persistence
- **Stateful tasks**: Can save/restore state on interruption
- **Interruptible tasks**: Respond to channel-based interruption signals

### Architecture
- **Single processor mode**: For simple use cases with priority-based scheduling
- **Multi-worker mode**: Dedicated UI and background worker pools
- **Resource injection**: Custom resource factories for different worker types

## Quick Start

```go
package main

import (
    "context"
    "time"
    
    "github.com/EternisAI/enchanted-twin/pkg/microscheduler"
    "github.com/charmbracelet/log"
)

func main() {
    // Create a logger
    logger := log.New(os.Stdout)
    
    // Create a task executor with 2 workers
    executor := microscheduler.NewTaskExecutor(2, logger)
    defer executor.Shutdown()
    
    // Create a simple task
    task := microscheduler.Task{
        Name:         "Hello World",
        Priority:     microscheduler.UI,
        InitialState: &microscheduler.NoOpTaskState{},
        Compute: func(resource interface{}, state microscheduler.TaskState, interrupt *microscheduler.InterruptContext, interruptChan <-chan struct{}) (interface{}, error) {
            return "Hello, World!", nil
        },
    }
    
    // Execute the task
    ctx := context.Background()
    result, err := executor.Execute(ctx, task, microscheduler.UI)
    if err != nil {
        log.Fatal(err)
    }
    
    log.Info("Result:", result)
}
```

## Task Examples

### Stateless Task
```go
task := microscheduler.Task{
    Name:         "Simple Task",
    Priority:     microscheduler.Background,
    InitialState: &microscheduler.NoOpTaskState{},
    Compute: func(resource interface{}, state microscheduler.TaskState, interrupt *microscheduler.InterruptContext, interruptChan <-chan struct{}) (interface{}, error) {
        // Your task logic here
        return "Task completed", nil
    },
}
```

### Interruptible Task
```go
task := microscheduler.Task{
    Name:         "Interruptible Task",
    Priority:     microscheduler.Background,
    InitialState: &microscheduler.NoOpTaskState{},
    Compute: func(resource interface{}, state microscheduler.TaskState, interrupt *microscheduler.InterruptContext, interruptChan <-chan struct{}) (interface{}, error) {
        for i := 0; i < 100; i++ {
            select {
            case <-interruptChan:
                return fmt.Sprintf("Interrupted at step %d", i), fmt.Errorf("interrupted")
            default:
                // Do work
                time.Sleep(10 * time.Millisecond)
            }
        }
        return "Completed all steps", nil
    },
}
```

### Task with No Rescheduling
```go
task := microscheduler.Task{
    Name:         "No Reschedule Task",
    Priority:     microscheduler.Background,
    InitialState: &microscheduler.NoOpTaskState{},
    Compute: func(resource interface{}, state microscheduler.TaskState, interrupt *microscheduler.InterruptContext, interruptChan <-chan struct{}) (interface{}, error) {
        for i := 0; i < 100; i++ {
            select {
            case <-interruptChan:
                // Request not to be rescheduled when interrupted
                interrupt.NoReschedule()
                return fmt.Sprintf("Interrupted at step %d - no reschedule", i), nil
            default:
                // Do work
                time.Sleep(10 * time.Millisecond)
            }
        }
        return "Completed all steps", nil
    },
}
```

### Stateful Task
```go
type MyTaskState struct {
    Counter int    `json:"counter"`
    Message string `json:"message"`
}

func (s *MyTaskState) Save() ([]byte, error) {
    return json.Marshal(s)
}

func (s *MyTaskState) Restore(data []byte) error {
    return json.Unmarshal(data, s)
}

task := microscheduler.Task{
    Name:         "Stateful Task",
    Priority:     microscheduler.Background,
    InitialState: &MyTaskState{Counter: 0, Message: "starting"},
    Compute: func(resource interface{}, state microscheduler.TaskState, interrupt *microscheduler.InterruptContext, interruptChan <-chan struct{}) (interface{}, error) {
        myState := state.(*MyTaskState)
        
        for i := myState.Counter; i < 10; i++ {
            select {
            case <-interruptChan:
                // Save progress before interruption
                myState.Counter = i
                myState.Message = fmt.Sprintf("interrupted at %d", i)
                interrupt.SaveState(myState)
                return fmt.Sprintf("Saved at step %d", i), fmt.Errorf("interrupted")
            default:
                // Do work
                myState.Counter = i + 1
                time.Sleep(50 * time.Millisecond)
            }
        }
        
        myState.Message = "completed"
        return fmt.Sprintf("Finished: %d steps", myState.Counter), nil
    },
}
```

## Configuration

### Basic Configuration
```go
executor := microscheduler.NewTaskExecutor(processorCount, logger)
```

### Advanced Configuration
```go
config := microscheduler.WorkerConfig{
    UIWorkers:            2,
    BackgroundWorkers:    4,
    UIQueueBufferSize:    50,
    LastEffortBufferSize: 30,
    BackgroundBufferSize: 100,
    ResourceFactory: func(workerID int, workerType microscheduler.WorkerType) interface{} {
        // Return custom resources for different worker types
        if workerType == microscheduler.UIWorker {
            return &UIResource{ID: workerID}
        }
        return &BackgroundResource{ID: workerID}
    },
}

executor, err := microscheduler.NewTaskExecutorWithConfig(config, logger)
```

## Testing

The microscheduler package includes comprehensive tests organized by functionality:

```bash
# Run all tests
go test

# Run specific test categories
go test -run "TestBasic"           # Basic executor functionality
go test -run "TestContext"         # Context handling
go test -run "TestPriority"        # Priority scheduling
go test -run "TestConfig"          # Configuration
go test -run "TestResource"        # Resource management
go test -run "TestStateful"        # Stateful tasks
go test -run "TestInterruption"    # Task interruption
go test -run "TestSimplified"      # Unified API
```

## InterruptContext API

The `InterruptContext` provides several methods for tasks to interact with the interruption system:

- **`SaveState(TaskState)`**: Save the current task state for restoration if rescheduled
- **`IsInterrupted()`**: Check if an interruption signal has been received
- **`NoReschedule()`**: Request that the task should not be rescheduled if interrupted

### Task Interruption Behavior

1. **Normal interruption**: Task is rescheduled for later execution
2. **Interruption with state save**: Task is rescheduled with saved state
3. **Interruption with NoReschedule**: Task completes immediately, no rescheduling

## Architecture Notes

- **Single Processor Mode**: When `processorCount = 1`, uses a single worker that handles all priorities with preemption
- **Multi-Worker Mode**: Dedicated UI workers and background workers for better isolation
- **Priority Preemption**: Higher priority tasks can interrupt lower priority tasks in single processor mode
- **State Persistence**: Interrupted stateful tasks can be rescheduled with their saved state
- **Resource Isolation**: Different worker types can have different resources (DB connections, HTTP clients, etc.)
- **Task Control**: Tasks can control their own rescheduling behavior using `NoReschedule()`

## Dependencies

- `github.com/charmbracelet/log` - Structured logging
- Standard Go libraries (`context`, `sync`, `time`, etc.)

## License

This package is part of the Enchanted Twin project.
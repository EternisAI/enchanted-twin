package microscheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/charmbracelet/log"
)

type Priority int

const (
	UI Priority = iota
	// LastEffort priority is between UI and Background.
	// WARNING: Use this priority ONLY in critical circumstances when you have a background task
	// that is absolutely necessary but cannot wait in the background queue any longer
	// (e.g., after 10 retries, critical system recovery, etc.).
	//
	// CAUTION: LastEffort tasks will interleave with UI tasks and WILL hang user requests.
	// This priority should be used sparingly and only when the alternative is system failure
	// or data loss. Consider if the task can be redesigned to avoid this priority.
	LastEffort
	Background
)

func (p Priority) String() string {
	switch p {
	case UI:
		return "UI"
	case LastEffort:
		return "LastEffort"
	case Background:
		return "Background"
	default:
		return "Unknown"
	}
}

type TaskResult struct {
	Value interface{}
	Error error
}

// TaskState represents the execution state of a task.
type TaskState interface {
	// Save serializes the current state for later restoration
	Save() ([]byte, error)
	// Restore deserializes and applies the saved state
	Restore(data []byte) error
}

// NoOpTaskState is a no-operation state for stateless tasks.
type NoOpTaskState struct{}

func (n *NoOpTaskState) Save() ([]byte, error) {
	return []byte{}, nil
}

func (n *NoOpTaskState) Restore(data []byte) error {
	return nil
}

// InterruptContext provides information about the interruption to the task.
type InterruptContext struct {
	Reason        string
	SaveState     func(TaskState) error
	IsInterrupted func() bool
	NoReschedule  func() // Call this to indicate the task should not be rescheduled
}

// TaskCompute is the unified compute function that handles all task types.
type TaskCompute func(resource interface{}, state TaskState, interrupt *InterruptContext, interruptChan <-chan struct{}) (interface{}, error)

type Task struct {
	Name           string
	Priority       Priority
	Duration       time.Duration
	Compute        TaskCompute // Unified compute function
	InitialState   TaskState   // Will use NoOpTaskState for stateless tasks
	SavedStateData []byte      // For rescheduled tasks
}

type Executor interface {
	Execute(ctx context.Context, task Task, priority Priority) (interface{}, error)
}

type TaskProcessor interface {
	Process(req TaskRequest) TaskResult
}

type WorkerProcessor struct {
	ID        int
	Resource  interface{}
	logger    *log.Logger
	interrupt chan struct{}
}

func NewWorkerProcessor(id int, resource interface{}, logger *log.Logger) *WorkerProcessor {
	return &WorkerProcessor{
		ID:        id,
		Resource:  resource,
		logger:    logger,
		interrupt: make(chan struct{}, 1),
	}
}

func (w *WorkerProcessor) Interrupt() {
	select {
	case w.interrupt <- struct{}{}:
	default:
	}
}

type InterruptedTaskResult struct {
	TaskResult
	Interrupted  bool
	SavedState   []byte // Serialized state if task was interrupted
	StateSaved   bool
	NoReschedule bool // True if task requested not to be rescheduled
}

func (w *WorkerProcessor) ProcessWithInterruption(req TaskRequest) InterruptedTaskResult {
	select {
	case <-req.Context.Done():
		w.logger.Debug("Skipping orphaned task", "processorID", w.ID, "taskName", req.Task.Name, "reason", "context canceled")
		return InterruptedTaskResult{
			TaskResult:  TaskResult{Value: nil, Error: req.Context.Err()},
			Interrupted: false,
		}
	default:
	}

	w.logger.Info("Executing task", "processorID", w.ID, "taskName", req.Task.Name, "priority", req.Task.Priority.String())

	// Clear any pending interrupts before starting the task
	select {
	case <-w.interrupt:
	default:
	}

	// Process regular task with duration handling first
	if req.Task.Duration > 0 {
		select {
		case <-time.After(req.Task.Duration):
		case <-req.Context.Done():
			w.logger.Debug("Task canceled during execution", "processorID", w.ID, "taskName", req.Task.Name)
			return InterruptedTaskResult{
				TaskResult:  TaskResult{Value: nil, Error: req.Context.Err()},
				Interrupted: false,
			}
		case <-w.interrupt:
			w.logger.Debug("Task interrupted during execution", "processorID", w.ID, "taskName", req.Task.Name)
			return InterruptedTaskResult{
				TaskResult:  TaskResult{Value: nil, Error: fmt.Errorf("task interrupted")},
				Interrupted: true,
			}
		}
	}

	// Check for interruption before compute
	select {
	case <-req.Context.Done():
		w.logger.Debug("Task canceled before compute", "processorID", w.ID, "taskName", req.Task.Name)
		return InterruptedTaskResult{
			TaskResult:  TaskResult{Value: nil, Error: req.Context.Err()},
			Interrupted: false,
		}
	case <-w.interrupt:
		w.logger.Debug("Task interrupted before compute", "processorID", w.ID, "taskName", req.Task.Name)
		return InterruptedTaskResult{
			TaskResult:  TaskResult{Value: nil, Error: fmt.Errorf("task interrupted")},
			Interrupted: true,
		}
	default:
	}

	// All tasks now use the unified processing
	if req.Task.Compute != nil {
		return w.processTask(req)
	}

	return InterruptedTaskResult{
		TaskResult:  TaskResult{Value: fmt.Sprintf("Task %s completed", req.Task.Name), Error: nil},
		Interrupted: false,
	}
}

func (w *WorkerProcessor) processTask(req TaskRequest) InterruptedTaskResult {
	var taskState TaskState
	var err error

	// Restore state if this is a rescheduled task
	if len(req.Task.SavedStateData) > 0 && req.Task.InitialState != nil {
		taskState = req.Task.InitialState
		err = taskState.Restore(req.Task.SavedStateData)
		if err != nil {
			w.logger.Error("Failed to restore task state", "error", err, "taskName", req.Task.Name)
			return InterruptedTaskResult{
				TaskResult:  TaskResult{Value: nil, Error: fmt.Errorf("failed to restore state: %w", err)},
				Interrupted: false,
			}
		}
		w.logger.Debug("Restored task state", "taskName", req.Task.Name, "processorID", w.ID)
	} else if req.Task.InitialState != nil {
		taskState = req.Task.InitialState
		w.logger.Debug("Using initial task state", "taskName", req.Task.Name, "processorID", w.ID)
	}

	// Set up interrupt context
	var savedState []byte
	var stateSaved bool
	var noReschedule bool

	saveStateFunc := func(state TaskState) error {
		if state == nil {
			return fmt.Errorf("cannot save nil state")
		}
		data, err := state.Save()
		if err != nil {
			return err
		}
		savedState = data
		stateSaved = true
		w.logger.Debug("Task state saved", "taskName", req.Task.Name, "processorID", w.ID, "stateSize", len(data))
		return nil
	}

	isInterruptedFunc := func() bool {
		select {
		case <-w.interrupt:
			return true
		default:
			return false
		}
	}

	noRescheduleFunc := func() {
		noReschedule = true
		w.logger.Debug("Task requested no rescheduling", "taskName", req.Task.Name, "processorID", w.ID)
	}

	interruptCtx := &InterruptContext{
		Reason:        "priority preemption",
		SaveState:     saveStateFunc,
		IsInterrupted: isInterruptedFunc,
		NoReschedule:  noRescheduleFunc,
	}

	// Use NoOpTaskState for stateless tasks
	if taskState == nil {
		taskState = &NoOpTaskState{}
	}

	// Track if state was saved during execution
	finalSavedState := savedState
	finalStateSaved := stateSaved

	// Update the save state function to capture the final state
	originalSaveState := interruptCtx.SaveState
	interruptCtx.SaveState = func(state TaskState) error {
		if err := originalSaveState(state); err != nil {
			return err
		}
		// Get the actual saved state data from the original function's closure
		data, err := state.Save()
		if err != nil {
			return err
		}
		finalSavedState = data
		finalStateSaved = true
		w.logger.Debug("Task state saved during interruption", "taskName", req.Task.Name, "processorID", w.ID, "stateSize", len(data))
		return nil
	}

	// Create a done channel to signal completion
	done := make(chan struct {
		value interface{}
		err   error
	}, 1)

	// Start the compute function in a goroutine
	go func() {
		value, err := req.Task.Compute(w.Resource, taskState, interruptCtx, w.interrupt)
		done <- struct {
			value interface{}
			err   error
		}{value: value, err: err}
	}()

	// Wait for either completion or context cancellation
	select {
	case result := <-done:
		return InterruptedTaskResult{
			TaskResult:   TaskResult{Value: result.value, Error: result.err},
			Interrupted:  false,
			SavedState:   finalSavedState,
			StateSaved:   finalStateSaved,
			NoReschedule: noReschedule,
		}
	case <-req.Context.Done():
		w.logger.Debug("Task canceled", "processorID", w.ID, "taskName", req.Task.Name)
		// Signal interruption and wait for the goroutine to finish
		w.Interrupt()
		result := <-done // Wait for goroutine to complete
		return InterruptedTaskResult{
			TaskResult:   TaskResult{Value: result.value, Error: result.err},
			Interrupted:  true,
			SavedState:   finalSavedState,
			StateSaved:   finalStateSaved,
			NoReschedule: noReschedule,
		}
	}
}

func (w *WorkerProcessor) Process(req TaskRequest) TaskResult {
	result := w.ProcessWithInterruption(req)
	return result.TaskResult
}

type TaskRequest struct {
	Task    Task
	Context context.Context
	Done    chan TaskResult
}

type WorkerType int

const (
	UIWorker WorkerType = iota
	BackgroundWorker
)

func (wt WorkerType) String() string {
	switch wt {
	case UIWorker:
		return "UIWorker"
	case BackgroundWorker:
		return "BackgroundWorker"
	default:
		return "Unknown"
	}
}

type ResourceFactory func(workerID int, workerType WorkerType) interface{}

type WorkerConfig struct {
	UIWorkers            int
	BackgroundWorkers    int
	ResourceFactory      ResourceFactory
	UIQueueBufferSize    int
	LastEffortBufferSize int
	BackgroundBufferSize int
}

func (c *WorkerConfig) validate() error {
	totalWorkers := c.UIWorkers + c.BackgroundWorkers

	if totalWorkers <= 0 {
		return fmt.Errorf("total workers cannot be zero")
	}

	if totalWorkers == 1 {
		if c.UIWorkers < 0 || c.BackgroundWorkers < 0 {
			return fmt.Errorf("worker counts cannot be negative")
		}
	} else {
		if c.UIWorkers <= 0 {
			return fmt.Errorf("UIWorkers must be > 0 when total workers > 1, got %d", c.UIWorkers)
		}

		if c.BackgroundWorkers <= 0 {
			return fmt.Errorf("BackgroundWorkers must be > 0 when total workers > 1, got %d", c.BackgroundWorkers)
		}
	}

	if c.UIQueueBufferSize < 0 {
		return fmt.Errorf("UIQueueBufferSize cannot be negative, got %d", c.UIQueueBufferSize)
	}

	if c.LastEffortBufferSize < 0 {
		return fmt.Errorf("LastEffortBufferSize cannot be negative, got %d", c.LastEffortBufferSize)
	}

	if c.BackgroundBufferSize < 0 {
		return fmt.Errorf("BackgroundBufferSize cannot be negative, got %d", c.BackgroundBufferSize)
	}

	return nil
}

type TaskExecutor struct {
	uiQueue                 chan TaskRequest
	lastEffortQueue         chan TaskRequest
	backgroundQueue         chan TaskRequest
	backgroundPriorityQueue []TaskRequest // For rescheduled tasks
	config                  WorkerConfig
	logger                  *log.Logger
	mu                      sync.RWMutex
	once                    sync.Once
	shutdown                chan bool
	isShutdown              bool
}

func NewTaskExecutor(processorCount int, logger *log.Logger) *TaskExecutor {
	if processorCount < 1 {
		processorCount = 1
	}

	var config WorkerConfig
	if processorCount == 1 {
		config = WorkerConfig{
			UIWorkers:            1,
			BackgroundWorkers:    0,
			UIQueueBufferSize:    100,
			LastEffortBufferSize: 100,
			BackgroundBufferSize: 100,
		}
	} else {
		config = WorkerConfig{
			UIWorkers:            1,
			BackgroundWorkers:    processorCount - 1,
			UIQueueBufferSize:    100,
			LastEffortBufferSize: 100,
			BackgroundBufferSize: 100,
		}
	}

	// Default resource factory returns nil
	config.ResourceFactory = func(workerID int, workerType WorkerType) interface{} {
		return nil
	}

	e := &TaskExecutor{
		uiQueue:                 make(chan TaskRequest, config.UIQueueBufferSize),
		lastEffortQueue:         make(chan TaskRequest, config.LastEffortBufferSize),
		backgroundQueue:         make(chan TaskRequest, config.BackgroundBufferSize),
		backgroundPriorityQueue: make([]TaskRequest, 0),
		config:                  config,
		logger:                  logger,
		shutdown:                make(chan bool),
	}

	e.startProcessors()
	return e
}

func NewTaskExecutorWithConfig(config WorkerConfig, logger *log.Logger) (*TaskExecutor, error) {
	// Set default buffer sizes if not provided
	if config.UIQueueBufferSize == 0 {
		config.UIQueueBufferSize = 100
	}
	if config.LastEffortBufferSize == 0 {
		config.LastEffortBufferSize = 100
	}
	if config.BackgroundBufferSize == 0 {
		config.BackgroundBufferSize = 100
	}

	if err := config.validate(); err != nil {
		return nil, err
	}

	// Default resource factory if none provided
	if config.ResourceFactory == nil {
		config.ResourceFactory = func(workerID int, workerType WorkerType) interface{} {
			return nil
		}
	}

	e := &TaskExecutor{
		uiQueue:                 make(chan TaskRequest, config.UIQueueBufferSize),
		lastEffortQueue:         make(chan TaskRequest, config.LastEffortBufferSize),
		backgroundQueue:         make(chan TaskRequest, config.BackgroundBufferSize),
		backgroundPriorityQueue: make([]TaskRequest, 0),
		config:                  config,
		logger:                  logger,
		shutdown:                make(chan bool),
	}

	e.startProcessors()
	return e, nil
}

func (e *TaskExecutor) startProcessors() {
	totalWorkers := e.config.UIWorkers + e.config.BackgroundWorkers

	if totalWorkers == 1 {
		go e.singleProcessor()
	} else {
		for i := 0; i < e.config.UIWorkers; i++ {
			go e.dedicatedUIProcessor(i)
		}
		for i := 0; i < e.config.BackgroundWorkers; i++ {
			go e.backgroundProcessor(e.config.UIWorkers + i)
		}
	}
}

func (e *TaskExecutor) singleProcessor() {
	// Single processor handles all priorities, consider it a UI worker
	processor := NewWorkerProcessor(0, e.config.ResourceFactory(0, UIWorker), e.logger)

	for {
		select {
		case req := <-e.uiQueue:
			result := processor.Process(req)
			req.Done <- result
		case req := <-e.lastEffortQueue:
			result := processor.Process(req)
			req.Done <- result
		case <-e.shutdown:
			return
		default:
			// Check for background tasks when no high priority tasks are available
			if req, ok := e.getNextBackgroundTask(); ok {
				// Check once more for high priority tasks before starting background task
				select {
				case uiReq := <-e.uiQueue:
					// High priority task arrived, interrupt background task if running and reschedule
					processor.Interrupt()
					e.rescheduleTask(req)
					result := processor.Process(uiReq)
					uiReq.Done <- result
				case lastEffortReq := <-e.lastEffortQueue:
					// High priority task arrived, interrupt background task if running and reschedule
					processor.Interrupt()
					e.rescheduleTask(req)
					result := processor.Process(lastEffortReq)
					lastEffortReq.Done <- result
				default:
					// No high priority tasks, process background task with interruption support
					// Run the background task in a goroutine so it can be interrupted
					resultChan := make(chan InterruptedTaskResult, 1)
					go func() {
						result := processor.ProcessWithInterruption(req)
						resultChan <- result
					}()

					// Wait for either task completion or higher priority tasks
					select {
					case result := <-resultChan:
						// Background task completed
						if result.Interrupted {
							// Task was interrupted - handle rescheduling based on task preference and state
							if result.NoReschedule {
								// Task requested not to be rescheduled
								e.logger.Debug("Task interrupted but requested no rescheduling", "taskName", req.Task.Name)
								req.Done <- result.TaskResult
							} else if result.StateSaved {
								// Reschedule with saved state
								e.rescheduleTaskWithState(req, result.SavedState)
								e.logger.Debug("Task interrupted and rescheduled with state",
									"taskName", req.Task.Name, "stateSize", len(result.SavedState))
							} else if req.Task.Priority == Background {
								// Regular background task interrupted, reschedule without state
								e.rescheduleTask(req)
								e.logger.Debug("Background task interrupted and rescheduled", "taskName", req.Task.Name)
							}
						} else {
							// Task completed normally
							req.Done <- result.TaskResult
						}
					case uiReq := <-e.uiQueue:
						// UI task arrived while background was running - interrupt it
						processor.Interrupt()

						// Wait for background task to finish being interrupted
						result := <-resultChan
						if result.Interrupted && !result.NoReschedule {
							if result.StateSaved {
								e.rescheduleTaskWithState(req, result.SavedState)
							} else {
								e.rescheduleTask(req)
							}
						} else if result.Interrupted && result.NoReschedule {
							e.logger.Debug("Task interrupted by UI task but requested no rescheduling", "taskName", req.Task.Name)
							req.Done <- result.TaskResult
						}

						// Process the UI task
						uiResult := processor.Process(uiReq)
						uiReq.Done <- uiResult
					case lastEffortReq := <-e.lastEffortQueue:
						// LastEffort task arrived while background was running - interrupt it
						processor.Interrupt()

						// Wait for background task to finish being interrupted
						result := <-resultChan
						if result.Interrupted && !result.NoReschedule {
							if result.StateSaved {
								e.rescheduleTaskWithState(req, result.SavedState)
							} else {
								e.rescheduleTask(req)
							}
						} else if result.Interrupted && result.NoReschedule {
							e.logger.Debug("Task interrupted by LastEffort task but requested no rescheduling", "taskName", req.Task.Name)
							req.Done <- result.TaskResult
						}

						// Process the LastEffort task
						lastEffortResult := processor.Process(lastEffortReq)
						lastEffortReq.Done <- lastEffortResult
					}
				}
			} else {
				// No tasks available, wait a bit
				time.Sleep(1 * time.Millisecond)
			}
		}
	}
}

func (e *TaskExecutor) dedicatedUIProcessor(id int) {
	processor := NewWorkerProcessor(id, e.config.ResourceFactory(id, UIWorker), e.logger)
	for {
		select {
		case req := <-e.uiQueue:
			result := processor.Process(req)
			req.Done <- result
		case req := <-e.lastEffortQueue:
			result := processor.Process(req)
			req.Done <- result
		case <-e.shutdown:
			return
		}
	}
}

func (e *TaskExecutor) backgroundProcessor(id int) {
	processor := NewWorkerProcessor(id, e.config.ResourceFactory(id, BackgroundWorker), e.logger)
	for {
		select {
		case req := <-e.backgroundQueue:
			result := processor.Process(req)
			req.Done <- result
		case <-e.shutdown:
			return
		}
	}
}

func (e *TaskExecutor) Execute(ctx context.Context, task Task, priority Priority) (interface{}, error) {
	e.mu.RLock()
	if e.isShutdown {
		e.mu.RUnlock()
		return nil, fmt.Errorf("executor is shutdown")
	}
	e.mu.RUnlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	done := make(chan TaskResult, 1)
	req := TaskRequest{
		Task:    task,
		Context: ctx,
		Done:    done,
	}

	switch priority {
	case UI:
		select {
		case e.uiQueue <- req:
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-e.shutdown:
			return nil, fmt.Errorf("executor is shutdown")
		}
	case LastEffort:
		select {
		case e.lastEffortQueue <- req:
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-e.shutdown:
			return nil, fmt.Errorf("executor is shutdown")
		}
	case Background:
		select {
		case e.backgroundQueue <- req:
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-e.shutdown:
			return nil, fmt.Errorf("executor is shutdown")
		}
	}

	select {
	case result := <-done:
		return result.Value, result.Error
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-e.shutdown:
		return nil, fmt.Errorf("executor is shutdown")
	}
}

func (e *TaskExecutor) rescheduleTask(req TaskRequest) {
	e.mu.Lock()
	defer e.mu.Unlock()
	// Add task to the beginning of the priority queue for immediate processing
	e.backgroundPriorityQueue = append([]TaskRequest{req}, e.backgroundPriorityQueue...)
	e.logger.Debug("Task rescheduled", "taskName", req.Task.Name, "queueLength", len(e.backgroundPriorityQueue))
}

func (e *TaskExecutor) rescheduleTaskWithState(req TaskRequest, savedState []byte) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Update the task with saved state for restoration
	req.Task.SavedStateData = savedState

	// Add task to the beginning of the priority queue for immediate processing
	e.backgroundPriorityQueue = append([]TaskRequest{req}, e.backgroundPriorityQueue...)
	e.logger.Debug("Stateful task rescheduled", "taskName", req.Task.Name, "queueLength", len(e.backgroundPriorityQueue), "stateSize", len(savedState))
}

func (e *TaskExecutor) getNextBackgroundTask() (TaskRequest, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// First check priority queue for rescheduled tasks
	if len(e.backgroundPriorityQueue) > 0 {
		task := e.backgroundPriorityQueue[0]
		e.backgroundPriorityQueue = e.backgroundPriorityQueue[1:]
		return task, true
	}

	// Then check regular background queue
	select {
	case req := <-e.backgroundQueue:
		return req, true
	default:
		return TaskRequest{}, false
	}
}

func (e *TaskExecutor) Shutdown() {
	e.once.Do(func() {
		e.mu.Lock()
		e.isShutdown = true
		e.mu.Unlock()
		close(e.shutdown)
	})
}

type UIWorkerResource struct {
	WorkerID    int
	HTTPClient  string
	UIRenderer  string
	TaskCounter int
}

type BackgroundWorkerResource struct {
	WorkerID      int
	DBConnection  string
	DataProcessor string
	TaskCounter   int
}

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
	// LastEffort priority is between UI and Background for critical tasks only.
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

type TaskState interface {
	Save() ([]byte, error)
	Restore(data []byte) error
}

type NoOpTaskState struct{}

func (n *NoOpTaskState) Save() ([]byte, error) {
	return []byte{}, nil
}

func (n *NoOpTaskState) Restore(data []byte) error {
	return nil
}

type InterruptContext struct {
	Reason        string
	SaveState     func(TaskState) error
	IsInterrupted func() bool
	NoReschedule  func()
}

type TaskCompute func(resource interface{}, state TaskState, interrupt *InterruptContext, interruptChan <-chan struct{}) (interface{}, error)

type Task struct {
	Name           string
	Priority       Priority
	Duration       time.Duration
	Compute        TaskCompute // Unified compute function
	InitialState   TaskState
	SavedStateData []byte
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
	SavedState   []byte
	StateSaved   bool
	NoReschedule bool
}

func (w *WorkerProcessor) ProcessWithInterruption(req TaskRequest) InterruptedTaskResult {
	select {
	case <-req.Context.Done():
		w.logger.Debug("Skipping orphaned task", "processorID", w.ID, "taskName", req.Task.Name)
		return InterruptedTaskResult{
			TaskResult:  TaskResult{Value: nil, Error: req.Context.Err()},
			Interrupted: false,
		}
	default:
	}

	w.logger.Info("Executing task", "processorID", w.ID, "taskName", req.Task.Name, "priority", req.Task.Priority.String())

	select {
	case <-w.interrupt:
	default:
	}

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

	if taskState == nil {
		taskState = &NoOpTaskState{}
	}

	finalSavedState := savedState
	finalStateSaved := stateSaved

	originalSaveState := interruptCtx.SaveState
	interruptCtx.SaveState = func(state TaskState) error {
		if err := originalSaveState(state); err != nil {
			return err
		}
		data, err := state.Save()
		if err != nil {
			return err
		}
		finalSavedState = data
		finalStateSaved = true
		w.logger.Debug("Task state saved during interruption", "taskName", req.Task.Name, "processorID", w.ID, "stateSize", len(data))
		return nil
	}

	done := make(chan struct {
		value interface{}
		err   error
	}, 1)

	go func() {
		value, err := req.Task.Compute(w.Resource, taskState, interruptCtx, w.interrupt)
		done <- struct {
			value interface{}
			err   error
		}{value: value, err: err}
	}()

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
		w.Interrupt()
		result := <-done
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
			if req, ok := e.getNextBackgroundTask(); ok {
				select {
				case uiReq := <-e.uiQueue:
					processor.Interrupt()
					e.rescheduleTask(req)
					result := processor.Process(uiReq)
					uiReq.Done <- result
				case lastEffortReq := <-e.lastEffortQueue:
					processor.Interrupt()
					e.rescheduleTask(req)
					result := processor.Process(lastEffortReq)
					lastEffortReq.Done <- result
				default:
					resultChan := make(chan InterruptedTaskResult, 1)
					go func() {
						result := processor.ProcessWithInterruption(req)
						resultChan <- result
					}()

					select {
					case result := <-resultChan:
						if result.Interrupted {
							if result.NoReschedule {
								e.logger.Debug("Task interrupted but requested no rescheduling", "taskName", req.Task.Name)
								req.Done <- result.TaskResult
							} else if result.StateSaved {
								e.rescheduleTaskWithState(req, result.SavedState)
								e.logger.Debug("Task interrupted and rescheduled with state",
									"taskName", req.Task.Name, "stateSize", len(result.SavedState))
							} else if req.Task.Priority == Background {
								e.rescheduleTask(req)
								e.logger.Debug("Background task interrupted and rescheduled", "taskName", req.Task.Name)
							}
						} else {
							req.Done <- result.TaskResult
						}
					case uiReq := <-e.uiQueue:
						processor.Interrupt()

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

						uiResult := processor.Process(uiReq)
						uiReq.Done <- uiResult
					case lastEffortReq := <-e.lastEffortQueue:
						processor.Interrupt()

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

						lastEffortResult := processor.Process(lastEffortReq)
						lastEffortReq.Done <- lastEffortResult
					}
				}
			} else {
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
	e.backgroundPriorityQueue = append([]TaskRequest{req}, e.backgroundPriorityQueue...)
	e.logger.Debug("Task rescheduled", "taskName", req.Task.Name, "queueLength", len(e.backgroundPriorityQueue))
}

func (e *TaskExecutor) rescheduleTaskWithState(req TaskRequest, savedState []byte) {
	e.mu.Lock()
	defer e.mu.Unlock()

	req.Task.SavedStateData = savedState
	e.backgroundPriorityQueue = append([]TaskRequest{req}, e.backgroundPriorityQueue...)
	e.logger.Debug("Stateful task rescheduled", "taskName", req.Task.Name, "queueLength", len(e.backgroundPriorityQueue), "stateSize", len(savedState))
}

func (e *TaskExecutor) getNextBackgroundTask() (TaskRequest, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.backgroundPriorityQueue) > 0 {
		task := e.backgroundPriorityQueue[0]
		e.backgroundPriorityQueue = e.backgroundPriorityQueue[1:]
		return task, true
	}

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

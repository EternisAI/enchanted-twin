package helpers

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

type Task struct {
	Name     string
	Priority Priority
	Duration time.Duration
	Compute  func(resource interface{}) (interface{}, error)
}

type Executor interface {
	Execute(ctx context.Context, task Task, priority Priority) (interface{}, error)
}

type TaskProcessor interface {
	Process(req TaskRequest) TaskResult
}

type WorkerProcessor struct {
	ID       int
	Resource interface{}
	logger   *log.Logger
}

func NewWorkerProcessor(id int, resource interface{}, logger *log.Logger) *WorkerProcessor {
	return &WorkerProcessor{
		ID:       id,
		Resource: resource,
		logger:   logger,
	}
}

func (w *WorkerProcessor) Process(req TaskRequest) TaskResult {
	select {
	case <-req.Context.Done():
		w.logger.Debug("Skipping orphaned task", "processorID", w.ID, "taskName", req.Task.Name, "reason", "context canceled")
		return TaskResult{Value: nil, Error: req.Context.Err()}
	default:
	}

	w.logger.Info("Executing task", "processorID", w.ID, "taskName", req.Task.Name, "priority", req.Task.Priority.String())

	if req.Task.Duration > 0 {
		select {
		case <-time.After(req.Task.Duration):
		case <-req.Context.Done():
			w.logger.Debug("Task canceled during execution", "processorID", w.ID, "taskName", req.Task.Name)
			return TaskResult{Value: nil, Error: req.Context.Err()}
		}
	}

	select {
	case <-req.Context.Done():
		w.logger.Debug("Task canceled before compute", "processorID", w.ID, "taskName", req.Task.Name)
		return TaskResult{Value: nil, Error: req.Context.Err()}
	default:
	}

	if req.Task.Compute != nil {
		value, err := req.Task.Compute(w.Resource)
		return TaskResult{Value: value, Error: err}
	}

	return TaskResult{Value: fmt.Sprintf("Task %s completed", req.Task.Name), Error: nil}
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
	uiQueue         chan TaskRequest
	lastEffortQueue chan TaskRequest
	backgroundQueue chan TaskRequest
	config          WorkerConfig
	logger          *log.Logger
	mu              sync.RWMutex
	once            sync.Once
	shutdown        chan bool
	isShutdown      bool
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
		uiQueue:         make(chan TaskRequest, config.UIQueueBufferSize),
		lastEffortQueue: make(chan TaskRequest, config.LastEffortBufferSize),
		backgroundQueue: make(chan TaskRequest, config.BackgroundBufferSize),
		config:          config,
		logger:          logger,
		shutdown:        make(chan bool),
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
		uiQueue:         make(chan TaskRequest, config.UIQueueBufferSize),
		lastEffortQueue: make(chan TaskRequest, config.LastEffortBufferSize),
		backgroundQueue: make(chan TaskRequest, config.BackgroundBufferSize),
		config:          config,
		logger:          logger,
		shutdown:        make(chan bool),
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
			select {
			case req := <-e.backgroundQueue:
				result := processor.Process(req)
				req.Done <- result
			case <-e.shutdown:
				return
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

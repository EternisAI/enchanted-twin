package evolvingmemory

import (
	"context"
	"sync"
	"time"
)

// Job represents a unit of work.
type Job[T any] interface {
	Process(ctx context.Context) (T, error)
}

// WorkerPool is a generic dynamic worker pool with work-stealing.
type WorkerPool[J Job[R], R any] struct {
	workers int
	logger  interface {
		Debugf(format string, args ...interface{})
		Infof(format string, args ...interface{})
	}
}

// NewWorkerPool creates a new worker pool.
func NewWorkerPool[J Job[R], R any](workers int, logger interface {
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
},
) *WorkerPool[J, R] {
	return &WorkerPool[J, R]{
		workers: workers,
		logger:  logger,
	}
}

// ProcessResult contains the result of processing a job.
type ProcessResult[J Job[R], R any] struct {
	Job    J
	Result R
	Error  error
}

// Process executes jobs using dynamic work distribution.
func (wp *WorkerPool[J, R]) Process(
	ctx context.Context,
	jobs []J,
	timeout time.Duration,
) <-chan ProcessResult[J, R] {
	jobQueue := make(chan J, len(jobs))
	results := make(chan ProcessResult[J, R], len(jobs))

	// Load all jobs into the queue
	for _, job := range jobs {
		jobQueue <- job
	}
	close(jobQueue)

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < wp.workers; i++ {
		wg.Add(1)
		go wp.worker(ctx, i, jobQueue, results, timeout, &wg)
	}

	// Close results channel when all workers are done
	go func() {
		wg.Wait()
		close(results)
	}()

	return results
}

// worker processes jobs from the queue.
func (wp *WorkerPool[J, R]) worker(
	ctx context.Context,
	id int,
	jobs <-chan J,
	results chan<- ProcessResult[J, R],
	timeout time.Duration,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	processedCount := 0
	startTime := time.Now()

	for job := range jobs {
		jobStart := time.Now()

		// Create timeout context for this job
		jobCtx, cancel := context.WithTimeout(ctx, timeout)

		// Process the job
		result, err := job.Process(jobCtx)
		cancel()

		processingTime := time.Since(jobStart)

		if err != nil {
			wp.logger.Debugf("Worker %d: Job failed after %v: %v", id, processingTime, err)
		} else {
			wp.logger.Debugf("Worker %d: Job completed in %v", id, processingTime)
			processedCount++
		}

		// Send result
		select {
		case results <- ProcessResult[J, R]{Job: job, Result: result, Error: err}:
		case <-ctx.Done():
			wp.logger.Infof("Worker %d: Stopped after processing %d jobs", id, processedCount)
			return
		}
	}

	totalTime := time.Since(startTime)
	if processedCount > 0 {
		avgTime := totalTime / time.Duration(processedCount)
		wp.logger.Infof("Worker %d: Completed %d jobs in %v (avg: %v/job)",
			id, processedCount, totalTime, avgTime)
	}
}

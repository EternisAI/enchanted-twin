package holon

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/charmbracelet/log"
)

// BackgroundProcessor handles automatic thread processing in the background
type BackgroundProcessor struct {
	service            *Service
	logger             *log.Logger
	processingInterval time.Duration
	stopChan           chan struct{}
	wg                 sync.WaitGroup
	running            bool
	mu                 sync.RWMutex
}

// NewBackgroundProcessor creates a new background processor
func NewBackgroundProcessor(service *Service, logger *log.Logger, processingInterval time.Duration) *BackgroundProcessor {
	if processingInterval == 0 {
		processingInterval = 30 * time.Second // Default to 30 seconds
	}

	return &BackgroundProcessor{
		service:            service,
		logger:             logger,
		processingInterval: processingInterval,
		stopChan:           make(chan struct{}),
	}
}

// Start begins background processing of received threads
func (bp *BackgroundProcessor) Start(ctx context.Context) error {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	if bp.running {
		return nil // Already running
	}

	// Set running state and immediately increment WaitGroup to ensure they're always paired
	bp.running = true
	bp.wg.Add(1)

	if !bp.service.IsThreadProcessorReady() {
		bp.logger.Warn("Thread processor not ready, background processing will be limited")
	}

	go func() {
		defer bp.wg.Done()
		bp.run(ctx)
	}()

	bp.logger.Info("Background thread processor started", "interval", bp.processingInterval)
	return nil
}

// Stop gracefully stops the background processor
func (bp *BackgroundProcessor) Stop() {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	if !bp.running {
		return
	}

	close(bp.stopChan)
	bp.running = false
	bp.wg.Wait()
	bp.logger.Info("Background thread processor stopped")
}

// run is the main processing loop
func (bp *BackgroundProcessor) run(ctx context.Context) {
	ticker := time.NewTicker(bp.processingInterval)
	defer ticker.Stop()

	// Run initial bootstrap processing
	if err := bp.runBootstrap(ctx); err != nil {
		bp.logger.Error("Bootstrap processing failed", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			bp.logger.Info("Background processor stopping due to context cancellation")
			return
		case <-bp.stopChan:
			bp.logger.Info("Background processor stopping due to stop signal")
			return
		case <-ticker.C:
			if err := bp.processReceivedThreads(ctx); err != nil {
				bp.logger.Error("Failed to process received threads", "error", err)
			}
		}
	}
}

// runBootstrap performs initial processing of existing received threads
func (bp *BackgroundProcessor) runBootstrap(ctx context.Context) error {
	bp.logger.Info("Running bootstrap processing of received threads")

	if !bp.service.IsThreadProcessorReady() {
		bp.logger.Warn("Skipping bootstrap - thread processor not ready")
		return nil
	}

	return bp.service.BootstrapProcessAllReceivedThreads(ctx)
}

// processReceivedThreads processes any new threads with 'received' state
func (bp *BackgroundProcessor) processReceivedThreads(ctx context.Context) error {
	if !bp.service.IsThreadProcessorReady() {
		// Skip processing if thread processor isn't ready
		return nil
	}

	// Get count of received threads for logging
	receivedThreads, err := bp.service.GetThreadsByState(ctx, "received")
	if err != nil {
		return err
	}

	if len(receivedThreads) == 0 {
		// No threads to process
		return nil
	}

	bp.logger.Info("Processing received threads", "count", len(receivedThreads))
	return bp.service.ProcessReceivedThreads(ctx)
}

// IsRunning returns true if the background processor is running
func (bp *BackgroundProcessor) IsRunning() bool {
	bp.mu.RLock()
	defer bp.mu.RUnlock()
	return bp.running
}

// ProcessSingleThreadNow immediately processes a specific thread (useful for newly arrived threads)
func (bp *BackgroundProcessor) ProcessSingleThreadNow(ctx context.Context, threadID string) error {
	if !bp.service.IsThreadProcessorReady() {
		return fmt.Errorf("thread processor not ready")
	}

	bp.logger.Info("Processing single thread immediately", "thread_id", threadID)
	return bp.service.ProcessSingleReceivedThread(ctx, threadID)
}

// GetStatus returns the current status of the background processor
func (bp *BackgroundProcessor) GetStatus() map[string]interface{} {
	bp.mu.RLock()
	defer bp.mu.RUnlock()

	return map[string]interface{}{
		"running":             bp.running,
		"processing_interval": bp.processingInterval.String(),
		"processor_ready":     bp.service.IsThreadProcessorReady(),
	}
}

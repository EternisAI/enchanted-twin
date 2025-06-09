package holon

import (
	"context"
	"fmt"
	"time"

	clog "github.com/charmbracelet/log"
	"go.temporal.io/sdk/worker"
)

// HolonSyncWorkflowInput contains the input parameters for the holon sync workflow
type HolonSyncWorkflowInput struct {
	ForceSync bool
}

// HolonSyncWorkflowOutput contains the output parameters for the holon sync workflow
type HolonSyncWorkflowOutput struct {
	Success          bool      `json:"success"`
	ParticipantCount int       `json:"participant_count"`
	ThreadCount      int       `json:"thread_count"`
	ReplyCount       int       `json:"reply_count"`
	LastSyncTime     time.Time `json:"last_sync_time"`
	Error            string    `json:"error,omitempty"`
}

// HolonSyncActivities defines the activities for syncing holons
type HolonSyncActivities struct {
	logger  *clog.Logger
	manager *Manager
}

// NewHolonSyncActivities creates a new instance of holon sync activities
func NewHolonSyncActivities(logger *clog.Logger, manager *Manager) *HolonSyncActivities {
	return &HolonSyncActivities{
		logger:  logger,
		manager: manager,
	}
}

// RegisterWorkflowsAndActivities registers the workflows and activities with the Temporal worker
func (a *HolonSyncActivities) RegisterWorkflowsAndActivities(worker worker.Worker) {
	worker.RegisterWorkflow(HolonSyncWorkflow)
	worker.RegisterActivity(a.SyncHolonDataActivity)
}

// SyncHolonDataActivity performs the complete holon data synchronization workflow
func (a *HolonSyncActivities) SyncHolonDataActivity(ctx context.Context, input HolonSyncWorkflowInput) (HolonSyncWorkflowOutput, error) {
	result := HolonSyncWorkflowOutput{
		Success:      true,
		LastSyncTime: time.Now(),
	}

	a.logger.Info("Starting holon data synchronization workflow")

	// Step 1: Push any pending local content to HolonZero API (outbound sync)
	a.logger.Info("Pushing pending content to HolonZero API")
	if err := a.pushPendingContent(ctx); err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to push pending content: %v", err)
		a.logger.Error("Failed to push pending content", "error", err)
		return result, err
	}
	a.logger.Info("Successfully pushed pending content to HolonZero API")

	// Step 2: Fetch latest threads from HolonZero API (inbound sync)
	a.logger.Info("Fetching threads from HolonZero API")
	threads, err := a.manager.fetcherService.SyncThreads(ctx)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to sync threads: %v", err)
		a.logger.Error("Failed to sync threads", "error", err)
		return result, err
	}
	result.ThreadCount = len(threads)
	a.logger.Info("Successfully synced threads", "count", len(threads))

	// Step 3: Fetch latest replies from HolonZero API (inbound sync)
	a.logger.Info("Fetching replies from HolonZero API")
	replies, err := a.manager.fetcherService.SyncReplies(ctx)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to sync replies: %v", err)
		a.logger.Error("Failed to sync replies", "error", err)
		return result, err
	}
	result.ReplyCount = len(replies)
	a.logger.Info("Successfully synced replies", "count", len(replies))

	a.logger.Info("Holon data synchronization workflow completed successfully",
		"threads_synced", len(threads),
		"replies_synced", len(replies))

	return result, nil
}

// pushPendingContent is a helper method to push pending content to HolonZero API
func (a *HolonSyncActivities) pushPendingContent(ctx context.Context) error {
	if a.manager.fetcherService == nil {
		return fmt.Errorf("fetcher service is not available")
	}
	return a.manager.fetcherService.PushPendingContent(ctx)
}

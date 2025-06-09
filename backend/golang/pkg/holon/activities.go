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
	worker.RegisterActivity(a.SyncThreads)
	worker.RegisterActivity(a.SyncReplies)
	worker.RegisterActivity(a.PushPendingThreads)
	worker.RegisterActivity(a.PushPendingContent)
}

// SyncThreads is an activity that syncs threads from the holon API
func (a *HolonSyncActivities) SyncThreads(ctx context.Context) error {
	a.logger.Debug("Starting holon threads sync activity")

	if a.manager.fetcherService == nil {
		return fmt.Errorf("fetcher service is not available")
	}

	_, err := a.manager.fetcherService.SyncThreads(ctx)
	if err != nil {
		a.logger.Error("Failed to sync threads", "error", err)
		return err
	}

	a.logger.Debug("Holon threads sync completed successfully")
	return nil
}

// SyncReplies is an activity that syncs replies from the holon API
func (a *HolonSyncActivities) SyncReplies(ctx context.Context) error {
	a.logger.Debug("Starting holon replies sync activity")

	if a.manager.fetcherService == nil {
		return fmt.Errorf("fetcher service is not available")
	}

	_, err := a.manager.fetcherService.SyncReplies(ctx)
	if err != nil {
		a.logger.Error("Failed to sync replies", "error", err)
		return err
	}

	a.logger.Debug("Holon replies sync completed successfully")
	return nil
}

// PushPendingThreads is an activity that pushes pending threads to the holon API
func (a *HolonSyncActivities) PushPendingThreads(ctx context.Context) error {
	a.logger.Debug("Starting push pending threads activity")

	if a.manager.fetcherService == nil {
		return fmt.Errorf("fetcher service is not available")
	}

	err := a.manager.fetcherService.PushPendingThreads(ctx)
	if err != nil {
		a.logger.Error("Failed to push pending threads", "error", err)
		return err
	}

	a.logger.Debug("Push pending threads completed successfully")
	return nil
}

// PushPendingContent is an activity that pushes all pending content (threads and replies) to the holon API
func (a *HolonSyncActivities) PushPendingContent(ctx context.Context) error {
	a.logger.Debug("Starting holon pending content push activity")

	if a.manager.fetcherService == nil {
		return fmt.Errorf("fetcher service is not available")
	}

	err := a.manager.fetcherService.PushPendingContent(ctx)
	if err != nil {
		a.logger.Error("Failed to push pending content", "error", err)
		return err
	}

	a.logger.Debug("Holon pending content push completed successfully")
	return nil
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
	if err := a.PushPendingContent(ctx); err != nil {
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

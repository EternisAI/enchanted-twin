package holon

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// HolonSyncWorkflowOutput defines the output for the holon sync workflow
type HolonSyncWorkflowOutput struct {
	Success          bool      `json:"success"`
	ParticipantCount int       `json:"participant_count"`
	ThreadCount      int       `json:"thread_count"`
	ReplyCount       int       `json:"reply_count"`
	LastSyncTime     time.Time `json:"last_sync_time"`
	Error            string    `json:"error,omitempty"`
}

// HolonSyncWorkflow orchestrates the periodic synchronization with HolonZero API
func HolonSyncWorkflow(
	ctx workflow.Context,
	input HolonSyncWorkflowInput,
) (HolonSyncWorkflowOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Debug("Starting holon sync workflow", "force_sync", input.ForceSync)

	// Configure activity options with appropriate timeouts and retry policy
	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute, // Allow up to 10 minutes for sync
		HeartbeatTimeout:    30 * time.Second, // Heartbeat every 30 seconds
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second * 5,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute * 2,
			MaximumAttempts:    3, // Retry up to 3 times
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions)

	var activities *HolonSyncActivities

	// Execute the sync activity
	var result HolonSyncWorkflowOutput
	err := workflow.ExecuteActivity(ctx, activities.SyncHolonDataActivity, input).Get(ctx, &result)
	if err != nil {
		logger.Error("Holon sync activity failed", "error", err)
		return HolonSyncWorkflowOutput{
			Success:      false,
			Error:        err.Error(),
			LastSyncTime: time.Now(),
		}, err
	}

	logger.Debug("Holon sync workflow completed successfully",
		"participants", result.ParticipantCount,
		"threads", result.ThreadCount,
		"replies", result.ReplyCount)

	return result, nil
}

// SyncHolonDataActivity performs the actual data synchronization
func (a *HolonSyncActivities) SyncHolonDataActivity(ctx context.Context, input HolonSyncWorkflowInput) (HolonSyncWorkflowOutput, error) {
	result := HolonSyncWorkflowOutput{
		Success:      true,
		LastSyncTime: time.Now(),
	}

	// Sync participants
	if err := a.SyncParticipants(ctx); err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to sync participants: %v", err)
		return result, err
	}

	// Sync threads
	if err := a.SyncThreads(ctx); err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to sync threads: %v", err)
		return result, err
	}

	// Sync replies
	if err := a.SyncReplies(ctx); err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to sync replies: %v", err)
		return result, err
	}

	return result, nil
}

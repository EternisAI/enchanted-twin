package holon

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// HolonSyncWorkflow orchestrates the periodic synchronization with HolonZero API.
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

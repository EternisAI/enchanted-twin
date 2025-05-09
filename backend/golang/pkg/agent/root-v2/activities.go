package root

import (
	"context"
	"errors"
	"fmt"

	"github.com/charmbracelet/log"
	"go.temporal.io/api/serviceerror" // Import Temporal service errors
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
)

type RootActivities struct {
	TemporalClient client.Client
	Logger         *log.Logger
}

func NewRootActivities(c client.Client, logger *log.Logger) *RootActivities {
	return &RootActivities{
		TemporalClient: c,
		Logger:         logger.With("component", "root-activities"),
	}
}

type TerminateWorkflowInput struct {
	WorkflowID string
	RunID      string
	Reason     string
}

// TerminateWorkflowActivity attempts to terminate a given workflow execution.
// It handles common errors gracefully for idempotency (e.g., NotFound).
func (a *RootActivities) TerminateWorkflowActivity(ctx context.Context, input TerminateWorkflowInput) error {
	sdkLogger := activity.GetLogger(ctx)
	logger := log.With(
		sdkLogger,
		"TargetWorkflowID", input.WorkflowID,
		"TargetRunID", input.RunID,
		"Reason", input.Reason,
	)
	logger.Info("Activity: Attempting to terminate workflow")

	// Call Temporal client to terminate
	err := a.TemporalClient.TerminateWorkflow(
		ctx,
		input.WorkflowID,
		input.RunID, // Pass RunID (if empty, terminates the current run)
		input.Reason,
		nil, // details - usually nil
	)
	if err != nil {
		// --- Handle Specific Errors for Idempotency ---

		var notFoundErr *serviceerror.NotFound
		if errors.As(err, &notFoundErr) {
			// Workflow doesn't exist. This could be because:
			// 1. The IDs were wrong.
			// 2. The workflow completed/failed/terminated long ago and was cleaned up by retention.
			// 3. The workflow completed/failed/terminated very recently.
			// In any case, the workflow is *not running*, so the goal of termination is achieved.
			logger.Warn("Activity: Workflow not found during termination attempt (considered idempotent success)", "error", err)
			return nil
		}

		// Check if the specific run is already completed (less common for Terminate, but possible)
		// Note: TerminateWorkflow might succeed even if the workflow is already completed,
		// or it might return NotFoundError depending on timing and server state.
		// Checking for NotFoundError is generally sufficient for idempotency.
		/*
			var alreadyCompletedErr *serviceerror.WorkflowExecutionAlreadyCompleted
			if errors.As(err, &alreadyCompletedErr) {
				logger.Info("Activity: Workflow already completed during termination attempt (idempotent success)")
				return nil // Indicate success as the desired state is met.
			}
		*/

		// --- Log and Return Genuine Errors ---
		// For any other error (permission denied, network issue after retries, invalid args, etc.),
		// log it as an error and return it to the workflow.
		logger.Error("Activity: Failed to terminate workflow with unhandled error", "error", err)
		// Wrap the error for clarity when it surfaces in the workflow.
		return fmt.Errorf("activity failed to terminate workflow %s (RunID: %s): %w", input.WorkflowID, input.RunID, err)
	}

	// If err is nil, the termination request was successfully sent/accepted by the Temporal server.
	logger.Info("Activity: Terminate request sent successfully to workflow")
	return nil // Explicit success
}

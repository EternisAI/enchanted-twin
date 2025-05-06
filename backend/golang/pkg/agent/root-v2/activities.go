package root

import (
	"context"
	"fmt"

	"github.com/charmbracelet/log"
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
	TargetWorkflowID string
	TargetRunID      string
	Reason           string
}

// TerminateWorkflowActivity is an activity that terminates a given workflow execution.
func (a *RootActivities) TerminateWorkflowActivity(ctx context.Context, input TerminateWorkflowInput) error {
	logger := log.With(
		activity.GetLogger(ctx),
		"TargetWorkflowID", input.TargetWorkflowID,
		"TargetRunID", input.TargetRunID,
		"Reason", input.Reason,
	)
	logger.Info("Activity: Attempting to terminate workflow")

	err := a.TemporalClient.TerminateWorkflow(ctx, input.TargetWorkflowID, input.TargetRunID, input.Reason, nil) // nil for details
	if err != nil {
		// Log the error but don't necessarily fail the activity if the workflow is already gone.
		// The parent workflow can decide how to interpret this.
		logger.Error("Activity: Failed to terminate workflow", "error", err)
		// We might want to check for specific errors like NotFoundError and return nil for idempotency.
		// For simplicity here, we'll just return the error. The workflow can handle it.
		return fmt.Errorf("activity failed to terminate workflow %s (RunID: %s): %w", input.TargetWorkflowID, input.TargetRunID, err)
	}

	logger.Info("Activity: Terminate signal sent successfully to workflow")
	return nil
}

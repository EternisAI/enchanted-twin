package helpers

import (
	"context"
	"time"

	"github.com/charmbracelet/log"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
)

func CreateScheduleIfNotExists(
	logger *log.Logger,
	temporalClient client.Client,
	scheduleID string,
	interval time.Duration,
	workflowName any,
	workflowArgs []any,
) error {
	ctx := context.Background()

	// Try to get handle to check if schedule exists
	scheduleHandle := temporalClient.ScheduleClient().GetHandle(ctx, scheduleID)

	// Check if schedule exists by attempting to describe it
	_, err := scheduleHandle.Describe(ctx)
	if err == nil {
		// Schedule already exists
		logger.Info("Schedule already exists, skipping creation", "scheduleID", scheduleID)
		return nil
	}

	// Schedule doesn't exist, create it
	_, err = temporalClient.ScheduleClient().Create(ctx, client.ScheduleOptions{
		ID: scheduleID,
		Spec: client.ScheduleSpec{
			Intervals: []client.ScheduleIntervalSpec{
				{
					Every: interval,
				},
			},
		},
		Action: &client.ScheduleWorkflowAction{
			ID:        scheduleID,
			Workflow:  workflowName,
			Args:      workflowArgs,
			TaskQueue: "default",
		},
		Overlap: enums.SCHEDULE_OVERLAP_POLICY_SKIP,
	})
	if err != nil {
		if err.Error() == "schedule with this ID is already registered" {
			logger.Info("Schedule already exists, skipping creation", "scheduleID", scheduleID)
			return nil
		}

		logger.Error("Error creating schedule", "error", err)
		return err
	}

	logger.Info("Schedule created", "scheduleID", scheduleID)
	return nil
}

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
	return CreateOrUpdateSchedule(logger, temporalClient, scheduleID, interval, workflowName, workflowArgs, false)
}

func CreateOrUpdateSchedule(
	logger *log.Logger,
	temporalClient client.Client,
	scheduleID string,
	interval time.Duration,
	workflowName any,
	workflowArgs []any,
	overrideIfDifferent bool,
) error {
	ctx := context.Background()

	// Try to get handle to check if schedule exists
	scheduleHandle := temporalClient.ScheduleClient().GetHandle(ctx, scheduleID)

	// Check if schedule exists by attempting to describe it
	desc, err := scheduleHandle.Describe(ctx)
	if err == nil {
		// Schedule exists
		if !overrideIfDifferent {
			logger.Info("Schedule already exists, skipping creation", "scheduleID", scheduleID)
			return nil
		}

		// Check if settings are different
		existingInterval := time.Duration(0)
		if len(desc.Schedule.Spec.Intervals) > 0 {
			existingInterval = desc.Schedule.Spec.Intervals[0].Every
		}

		if existingInterval == interval {
			logger.Info("Schedule already exists with same settings, skipping update",
				"scheduleID", scheduleID,
				"interval", interval)
			return nil
		}

		// Settings are different, delete and recreate
		logger.Info("Schedule exists with different settings, recreating",
			"scheduleID", scheduleID,
			"existingInterval", existingInterval,
			"newInterval", interval)

		err = scheduleHandle.Delete(ctx)
		if err != nil {
			logger.Error("Failed to delete existing schedule", "error", err, "scheduleID", scheduleID)
			return err
		}
		logger.Info("Deleted existing schedule", "scheduleID", scheduleID)
	}

	// Schedule doesn't exist or was deleted, create it
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

	logger.Info("Schedule created", "scheduleID", scheduleID, "interval", interval)
	return nil
}

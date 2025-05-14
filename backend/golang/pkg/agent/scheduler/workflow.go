package scheduler

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type TaskScheduleWorkflowInput struct {
	Task   string `json:"task"`
	Name   string `json:"name"`
	ChatID string `json:"chat_id"`
	Delay  int    `json:"delay"`
	Cron   string `json:"cron"`
	Notify bool   `json:"notify"`
}

type TaskScheduleWorkflowOutput struct {
	Result   string `json:"result"`
	Progress string `json:"progress"`
}

func TaskScheduleWorkflow(ctx workflow.Context, input *TaskScheduleWorkflowInput) (TaskScheduleWorkflowOutput, error) {
	var a *TaskSchedulerActivities
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 3 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    5,
		},
	})

	var lastWorkflowOutput *TaskScheduleWorkflowOutput
	if workflow.HasLastCompletionResult(ctx) {
		err := workflow.GetLastCompletionResult(ctx, &lastWorkflowOutput)
		if err != nil {
			return TaskScheduleWorkflowOutput{}, err
		}
	}

	var lastWorkflowResult *string
	if lastWorkflowOutput != nil {
		lastWorkflowResult = &lastWorkflowOutput.Result
	}

	var completion string
	executeTaskInput := ExecuteTaskActivityInput{
		Task:           input.Task,
		PreviousResult: lastWorkflowResult,
		ChatID:         input.ChatID,
		Notify:         input.Notify,
	}
	if err := workflow.ExecuteActivity(
		ctx,
		a.executeActivity,
		executeTaskInput,
	).Get(ctx, &completion); err != nil {
		return TaskScheduleWorkflowOutput{}, err
	}

	return TaskScheduleWorkflowOutput{
		Result:   completion,
		Progress: "Completed",
	}, nil
}

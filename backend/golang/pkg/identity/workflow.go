package identity

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	PersonalityWorkflowID = "identity-personality-workflow"
)

type DerivePersonalityOutput struct {
	Personality string `json:"personality"`
}

func DerivePersonalityWorkflow(ctx workflow.Context) (DerivePersonalityOutput, error) {
	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    5,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions)

	var a *identityActivities

	var personality string
	if err := workflow.ExecuteActivity(ctx, a.GeneratePersonalityActivity).Get(ctx, &personality); err != nil {
		return DerivePersonalityOutput{}, err
	}

	return DerivePersonalityOutput{
		Personality: personality,
	}, nil
}

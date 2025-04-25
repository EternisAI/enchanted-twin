package auth

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type TokenRefreshWorkflowInput struct {
	Provider string `json:"provider"`
}

func TokenRefreshWorkflow(ctx workflow.Context, input TokenRefreshWorkflowInput) (TokenRequest, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Started token refresh workflow", "provider", input.Provider)

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

	var a *OAuthActivities

	var tokenReq TokenRequest
	if err := workflow.ExecuteActivity(ctx, a.RefreshTokenActivity, input.Provider).Get(ctx, &tokenReq); err != nil {
		logger.Error("Failed to refresh token", "provider", input.Provider, "error", err)
		return TokenRequest{}, err
	}

	logger.Info("Successfully refreshed token", "provider", input.Provider)
	return tokenReq, nil
}

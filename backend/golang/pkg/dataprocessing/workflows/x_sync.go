package workflows

import (
	"time"

	"github.com/pkg/errors"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type XSyncWorkflowInput struct {
	Username string `json:"username"`
}

type XSyncWorkflowResponse struct {
	EndTime time.Time `json:"endTime"`
	Success bool      `json:"success"`
}

func (w *SyncWorkflows) XSyncWorkflow(ctx workflow.Context, input XSyncWorkflowInput) (XSyncWorkflowResponse, error) {
	if w.Store == nil {
		return XSyncWorkflowResponse{}, errors.New("store is nil")
	}

	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 120 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second * 2,
			MaximumInterval:    time.Minute * 10,
			BackoffCoefficient: 4,
			MaximumAttempts:    1,
		},
	})

	endTime := time.Now()

	return XSyncWorkflowResponse{EndTime: endTime, Success: true}, nil
}

type XSyncActivityInput struct {
	Username string `json:"username"`
}

type XSyncActivityResponse struct {
	Success bool `json:"success"`
}

func (w *SyncWorkflows) XSyncActivity(ctx workflow.Context, input XSyncActivityInput) (XSyncActivityResponse, error) {
	return XSyncActivityResponse{Success: true}, nil
}

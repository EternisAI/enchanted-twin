package workflows

import (
	"context"
	"time"

	dataprocessing "github.com/EternisAI/enchanted-twin/pkg/dataprocessing"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/gmail"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
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
	var response XFetchActivityResponse
	err := workflow.ExecuteActivity(ctx, w.XFetchActivity, XFetchActivityInput{Username: input.Username}).Get(ctx, &response)
	if err != nil {
		return XSyncWorkflowResponse{}, err
	}

	err = workflow.ExecuteActivity(ctx, w.XIndexActivity, XIndexActivityInput{Records: response.Records}).Get(ctx, nil)
	if err != nil {
		return XSyncWorkflowResponse{}, err
	}

	endTime := time.Now()

	return XSyncWorkflowResponse{EndTime: endTime, Success: true}, nil
}

type XFetchActivityInput struct {
	Username string `json:"username"`
}

type XFetchActivityResponse struct {
	Records []types.Record `json:"records"`
}

func (w *SyncWorkflows) XFetchActivity(ctx workflow.Context, input XFetchActivityInput) (XFetchActivityResponse, error) {
	records, err := dataprocessing.Sync("x", w.Store)
	if err != nil {
		return XFetchActivityResponse{}, err
	}
	return XFetchActivityResponse{Records: records}, nil
}

type XIndexActivityInput struct {
	Records []types.Record `json:"records"`
}

type XIndexActivityResponse struct{}

func (w *SyncWorkflows) XIndexActivity(ctx context.Context, input XIndexActivityInput) (XIndexActivityResponse, error) {
	documents, err := gmail.ToDocuments(input.Records)
	if err != nil {
		return XIndexActivityResponse{}, err
	}
	w.Logger.Info("Documents", "gmail", len(documents))
	err = w.Memory.Store(ctx, documents)
	if err != nil {
		return XIndexActivityResponse{}, err
	}
	w.Logger.Info("Indexed documents", "documents", len(documents))

	return XIndexActivityResponse{}, nil
}

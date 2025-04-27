package workflows

import (
	"context"
	"fmt"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/auth"
	dataprocessing "github.com/EternisAI/enchanted-twin/pkg/dataprocessing"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/x"
	"github.com/pkg/errors"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type XSyncWorkflowInput struct{}

type XSyncWorkflowResponse struct {
	EndTime             time.Time `json:"endTime"`
	Success             bool      `json:"success"`
	LastRecordID        string    `json:"lastRecordID"`
	LastRecordTimestamp time.Time `json:"lastRecordTimestamp"`
}

func (w *DataProcessingWorkflows) XSyncWorkflow(ctx workflow.Context, input XSyncWorkflowInput) (XSyncWorkflowResponse, error) {
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

	var previousResult XSyncWorkflowResponse

	if workflow.HasLastCompletionResult(ctx) {
		err := workflow.GetLastCompletionResult(ctx, &previousResult)
		if err != nil {
			return XSyncWorkflowResponse{}, err
		}
		workflow.GetLogger(ctx).Info("Recovered last result", "value", previousResult)
	}

	_, err := auth.RefreshExpiredTokens(context.Background(), w.Logger, w.Store)
	if err != nil {
		return XSyncWorkflowResponse{}, fmt.Errorf("failed to refresh expired tokens: %w", err)
	}

	workflowResponse := XSyncWorkflowResponse{
		LastRecordID:        previousResult.LastRecordID,
		LastRecordTimestamp: previousResult.LastRecordTimestamp,
	}

	var response XFetchActivityResponse
	err = workflow.ExecuteActivity(ctx, w.XFetchActivity, XFetchActivityInput{}).Get(ctx, &response)
	if err != nil {
		return workflowResponse, err
	}

	filteredRecords := []types.Record{}
	for _, record := range response.Records {
		if previousResult.LastRecordTimestamp.IsZero() {
			filteredRecords = append(filteredRecords, record)
			continue
		}

		if record.Timestamp.After(previousResult.LastRecordTimestamp) {
			filteredRecords = append(filteredRecords, record)
		}
	}

	if len(filteredRecords) == 0 {
		workflowResponse.EndTime = time.Now()
		return XSyncWorkflowResponse{
			EndTime:             time.Now(),
			Success:             true,
			LastRecordID:        previousResult.LastRecordID,
			LastRecordTimestamp: previousResult.LastRecordTimestamp,
		}, nil
	}

	w.Logger.Info("filteredRecords", "value", filteredRecords)
	err = workflow.ExecuteActivity(ctx, w.XIndexActivity, XIndexActivityInput{Records: filteredRecords}).Get(ctx, nil)
	if err != nil {
		return XSyncWorkflowResponse{}, err
	}

	lastRecord := response.Records[0]

	w.Logger.Info("lastRecord", "value", lastRecord)

	lastRecordID := ""
	if id, ok := lastRecord.Data["id"]; ok && id != nil {
		lastRecordID = id.(string)
	}
	workflowResponse.LastRecordID = lastRecordID
	workflowResponse.LastRecordTimestamp = lastRecord.Timestamp
	workflowResponse.Success = true
	workflowResponse.EndTime = time.Now()

	return workflowResponse, nil
}

type XFetchActivityInput struct {
	Username string `json:"username"`
}

type XFetchActivityResponse struct {
	Records []types.Record `json:"records"`
}

func (w *DataProcessingWorkflows) XFetchActivity(ctx context.Context, input XFetchActivityInput) (XFetchActivityResponse, error) {
	tokens, err := w.Store.GetOAuthTokens(ctx, "twitter")
	if err != nil {
		return XFetchActivityResponse{}, fmt.Errorf("failed to get OAuth tokens: %w", err)
	}
	if tokens == nil {
		return XFetchActivityResponse{}, fmt.Errorf("no OAuth tokens found for X")
	}
	w.Logger.Debug("XFetchActivity", "tokens", tokens)
	records, err := dataprocessing.Sync(ctx, "x", tokens.AccessToken)
	if err != nil {
		return XFetchActivityResponse{}, err
	}
	return XFetchActivityResponse{Records: records}, nil
}

type XIndexActivityInput struct {
	Records []types.Record `json:"records"`
}

type XIndexActivityResponse struct{}

func (w *DataProcessingWorkflows) XIndexActivity(ctx context.Context, input XIndexActivityInput) (XIndexActivityResponse, error) {
	documents, err := x.ToDocuments(input.Records)
	if err != nil {
		return XIndexActivityResponse{}, err
	}
	w.Logger.Info("X", "tweets", len(documents))
	err = w.Memory.Store(ctx, documents)
	if err != nil {
		return XIndexActivityResponse{}, err
	}

	return XIndexActivityResponse{}, nil
}

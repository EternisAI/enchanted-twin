package workflows

import (
	"context"
	"fmt"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/auth"
	dataprocessing "github.com/EternisAI/enchanted-twin/pkg/dataprocessing"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/gmail"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
	"github.com/pkg/errors"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type GmailSyncWorkflowInput struct{}

type GmailSyncWorkflowResponse struct {
	EndTime             time.Time `json:"endTime"`
	Success             bool      `json:"success"`
	LastRecordTimestamp time.Time `json:"lastRecordTimestamp"`
}

func (w *DataProcessingWorkflows) GmailSyncWorkflow(ctx workflow.Context, input GmailSyncWorkflowInput) (GmailSyncWorkflowResponse, error) {
	if w.Store == nil {
		return GmailSyncWorkflowResponse{}, errors.New("store is nil")
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

	var previousResult GmailSyncWorkflowResponse

	if workflow.HasLastCompletionResult(ctx) {
		err := workflow.GetLastCompletionResult(ctx, &previousResult)
		if err != nil {
			return GmailSyncWorkflowResponse{}, err
		}
		workflow.GetLogger(ctx).Info("Recovered last result", "value", previousResult)
	}

	_, err := auth.RefreshExpiredTokens(context.Background(), w.Logger, w.Store)
	if err != nil {
		return GmailSyncWorkflowResponse{}, fmt.Errorf("failed to refresh expired tokens: %w", err)
	}

	workflowResponse := GmailSyncWorkflowResponse{
		LastRecordTimestamp: previousResult.LastRecordTimestamp,
	}

	var response GmailFetchActivityResponse
	err = workflow.ExecuteActivity(ctx, w.GmailFetchActivity, GmailFetchActivityInput{}).Get(ctx, &response)
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
		return GmailSyncWorkflowResponse{
			EndTime:             time.Now(),
			Success:             true,
			LastRecordTimestamp: previousResult.LastRecordTimestamp,
		}, nil
	}

	w.Logger.Info("filteredRecords", "value", filteredRecords)
	err = workflow.ExecuteActivity(ctx, w.GmailIndexActivity, GmailIndexActivityInput{Records: filteredRecords}).Get(ctx, nil)
	if err != nil {
		return GmailSyncWorkflowResponse{}, err
	}

	lastRecord := response.Records[0]

	w.Logger.Info("lastRecord", "value", lastRecord)

	workflowResponse.LastRecordTimestamp = lastRecord.Timestamp
	workflowResponse.Success = true
	workflowResponse.EndTime = time.Now()

	return workflowResponse, nil
}

type GmailFetchActivityInput struct {
	Username string `json:"username"`
}

type GmailFetchActivityResponse struct {
	Records []types.Record `json:"records"`
}

func (w *DataProcessingWorkflows) GmailFetchActivity(ctx context.Context, input GmailFetchActivityInput) (GmailFetchActivityResponse, error) {
	tokens, err := w.Store.GetOAuthTokens(ctx, "google")
	if err != nil {
		return GmailFetchActivityResponse{}, fmt.Errorf("failed to get OAuth tokens: %w", err)
	}
	if tokens == nil {
		return GmailFetchActivityResponse{}, fmt.Errorf("no OAuth tokens found for Google")
	}

	records, err := dataprocessing.Sync(ctx, "gmail", tokens.AccessToken)
	if err != nil {
		return GmailFetchActivityResponse{}, err
	}
	return GmailFetchActivityResponse{Records: records}, nil
}

type GmailIndexActivityInput struct {
	Records []types.Record `json:"records"`
}

type GmailIndexActivityResponse struct{}

func (w *DataProcessingWorkflows) GmailIndexActivity(ctx context.Context, input GmailIndexActivityInput) (GmailIndexActivityResponse, error) {
	documents, err := gmail.ToDocuments(input.Records)
	if err != nil {
		return GmailIndexActivityResponse{}, err
	}
	w.Logger.Info("Gmail", "emails", len(documents))
	err = w.Memory.Store(ctx, documents)
	if err != nil {
		return GmailIndexActivityResponse{}, err
	}

	return GmailIndexActivityResponse{}, nil
}

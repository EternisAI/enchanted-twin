package workflows

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/EternisAI/enchanted-twin/pkg/auth"
	dataprocessing "github.com/EternisAI/enchanted-twin/pkg/dataprocessing"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
)

type DriveSyncWorkflowInput struct {
	Username string `json:"username"`
}

type DriveSyncWorkflowResponse struct {
	EndTime             time.Time `json:"endTime"`
	Success             bool      `json:"success"`
	LastRecordTimestamp time.Time `json:"lastRecordTimestamp"`
}

func (w *DataProcessingWorkflows) DriveSyncWorkflow(
	ctx workflow.Context,
	input DriveSyncWorkflowInput,
) (DriveSyncWorkflowResponse, error) {
	if w.Store == nil {
		return DriveSyncWorkflowResponse{}, errors.New("store is nil")
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
			return DriveSyncWorkflowResponse{}, err
		}
		workflow.GetLogger(ctx).Info("Recovered last result", "value", previousResult)
	}

	_, err := auth.RefreshExpiredTokens(context.Background(), w.Logger, w.Store)
	if err != nil {
		return DriveSyncWorkflowResponse{}, fmt.Errorf("failed to refresh expired tokens: %w", err)
	}

	workflowResponse := DriveSyncWorkflowResponse{
		LastRecordTimestamp: previousResult.LastRecordTimestamp,
	}

	var response DriveFetchActivityResponse
	err = workflow.ExecuteActivity(ctx, w.DriveFetchActivity, DriveFetchActivityInput(input)).
		Get(ctx, &response)
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
		return DriveSyncWorkflowResponse{
			EndTime:             time.Now(),
			Success:             true,
			LastRecordTimestamp: previousResult.LastRecordTimestamp,
		}, nil
	}

	w.Logger.Info("filteredRecords", "length", len(filteredRecords))
	err = workflow.ExecuteActivity(ctx, w.DriveIndexActivity, DriveIndexActivityInput{Records: filteredRecords}).Get(ctx, nil)
	if err != nil {
		return DriveSyncWorkflowResponse{}, err
	}

	lastRecord := response.Records[0]

	w.Logger.Debug("lastRecord", "value", lastRecord)

	workflowResponse.LastRecordTimestamp = lastRecord.Timestamp
	workflowResponse.Success = true
	workflowResponse.EndTime = time.Now()

	return workflowResponse, nil
}

type DriveFetchActivityInput struct {
	Username string `json:"username"`
}

type DriveFetchActivityResponse struct {
	Records []types.Record `json:"records"`
}

func (w *DataProcessingWorkflows) DriveFetchActivity(
	ctx context.Context,
	input DriveFetchActivityInput,
) (DriveFetchActivityResponse, error) {
	tokens, err := w.Store.GetOAuthTokensByUsername(ctx, "google", input.Username)
	if err != nil {
		return DriveFetchActivityResponse{}, fmt.Errorf("failed to get OAuth tokens: %w", err)
	}
	if tokens == nil {
		return DriveFetchActivityResponse{}, fmt.Errorf("no OAuth tokens found for Google")
	}

	dataprocessingService := dataprocessing.NewDataProcessingService(w.OpenAIService, w.Config.CompletionsModel, w.Store, w.Logger)
	records, err := dataprocessingService.Sync(ctx, "drive", tokens.AccessToken)
	if err != nil {
		return DriveFetchActivityResponse{}, err
	}
	return DriveFetchActivityResponse{Records: records}, nil
}

type DriveIndexActivityInput struct {
	Records []types.Record `json:"records"`
}

type DriveIndexActivityResponse struct{}

func (w *DataProcessingWorkflows) DriveIndexActivity(
	ctx context.Context,
	input DriveIndexActivityInput,
) (DriveIndexActivityResponse, error) {
	dataprocessingService := dataprocessing.NewDataProcessingService(w.OpenAIService, w.Config.CompletionsModel, w.Store, w.Logger)
	documents, err := dataprocessingService.ToDocuments(ctx, "drive", input.Records)
	if err != nil {
		return DriveIndexActivityResponse{}, err
	}

	err = w.Memory.Store(ctx, documents, nil)
	if err != nil {
		return DriveIndexActivityResponse{}, err
	}

	return DriveIndexActivityResponse{}, nil
}

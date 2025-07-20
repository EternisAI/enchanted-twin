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
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/constants"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
)

type XSyncWorkflowInput struct {
	Username string `json:"username"`
}

type XSyncWorkflowResponse struct {
	EndTime             time.Time `json:"endTime"`
	Success             bool      `json:"success"`
	LastRecordID        string    `json:"lastRecordID"`
	LastRecordTimestamp time.Time `json:"lastRecordTimestamp"`
}

func (w *DataProcessingWorkflows) XSyncWorkflow(
	ctx workflow.Context,
	input XSyncWorkflowInput,
) (XSyncWorkflowResponse, error) {
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

	workflowResponse := XSyncWorkflowResponse{
		LastRecordID:        previousResult.LastRecordID,
		LastRecordTimestamp: previousResult.LastRecordTimestamp,
	}

	var response XFetchActivityResponse
	err := workflow.ExecuteActivity(ctx, w.XFetchActivity, XFetchActivityInput(input)).Get(ctx, &response)
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
	err = workflow.ExecuteActivity(ctx, w.XIndexActivity, XIndexActivityInput{Records: filteredRecords}).
		Get(ctx, nil)
	if err != nil {
		return XSyncWorkflowResponse{}, err
	}

	lastRecord := response.Records[0]

	w.Logger.Info("lastRecord", "value", lastRecord)

	lastRecordID := ""
	if id, ok := lastRecord.Data["id"]; ok && id != nil {
		if idStr, ok := id.(string); ok {
			lastRecordID = idStr
		} else {
			// Try to convert to string if possible
			lastRecordID = fmt.Sprintf("%v", id)
			w.Logger.Warn("Expected string ID but got different type", "id_type", fmt.Sprintf("%T", id), "id_value", id)
		}
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

func (w *DataProcessingWorkflows) XFetchActivity(
	ctx context.Context,
	input XFetchActivityInput,
) (XFetchActivityResponse, error) {
	tokens, err := w.Store.GetOAuthTokensByUsername(ctx, "twitter", input.Username)
	if err != nil {
		return XFetchActivityResponse{}, fmt.Errorf("failed to get OAuth tokens: %w", err)
	}
	if tokens == nil {
		return XFetchActivityResponse{}, fmt.Errorf("no OAuth tokens found for X")
	}

	if tokens.ExpiresAt.Before(time.Now()) || tokens.Error {
		w.Logger.Debug("Refreshing expired Twitter token in X fetch activity")
		_, err = auth.RefreshOAuthToken(ctx, w.Logger, w.Store, "twitter")
		if err != nil {
			return XFetchActivityResponse{}, fmt.Errorf("failed to refresh OAuth tokens: %w", err)
		}
		tokens, err = w.Store.GetOAuthTokensByUsername(ctx, "twitter", input.Username)
		if err != nil {
			return XFetchActivityResponse{}, fmt.Errorf("failed to get refreshed OAuth tokens: %w", err)
		}
	}
	w.Logger.Debug("XFetchActivity", "tokens", tokens)

	dataprocessingService := dataprocessing.NewDataProcessingService(w.OpenAIService, w.Config.CompletionsModel, w.Store, w.Memory, w.Logger)
	records, err := dataprocessingService.Sync(ctx, constants.ProcessorX.String(), tokens.AccessToken)
	if err != nil {
		return XFetchActivityResponse{}, err
	}
	return XFetchActivityResponse{Records: records}, nil
}

type XIndexActivityInput struct {
	Records []types.Record `json:"records"`
}

type XIndexActivityResponse struct{}

func (w *DataProcessingWorkflows) XIndexActivity(
	ctx context.Context,
	input XIndexActivityInput,
) (XIndexActivityResponse, error) {
	dataprocessingService := dataprocessing.NewDataProcessingService(w.OpenAIService, w.Config.CompletionsModel, w.Store, w.Memory, w.Logger)
	documents, err := dataprocessingService.ToDocuments(ctx, constants.ProcessorX.String(), input.Records)
	if err != nil {
		return XIndexActivityResponse{}, err
	}

	err = w.Memory.Store(ctx, documents, nil)
	if err != nil {
		return XIndexActivityResponse{}, err
	}

	return XIndexActivityResponse{}, nil
}

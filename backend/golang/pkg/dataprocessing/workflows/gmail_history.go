package workflows

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	dataprocessing "github.com/EternisAI/enchanted-twin/pkg/dataprocessing"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
)

const MaxTemporalInputSizeBytes = 1900 * 1024

type GmailHistoryWorkflowInput struct {
	Username string `json:"username"`
}

type GmailHistoryWorkflowResponse struct{}

func (w *DataProcessingWorkflows) GmailHistoryWorkflow(
	ctx workflow.Context,
	input GmailHistoryWorkflowInput,
) (GmailHistoryWorkflowResponse, error) {
	if w.Store == nil {
		return GmailHistoryWorkflowResponse{}, errors.New("store is nil")
	}

	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 3 * 60 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second * 2,
			MaximumInterval:    time.Minute * 10,
			BackoffCoefficient: 4,
			MaximumAttempts:    1,
		},
	})

	daysBefore := 365
	windowSizeDays := 7
	limit := 10000

	var allRecords []types.Record
	processedMessageIds := make(map[string]bool)

	totalWindows := (daysBefore + windowSizeDays - 1) / windowSizeDays

	for window := 0; window < totalWindows; window++ {
		endDaysAgo := window * windowSizeDays

		startDaysAgo := endDaysAgo + windowSizeDays

		if startDaysAgo > daysBefore {
			startDaysAgo = daysBefore
		}

		startDate := time.Now().AddDate(0, 0, -startDaysAgo)
		endDate := time.Now().AddDate(0, 0, -endDaysAgo)

		startDateStr := startDate.Format("2006-01-02")
		endDateStr := endDate.Format("2006-01-02")

		w.Logger.Info("Fetching window", "startDate", startDateStr, "endDate", endDateStr)

		nextPageToken := ""
		hasMore := true

		for hasMore && len(allRecords) < limit {
			var response GmailHistoryFetchActivityResponse
			err := workflow.ExecuteActivity(ctx, w.GmailFetchHistoryActivity, GmailHistoryFetchActivityInput{
				Username:      input.Username,
				StartDate:     startDateStr,
				EndDate:       endDateStr,
				NextPageToken: nextPageToken,
			}).
				Get(ctx, &response)
			if err != nil {
				return GmailHistoryWorkflowResponse{}, err
			}

			var uniqueRecords []types.Record
			for _, record := range response.Records {
				if messageId, ok := record.Data["messageId"].(string); ok && messageId != "" {
					if !processedMessageIds[messageId] {
						processedMessageIds[messageId] = true
						uniqueRecords = append(uniqueRecords, record)
					}
				} else {
					uniqueRecords = append(uniqueRecords, record)
				}
			}

			allRecords = append(allRecords, uniqueRecords...)
			hasMore = response.More
			nextPageToken = response.NextPageToken

			if len(allRecords) >= limit {
				break
			}

			err = workflow.ExecuteActivity(ctx, w.GmailHistoryIndexActivity, GmailHistoryIndexActivityInput{Records: uniqueRecords}).Get(ctx, nil)
			if err != nil {
				return GmailHistoryWorkflowResponse{}, err
			}
		}

		if len(allRecords) >= limit {
			break
		}
	}

	return GmailHistoryWorkflowResponse{}, nil
}

type GmailHistoryFetchActivityInput struct {
	Username      string `json:"username"`
	StartDate     string `json:"startDate"`
	EndDate       string `json:"endDate"`
	NextPageToken string `json:"nextPageToken"`
}

type GmailHistoryFetchActivityResponse struct {
	Records       []types.Record `json:"records"`
	NextPageToken string         `json:"nextPageToken"`
	More          bool           `json:"more"`
}

func (w *DataProcessingWorkflows) GmailFetchHistoryActivity(
	ctx context.Context,
	input GmailHistoryFetchActivityInput,
) (GmailHistoryFetchActivityResponse, error) {
	tokens, err := w.Store.GetOAuthTokensByUsername(ctx, "google", input.Username)
	if err != nil {
		return GmailHistoryFetchActivityResponse{}, fmt.Errorf("failed to get OAuth tokens: %w", err)
	}
	if tokens == nil {
		return GmailHistoryFetchActivityResponse{}, fmt.Errorf("no OAuth tokens found for Google")
	}

	// Gmail no longer supports SyncWithDateRange - needs to be updated to new interface
	return GmailHistoryFetchActivityResponse{}, fmt.Errorf("gmail processor has been upgraded to new DocumentProcessor interface - workflow needs updating")
}

type GmailHistoryIndexActivityInput struct {
	Records []types.Record `json:"records"`
}

type GmailHistoryIndexActivityResponse struct{}

func (w *DataProcessingWorkflows) GmailHistoryIndexActivity(
	ctx context.Context,
	input GmailHistoryIndexActivityInput,
) (GmailHistoryIndexActivityResponse, error) {
	dataprocessingService := dataprocessing.NewDataProcessingService(w.OpenAIService, w.Config.CompletionsModel, w.Store, w.Logger)
	documents, err := dataprocessingService.ToDocuments(ctx, "gmail", input.Records)
	if err != nil {
		return GmailHistoryIndexActivityResponse{}, err
	}

	err = w.Memory.Store(ctx, documents, nil)
	if err != nil {
		return GmailHistoryIndexActivityResponse{}, err
	}

	return GmailHistoryIndexActivityResponse{}, nil
}

package workflows

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/gmail"
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
	windowSizeDays := 30

	err := workflow.ExecuteActivity(ctx, w.GmailFetchAndIndexActivity, GmailFetchAndIndexActivityInput{
		Username:       input.Username,
		DaysBefore:     daysBefore,
		WindowSizeDays: windowSizeDays,
	}).Get(ctx, nil)
	if err != nil {
		return GmailHistoryWorkflowResponse{}, err
	}

	return GmailHistoryWorkflowResponse{}, nil
}

type GmailFetchAndIndexActivityInput struct {
	Username       string `json:"username"`
	DaysBefore     int    `json:"daysBefore"`
	WindowSizeDays int    `json:"windowSizeDays"`
}

type GmailFetchAndIndexActivityResponse struct {
	TotalRecords int `json:"totalRecords"`
}

func (w *DataProcessingWorkflows) GmailFetchAndIndexActivity(
	ctx context.Context,
	input GmailFetchAndIndexActivityInput,
) (GmailFetchAndIndexActivityResponse, error) {
	tokens, err := w.Store.GetOAuthTokensByUsername(ctx, "google", input.Username)
	if err != nil {
		return GmailFetchAndIndexActivityResponse{}, fmt.Errorf("failed to get OAuth tokens: %w", err)
	}
	if tokens == nil {
		return GmailFetchAndIndexActivityResponse{}, fmt.Errorf("no OAuth tokens found for Google")
	}

	g := gmail.New()
	totalProcessed := 0

	totalWindows := (input.DaysBefore + input.WindowSizeDays - 1) / input.WindowSizeDays

	totalRecords := make([]types.Record, 0)

	seenMessageIDs := make(map[string]bool)

	for window := 0; window < totalWindows; window++ {
		endDaysAgo := window * input.WindowSizeDays
		startDaysAgo := endDaysAgo + input.WindowSizeDays

		if startDaysAgo > input.DaysBefore {
			startDaysAgo = input.DaysBefore
		}

		startDate := time.Now().AddDate(0, 0, -startDaysAgo)
		endDate := time.Now().AddDate(0, 0, -endDaysAgo)

		startDateStr := startDate.Format("2006-01-02")
		endDateStr := endDate.Format("2006-01-02")

		w.Logger.Info("Fetching emails for range", "startDate", startDateStr, "endDate", endDateStr)

		limit := 10000
		nextPageToken := ""
		windowProcessed := 0

		windowRecords := make([]types.Record, 0)
		for windowProcessed < limit {
			records, more, token, err := g.SyncWithDateRange(
				ctx,
				tokens.AccessToken,
				startDateStr,
				endDateStr,
				50,
				nextPageToken,
			)
			if err != nil {
				return GmailFetchAndIndexActivityResponse{}, err
			}

			for _, record := range records {
				if msgID, ok := record.Data["message_id"].(string); ok {
					if !seenMessageIDs[msgID] {
						seenMessageIDs[msgID] = true
						windowRecords = append(windowRecords, record)
					} else {
						w.Logger.Debug("Skipping duplicate email", "message_id", msgID)
					}
				} else {

					windowRecords = append(windowRecords, record)
					w.Logger.Warn("Email missing message_id", "subject", record.Data["subject"])
				}
			}

			windowProcessed += len(records)
			nextPageToken = token

			if !more || windowProcessed >= limit {
				break
			}
		}

		w.Logger.Info("== Window records processed", "total", len(windowRecords))
		totalRecords = append(totalRecords, windowRecords...)
	}

	if len(totalRecords) == 0 {
		return GmailFetchAndIndexActivityResponse{}, nil
	}
	w.Logger.Info("== Total records to index", "total", len(totalRecords))

	documents, err := gmail.ToDocuments(totalRecords)
	if err != nil {
		return GmailFetchAndIndexActivityResponse{}, err
	}

	progressChan := make(chan memory.ProgressUpdate, 10)
	err = w.Memory.Store(ctx, documents, progressChan)
	if err != nil {
		return GmailFetchAndIndexActivityResponse{}, err
	}

	totalProcessed += len(totalRecords)

	return GmailFetchAndIndexActivityResponse{
		TotalRecords: totalProcessed,
	}, nil
}

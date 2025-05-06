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

	monthsBefore := 12
	windowSize := 1
	limit := 10000

	var allRecords []types.Record

	totalWindows := (monthsBefore + windowSize - 1) / windowSize

	for window := 0; window < totalWindows; window++ {
		endMonthsAgo := window * windowSize

		startMonthsAgo := endMonthsAgo + windowSize

		if startMonthsAgo > monthsBefore {
			startMonthsAgo = monthsBefore
		}

		startDate := time.Now().AddDate(0, -startMonthsAgo, 0)
		endDate := time.Now().AddDate(0, -endMonthsAgo, 0)

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

			allRecords = append(allRecords, response.Records...)
			hasMore = response.More
			nextPageToken = response.NextPageToken

			if len(allRecords) >= limit {
				break
			}

			workflow.Sleep(ctx, 1*time.Second)
		}

		if len(allRecords) >= limit {
			break
		}
	}

	gmail.SortByOldest(allRecords)
	w.Logger.Info("All records", "value", len(allRecords))

	err := workflow.ExecuteActivity(ctx, w.GmailIndexActivity, GmailIndexActivityInput{Records: allRecords}).Get(ctx, nil)
	if err != nil {
		return GmailHistoryWorkflowResponse{}, err
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

	g := gmail.New()

	records, more, token, err := g.SyncWithDateRange(ctx, tokens.AccessToken, input.StartDate, input.EndDate, 50, input.NextPageToken)
	if err != nil {
		return GmailHistoryFetchActivityResponse{}, err
	}

	return GmailHistoryFetchActivityResponse{Records: records, NextPageToken: token, More: more}, nil
}

type GmailHistoryIndexActivityInput struct {
	Records []types.Record `json:"records"`
}

type GmailHistoryIndexActivityResponse struct{}

func (w *DataProcessingWorkflows) GmailHistoryIndexActivity(
	ctx context.Context,
	input GmailIndexActivityInput,
) (GmailIndexActivityResponse, error) {
	documents, err := gmail.ToDocuments(input.Records)
	if err != nil {
		return GmailIndexActivityResponse{}, err
	}

	progressChan := make(chan memory.ProgressUpdate, 10)
	err = w.Memory.Store(ctx, documents, progressChan)
	if err != nil {
		return GmailIndexActivityResponse{}, err
	}

	return GmailIndexActivityResponse{}, nil
}

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

	monthsBefore := 3
	limit := 10000

	endDate := time.Now().AddDate(0, -monthsBefore, 0)
	endDateStr := endDate.Format("2006-01-02")

	var allRecords []types.Record
	nextPageToken := ""
	hasMore := true

	for hasMore && len(allRecords) < limit {

		var response GmailHistoryFetchActivityResponse
		err := workflow.ExecuteActivity(ctx, w.GmailFetchHistoryActivity, GmailHistoryFetchActivityInput{
			Username:      input.Username,
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

		gmail.SortByOldest(allRecords)

		workflow.Sleep(ctx, 1*time.Second)

	}

	w.Logger.Info("All records", "value", len(allRecords))

	err := workflow.ExecuteActivity(ctx, w.GmailIndexActivity, GmailIndexActivityInput{Records: allRecords}).Get(ctx, nil)
	if err != nil {
		return GmailHistoryWorkflowResponse{}, err
	}

	return GmailHistoryWorkflowResponse{}, nil
}

type GmailHistoryFetchActivityInput struct {
	Username      string `json:"username"`
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

	records, more, token, err := g.SyncWithDateRange(ctx, tokens.AccessToken, "", input.EndDate, 50, input.NextPageToken)
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

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

	var allDocuments []memory.ConversationDocument
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

		for hasMore && len(allDocuments) < limit {
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

			var uniqueDocuments []memory.ConversationDocument
			for _, document := range response.Documents {
				// Use the document ID for deduplication instead of messageId
				docID := document.ID()
				if docID != "" {
					if !processedMessageIds[docID] {
						processedMessageIds[docID] = true
						uniqueDocuments = append(uniqueDocuments, document)
					}
				} else {
					uniqueDocuments = append(uniqueDocuments, document)
				}
			}

			allDocuments = append(allDocuments, uniqueDocuments...)
			hasMore = response.More
			nextPageToken = response.NextPageToken

			if len(allDocuments) >= limit {
				break
			}

			err = workflow.ExecuteActivity(ctx, w.GmailHistoryIndexActivity, GmailHistoryIndexActivityInput{Documents: uniqueDocuments}).Get(ctx, nil)
			if err != nil {
				return GmailHistoryWorkflowResponse{}, err
			}
		}

		if len(allDocuments) >= limit {
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
	Documents     []memory.ConversationDocument `json:"documents"`
	NextPageToken string                        `json:"nextPageToken"`
	More          bool                          `json:"more"`
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

	// Create Gmail processor
	processor, err := gmail.NewGmailProcessor(w.Store, w.Logger)
	if err != nil {
		return GmailHistoryFetchActivityResponse{}, fmt.Errorf("failed to create Gmail processor: %w", err)
	}

	// Parse date strings to time.Time
	startDate, err := time.Parse("2006-01-02", input.StartDate)
	if err != nil {
		return GmailHistoryFetchActivityResponse{}, fmt.Errorf("failed to parse start date: %w", err)
	}

	endDate, err := time.Parse("2006-01-02", input.EndDate)
	if err != nil {
		return GmailHistoryFetchActivityResponse{}, fmt.Errorf("failed to parse end date: %w", err)
	}

	// Use new paginated method with date range
	result, err := processor.SyncWithDateRangePaginated(ctx, tokens.AccessToken, startDate, endDate, input.NextPageToken)
	if err != nil {
		return GmailHistoryFetchActivityResponse{}, fmt.Errorf("failed to sync Gmail with date range: %w", err)
	}

	return GmailHistoryFetchActivityResponse{
		Documents:     result.Documents,
		NextPageToken: result.NextPageToken,
		More:          result.HasMore,
	}, nil
}

type GmailHistoryIndexActivityInput struct {
	Documents []memory.ConversationDocument `json:"documents"`
}

type GmailHistoryIndexActivityResponse struct{}

func (w *DataProcessingWorkflows) GmailHistoryIndexActivity(
	ctx context.Context,
	input GmailHistoryIndexActivityInput,
) (GmailHistoryIndexActivityResponse, error) {
	// Skip DataProcessingService entirely and store documents directly in memory
	var memoryDocs []memory.Document
	for _, doc := range input.Documents {
		// Create a copy to avoid pointer issues
		docCopy := doc
		memoryDocs = append(memoryDocs, &docCopy)
	}

	err := w.Memory.Store(ctx, memoryDocs, nil)
	if err != nil {
		return GmailHistoryIndexActivityResponse{}, fmt.Errorf("failed to store documents in memory: %w", err)
	}

	return GmailHistoryIndexActivityResponse{}, nil
}

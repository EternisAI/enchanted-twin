package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
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

			err = workflow.ExecuteActivity(ctx, w.GmailIndexActivity, GmailIndexActivityInput{Records: uniqueRecords}).Get(ctx, nil)
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

	g := gmail.New()

	records, more, token, err := g.SyncWithDateRange(ctx, tokens.AccessToken, input.StartDate, input.EndDate, 50, input.NextPageToken)
	if err != nil {
		return GmailHistoryFetchActivityResponse{}, err
	}

	trimmedRecords, err := ensureRecordsUnderSizeLimit(records)
	if err != nil {
		return GmailHistoryFetchActivityResponse{}, fmt.Errorf("failed to process records size: %w", err)
	}

	if len(trimmedRecords) < len(records) {
		w.Logger.Info("Trimmed oversized records payload",
			"original_count", len(records),
			"trimmed_count", len(trimmedRecords))
	}

	return GmailHistoryFetchActivityResponse{Records: trimmedRecords, NextPageToken: token, More: more}, nil
}

// Ensures that the records payload is under the Temporal size limit.
func ensureRecordsUnderSizeLimit(records []types.Record) ([]types.Record, error) {
	if len(records) == 0 {
		return records, nil
	}

	totalSize, recordSizes, err := calculateRecordsSize(records)
	if err != nil {
		return nil, err
	}

	if totalSize <= MaxTemporalInputSizeBytes {
		return records, nil
	}

	type recordWithSize struct {
		record types.Record
		size   int
		index  int
	}

	recordsWithSize := make([]recordWithSize, len(records))
	for i, size := range recordSizes {
		recordsWithSize[i] = recordWithSize{
			record: records[i],
			size:   size,
			index:  i,
		}
	}

	sort.Slice(recordsWithSize, func(i, j int) bool {
		return recordsWithSize[i].size > recordsWithSize[j].size
	})

	resultRecords := make([]types.Record, len(records))
	copy(resultRecords, records)

	for i := 0; totalSize > MaxTemporalInputSizeBytes && i < len(recordsWithSize); i++ {
		idx := recordsWithSize[i].index
		totalSize -= recordsWithSize[i].size

		resultRecords[idx] = types.Record{}
	}

	filteredRecords := make([]types.Record, 0, len(resultRecords))
	for _, r := range resultRecords {
		if r.Source != "" || r.Timestamp != (time.Time{}) || len(r.Data) > 0 {
			filteredRecords = append(filteredRecords, r)
		}
	}

	return filteredRecords, nil
}

func calculateRecordsSize(records []types.Record) (int, []int, error) {
	totalSize := 0
	recordSizes := make([]int, len(records))

	for i, record := range records {
		data, err := json.Marshal(record)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to marshal record for size calculation: %w", err)
		}

		size := len(data)
		recordSizes[i] = size
		totalSize += size
	}

	return totalSize, recordSizes, nil
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

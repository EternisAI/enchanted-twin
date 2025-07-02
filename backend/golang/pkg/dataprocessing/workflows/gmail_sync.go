package workflows

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/gmail"
)

type GmailSyncWorkflowInput struct {
	Username  string     `json:"username"`
	StartDate *time.Time `json:"startDate,omitempty"` // Optional date range
	EndDate   *time.Time `json:"endDate,omitempty"`   // Optional date range
}

type GmailSyncWorkflowResponse struct {
	EndTime             time.Time `json:"endTime"`
	Success             bool      `json:"success"`
	ProcessedDocuments  int       `json:"processedDocuments"`
	LastRecordTimestamp time.Time `json:"lastRecordTimestamp"`
}

// New unified sync activity that works with DocumentProcessor interface.
type GmailSyncActivityInput struct {
	Source    string     `json:"source"`
	Username  string     `json:"username"`
	StartDate *time.Time `json:"startDate,omitempty"`
	EndDate   *time.Time `json:"endDate,omitempty"`
}

type GmailSyncActivityResponse struct {
	Documents []memory.ConversationDocument `json:"documents"`
}

func (w *DataProcessingWorkflows) GmailSyncActivity(
	ctx context.Context,
	input GmailSyncActivityInput,
) (GmailSyncActivityResponse, error) {
	// Get OAuth tokens
	tokens, err := w.Store.GetOAuthTokensByUsername(ctx, "google", input.Username)
	if err != nil {
		return GmailSyncActivityResponse{}, fmt.Errorf("failed to get OAuth tokens: %w", err)
	}
	if tokens == nil {
		return GmailSyncActivityResponse{}, fmt.Errorf("no OAuth tokens found for Google")
	}

	// Create the appropriate processor
	var processor dataprocessing.DocumentProcessor
	switch input.Source {
	case "gmail":
		processor, err = gmail.NewGmailProcessor(w.Store, w.Logger)
		if err != nil {
			return GmailSyncActivityResponse{}, fmt.Errorf("failed to create Gmail processor: %w", err)
		}
	default:
		return GmailSyncActivityResponse{}, fmt.Errorf("unsupported source: %s", input.Source)
	}

	var documents []memory.ConversationDocument

	// Check if processor supports live sync at all
	liveSyncProcessor, supportsSync := processor.(dataprocessing.LiveSync)
	if !supportsSync {
		return GmailSyncActivityResponse{}, fmt.Errorf("processor %s does not support live sync", input.Source)
	}

	// Check if processor supports date range sync and dates are provided
	if dateRangeProcessor, ok := processor.(dataprocessing.DateRangeSync); ok && input.StartDate != nil && input.EndDate != nil {
		w.Logger.Info("Using date range sync", "source", input.Source, "startDate", input.StartDate, "endDate", input.EndDate)
		documents, err = dateRangeProcessor.SyncWithDateRange(ctx, tokens.AccessToken, *input.StartDate, *input.EndDate)
	} else {
		w.Logger.Info("Using regular sync", "source", input.Source)
		documents, err = liveSyncProcessor.Sync(ctx, tokens.AccessToken)
	}

	if err != nil {
		return GmailSyncActivityResponse{}, fmt.Errorf("failed to sync %s: %w", input.Source, err)
	}

	return GmailSyncActivityResponse{Documents: documents}, nil
}

func (w *DataProcessingWorkflows) GmailSyncWorkflow(
	ctx workflow.Context,
	input GmailSyncWorkflowInput,
) (GmailSyncWorkflowResponse, error) {
	if w.Store == nil {
		return GmailSyncWorkflowResponse{}, fmt.Errorf("store is nil")
	}

	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 120 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second * 2,
			MaximumInterval:    time.Minute * 10,
			BackoffCoefficient: 2,
			MaximumAttempts:    3,
		},
	})

	// Execute the new unified sync activity
	var syncResponse GmailSyncActivityResponse
	err := workflow.ExecuteActivity(ctx, w.GmailSyncActivity, GmailSyncActivityInput{
		Source:    "gmail",
		Username:  input.Username,
		StartDate: input.StartDate,
		EndDate:   input.EndDate,
	}).Get(ctx, &syncResponse)
	if err != nil {
		return GmailSyncWorkflowResponse{}, err
	}

	// Store documents directly in memory
	if len(syncResponse.Documents) > 0 {
		var memoryDocs []memory.Document
		for _, doc := range syncResponse.Documents {
			memoryDocs = append(memoryDocs, &doc)
		}

		err = workflow.ExecuteActivity(ctx, w.GmailStoreActivity, GmailStoreActivityInput{
			Documents: memoryDocs,
		}).Get(ctx, nil)
		if err != nil {
			return GmailSyncWorkflowResponse{}, err
		}
	}

	// Find the latest timestamp
	var lastTimestamp time.Time
	for _, doc := range syncResponse.Documents {
		if docTime := doc.Timestamp(); docTime != nil && docTime.After(lastTimestamp) {
			lastTimestamp = *docTime
		}
	}

	return GmailSyncWorkflowResponse{
		EndTime:             time.Now(),
		Success:             true,
		ProcessedDocuments:  len(syncResponse.Documents),
		LastRecordTimestamp: lastTimestamp,
	}, nil
}

// Activity to store documents in memory storage.
type GmailStoreActivityInput struct {
	Documents []memory.Document `json:"documents"`
}

func (w *DataProcessingWorkflows) GmailStoreActivity(
	ctx context.Context,
	input GmailStoreActivityInput,
) error {
	return w.Memory.Store(ctx, input.Documents, nil)
}

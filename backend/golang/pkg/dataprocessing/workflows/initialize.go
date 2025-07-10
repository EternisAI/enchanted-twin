package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/EternisAI/enchanted-twin/graph/model"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	dataprocessing "github.com/EternisAI/enchanted-twin/pkg/dataprocessing"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/constants"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/helpers"
	"github.com/EternisAI/enchanted-twin/pkg/db"
)

type InitializeWorkflowInput struct{}

// DataSourceResult represents the detailed result of processing a single data source.
type DataSourceResult struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Path              string `json:"path"`
	ProcessingStatus  string `json:"processing_status"` // "not_started", "completed", "failed", "skipped"
	IndexingStatus    string `json:"indexing_status"`   // "not_started", "completed", "failed", "skipped"
	IsProcessed       bool   `json:"is_processed"`
	IsIndexed         bool   `json:"is_indexed"`
	HasError          bool   `json:"has_error"`
	ErrorMessage      string `json:"error_message,omitempty"`
	DocumentsStored   int    `json:"documents_stored"`   // Number of documents successfully indexed
	ProcessingSkipped bool   `json:"processing_skipped"` // True if skipped due to concurrent processing
	IndexingSkipped   bool   `json:"indexing_skipped"`   // True if skipped due to concurrent indexing
}

// InitializeWorkflowResponse provides comprehensive results of the initialization process.
type InitializeWorkflowResponse struct {
	Message           string             `json:"message"`
	ProcessedSources  []DataSourceResult `json:"processed_sources"`  // Successfully processed and indexed
	FailedSources     []DataSourceResult `json:"failed_sources"`     // Failed during processing or indexing
	SkippedSources    []DataSourceResult `json:"skipped_sources"`    // Skipped due to concurrent processing
	TotalSources      int                `json:"total_sources"`      // Total number of data sources found
	SuccessfulSources int                `json:"successful_sources"` // Number of successfully processed sources
	FailedCount       int                `json:"failed_count"`       // Number of failed sources
	SkippedCount      int                `json:"skipped_count"`      // Number of skipped sources
}

type InitializeStateQuery struct {
	State model.IndexingState
}

func (w *DataProcessingWorkflows) InitializeWorkflow(
	ctx workflow.Context,
	input InitializeWorkflowInput,
) (InitializeWorkflowResponse, error) {
	if w.Store == nil {
		return InitializeWorkflowResponse{}, errors.New("store is nil")
	}

	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout:    5 * time.Minute,
		ScheduleToStartTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second * 2,
			MaximumInterval:    time.Minute * 10,
			BackoffCoefficient: 4,
			MaximumAttempts:    3,
		},
	})

	workflowID := workflow.GetInfo(ctx).WorkflowExecution.ID

	indexingState := model.IndexingStateNotStarted
	dataSources := []*model.DataSource{}
	dataSourceResults := make(map[string]*DataSourceResult) // Track results by ID
	w.publishIndexingStatus(ctx, indexingState, dataSources, nil)

	err := workflow.SetQueryHandler(
		ctx,
		"getInitializeState",
		func() (InitializeStateQuery, error) {
			return InitializeStateQuery{State: indexingState}, nil
		},
	)
	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to set query handler", "error", err)
		return InitializeWorkflowResponse{}, errors.Wrap(err, "failed to set query handler")
	}

	var cleanupResponse CleanupStaleProcessingResponse
	err = workflow.ExecuteActivity(ctx, w.CleanupStaleProcessingActivity, CleanupStaleProcessingInput{
		StaleAfterMinutes: 60,
	}).Get(ctx, &cleanupResponse)
	if err != nil {
		workflow.GetLogger(ctx).Warn("Failed to cleanup stale processing data sources", "error", err)
	} else if cleanupResponse.CleanedCount > 0 {
		workflow.GetLogger(ctx).Info("Cleaned up stale processing data sources", "count", cleanupResponse.CleanedCount)
	}

	var fetchDataSourcesResponse FetchDataSourcesActivityResponse
	err = workflow.ExecuteActivity(ctx, w.FetchDataSourcesActivity, FetchDataSourcesActivityInput{}).
		Get(ctx, &fetchDataSourcesResponse)
	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to fetch data sources", "error", err)
		indexingState = model.IndexingStateFailed
		errMsg := err.Error()
		w.publishIndexingStatus(ctx, indexingState, dataSources, &errMsg)
		return InitializeWorkflowResponse{}, errors.Wrap(err, "failed to fetch data sources")
	}

	if len(fetchDataSourcesResponse.DataSources) == 0 {
		indexingState = model.IndexingStateFailed
		errMsg := "No data sources found"
		w.publishIndexingStatus(ctx, indexingState, dataSources, &errMsg)
		return InitializeWorkflowResponse{
			Message:           errMsg,
			ProcessedSources:  []DataSourceResult{},
			FailedSources:     []DataSourceResult{},
			SkippedSources:    []DataSourceResult{},
			TotalSources:      0,
			SuccessfulSources: 0,
			FailedCount:       0,
			SkippedCount:      0,
		}, nil
	}

	for _, dataSource := range fetchDataSourcesResponse.DataSources {
		dataSources = append(dataSources, &model.DataSource{
			ID:          dataSource.ID,
			Name:        dataSource.Name,
			Path:        dataSource.Path,
			IsProcessed: false,
			IsIndexed:   *dataSource.IsIndexed,
			HasError:    *dataSource.HasError,
		})

		dataSourceResults[dataSource.ID] = &DataSourceResult{
			ID:               dataSource.ID,
			Name:             dataSource.Name,
			Path:             dataSource.Path,
			ProcessingStatus: "not_started",
			IndexingStatus:   "not_started",
			IsProcessed:      false,
			IsIndexed:        *dataSource.IsIndexed,
			HasError:         *dataSource.HasError,
			DocumentsStored:  0,
		}
	}

	indexingState = model.IndexingStateProcessingData
	w.publishIndexingStatus(ctx, indexingState, dataSources, nil)

	for i, dataSource := range fetchDataSourcesResponse.DataSources {
		w.publishIndexingStatus(ctx, indexingState, dataSources, nil)

		var claimResponse ClaimDataSourceResponse
		err = workflow.ExecuteActivity(ctx, w.ClaimDataSourceForProcessingActivity, ClaimDataSourceInput{
			DataSourceID: dataSource.ID,
			WorkflowID:   workflowID,
		}).Get(ctx, &claimResponse)
		if err != nil {
			workflow.GetLogger(ctx).Error("Failed to claim data source for processing",
				"error", err,
				"dataSource", dataSource.Name)
			dataSources[i].HasError = true
			errMsg := err.Error()
			if result, exists := dataSourceResults[dataSource.ID]; exists {
				result.ProcessingStatus = "failed"
				result.HasError = true
				result.ErrorMessage = errMsg
			}
			w.publishIndexingStatus(ctx, indexingState, dataSources, &errMsg)
			continue
		}

		if !claimResponse.Claimed {
			workflow.GetLogger(ctx).Info("Data source already claimed by another workflow",
				"dataSource", dataSource.Name)
			if result, exists := dataSourceResults[dataSource.ID]; exists {
				result.ProcessingStatus = "skipped"
				result.ProcessingSkipped = true
			}

			continue
		}

		processDataActivityInput := ProcessDataActivityInput{
			DataSourceName: dataSource.Name,
			SourcePath:     dataSource.Path,
			DataSourceID:   dataSource.ID,
			Username:       "xxx",
		}
		var processDataResponse ProcessDataActivityResponse
		err = workflow.ExecuteActivity(ctx, w.ProcessDataActivity, processDataActivityInput).
			Get(ctx, &processDataResponse)

		releaseErr := workflow.ExecuteActivity(ctx, w.ReleaseDataSourceActivity, ReleaseDataSourceInput{
			DataSourceID: dataSource.ID,
			WorkflowID:   workflowID,
		}).Get(ctx, nil)
		if releaseErr != nil {
			workflow.GetLogger(ctx).Error("Failed to release data source claim",
				"error", releaseErr,
				"dataSource", dataSource.Name)
		}

		dataSources[i].UpdatedAt = time.Now().Format(time.RFC3339)
		if err != nil {
			workflow.GetLogger(ctx).Error("Failed to process data source",
				"error", err,
				"dataSource", dataSource.Name)

			dataSources[i].IsProcessed = false
			dataSources[i].HasError = true
			errMsg := err.Error()
			if result, exists := dataSourceResults[dataSource.ID]; exists {
				result.ProcessingStatus = "failed"
				result.IsProcessed = false
				result.HasError = true
				result.ErrorMessage = errMsg
			}
			w.publishIndexingStatus(ctx, indexingState, dataSources, &errMsg)
		} else {
			dataSources[i].IsProcessed = true
			dataSources[i].HasError = false
			dataSources[i].IsIndexed = false
			if result, exists := dataSourceResults[dataSource.ID]; exists {
				result.ProcessingStatus = "completed"
				result.IsProcessed = true
				result.HasError = false
				result.ErrorMessage = ""
			}
			w.publishIndexingStatus(ctx, indexingState, dataSources, nil)
		}
	}

	indexingState = model.IndexingStateIndexingData
	w.publishIndexingStatus(ctx, indexingState, dataSources, nil)

	var fetchUnindexedResponse FetchDataSourcesActivityResponse
	err = workflow.ExecuteActivity(ctx, w.FetchDataSourcesActivity, FetchDataSourcesActivityInput{}).
		Get(ctx, &fetchUnindexedResponse)
	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to fetch unindexed data sources", "error", err)
		errMsg := err.Error()
		indexingState = model.IndexingStateFailed
		w.publishIndexingStatus(ctx, indexingState, dataSources, &errMsg)
		return InitializeWorkflowResponse{}, errors.Wrap(err, "failed to fetch unindexed data sources")
	}

	for i := range dataSources {
		dataSources[i].IndexProgress = 0
	}

	for i, dataSourceDB := range fetchUnindexedResponse.DataSources {
		if dataSourceDB.ProcessedPath == nil {
			workflow.GetLogger(ctx).Error("Processed path is nil", "dataSource", dataSourceDB.Name)
			continue
		}

		// Try to claim this data source for indexing
		var claimIndexResponse ClaimDataSourceResponse
		err = workflow.ExecuteActivity(ctx, w.ClaimDataSourceForIndexingActivity, ClaimDataSourceInput{
			DataSourceID: dataSourceDB.ID,
			WorkflowID:   workflowID,
		}).Get(ctx, &claimIndexResponse)
		if err != nil {
			workflow.GetLogger(ctx).Error("Failed to claim data source for indexing",
				"error", err,
				"dataSource", dataSourceDB.Name)
			dataSources[i].HasError = true
			errMsg := err.Error()
			// Update result tracking
			if result, exists := dataSourceResults[dataSourceDB.ID]; exists {
				result.IndexingStatus = "failed"
				result.HasError = true
				result.ErrorMessage = errMsg
			}
			w.publishIndexingStatus(ctx, indexingState, dataSources, &errMsg)
			continue
		}

		if !claimIndexResponse.Claimed {
			workflow.GetLogger(ctx).Info("Data source already claimed for indexing by another workflow",
				"dataSource", dataSourceDB.Name)
			// Update result tracking
			if result, exists := dataSourceResults[dataSourceDB.ID]; exists {
				result.IndexingStatus = "skipped"
				result.IndexingSkipped = true
			}
			// Skip this data source - it's being indexed by another workflow
			continue
		}

		// Ensure we release the claim in case of any error
		defer func(dsID string, idx int) {
			releaseErr := workflow.ExecuteActivity(ctx, w.ReleaseDataSourceActivity, ReleaseDataSourceInput{
				DataSourceID: dsID,
				WorkflowID:   workflowID,
			}).Get(ctx, nil)
			if releaseErr != nil {
				workflow.GetLogger(ctx).Error("Failed to release data source claim after indexing",
					"error", releaseErr,
					"dataSource", dsID)
			}
		}(dataSourceDB.ID, i)

		// TODO: systematically decide batching strategy
		batchSize := 20
		if strings.ToLower(dataSourceDB.Name) == constants.ProcessorWhatsapp.String() || strings.ToLower(dataSourceDB.Name) == constants.ProcessorTelegram.String() {
			batchSize = 3
		}
		fmt.Println("Indexing batch size", dataSourceDB.Name, batchSize)

		getBatchesInput := GetBatchesActivityInput{
			DataSourceID:   dataSourceDB.ID,
			DataSourceName: dataSourceDB.Name,
			ProcessedPath:  *dataSourceDB.ProcessedPath,
			BatchSize:      batchSize,
		}

		var getBatchesResponse GetBatchesActivityResponse
		err = workflow.ExecuteActivity(ctx, w.GetBatchesActivity, getBatchesInput).
			Get(ctx, &getBatchesResponse)
		if err != nil {
			workflow.GetLogger(ctx).Error("Failed to get batches", "error", err, "dataSource", dataSourceDB.Name)
			dataSources[i].HasError = true
			errMsg := err.Error()
			w.publishIndexingStatus(ctx, indexingState, dataSources, &errMsg)
			continue
		}

		if getBatchesResponse.TotalBatches == 0 {
			workflow.GetLogger(ctx).Warn("No batches found for data source", "dataSource", dataSourceDB.Name)
			dataSources[i].IsIndexed = true
			dataSources[i].IndexProgress = 100
			if result, exists := dataSourceResults[dataSourceDB.ID]; exists {
				result.IndexingStatus = "completed"
				result.IsIndexed = true
				result.DocumentsStored = 0
			}
			continue
		}

		failedBatches := 0
		successfulBatches := 0
		totalDocumentsStored := 0

		indexBatchCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
			StartToCloseTimeout:    30 * time.Minute,
			ScheduleToStartTimeout: 10 * time.Minute,
			RetryPolicy: &temporal.RetryPolicy{
				MaximumAttempts: 1,
			},
		})

		batchFutures := make([]workflow.Future, getBatchesResponse.TotalBatches)
		for batchIndex := 0; batchIndex < getBatchesResponse.TotalBatches; batchIndex++ {
			indexBatchInput := IndexBatchActivityInput{
				DataSourceID:   dataSourceDB.ID,
				DataSourceName: dataSourceDB.Name,
				ProcessedPath:  *dataSourceDB.ProcessedPath,
				BatchIndex:     batchIndex,
				BatchSize:      batchSize,
				TotalBatches:   getBatchesResponse.TotalBatches,
			}

			// Start activity without blocking (no .Get()) using the specific context
			batchFutures[batchIndex] = workflow.ExecuteActivity(indexBatchCtx, w.IndexBatchActivity, indexBatchInput)

			workflow.GetLogger(ctx).Info("Started batch processing",
				"dataSource", dataSourceDB.Name,
				"batch", batchIndex+1,
				"total", getBatchesResponse.TotalBatches)
		}

		// Wait for all batch activities to complete and collect results
		for batchIndex, future := range batchFutures {
			var indexBatchResponse IndexBatchActivityResponse
			err := future.Get(ctx, &indexBatchResponse)

			if err != nil {
				failedBatches++
				workflow.GetLogger(ctx).Error("Failed to index batch",
					"error", err,
					"dataSource", dataSourceDB.Name,
					"batch", batchIndex,
					"failedBatches", failedBatches,
					"totalBatches", getBatchesResponse.TotalBatches)

				failureRate := float64(failedBatches) / float64(batchIndex+1)
				if failureRate > 0.5 && failedBatches > 3 {
					workflow.GetLogger(ctx).Error("High failure rate detected, marking data source as failed",
						"dataSource", dataSourceDB.Name,
						"failureRate", failureRate,
						"failedBatches", failedBatches)
					dataSources[i].HasError = true
					errMsg := fmt.Sprintf("High batch failure rate: %d/%d batches failed", failedBatches, batchIndex+1)
					w.publishIndexingStatus(ctx, indexingState, dataSources, &errMsg)
					break
				}
			} else {
				successfulBatches++
				totalDocumentsStored += indexBatchResponse.DocumentsStored
				workflow.GetLogger(ctx).Info("Batch indexed successfully",
					"dataSource", dataSourceDB.Name,
					"batch", batchIndex+1,
					"total", getBatchesResponse.TotalBatches,
					"documentsStored", indexBatchResponse.DocumentsStored,
					"successfulBatches", successfulBatches,
					"failedBatches", failedBatches)
			}

			// Update progress as batches complete
			batchProgress := float64(batchIndex+1) / float64(getBatchesResponse.TotalBatches) * 100
			dataSources[i].IndexProgress = int32(batchProgress)
			w.publishIndexingStatus(ctx, indexingState, dataSources, nil)
		}

		finalFailureRate := float64(failedBatches) / float64(getBatchesResponse.TotalBatches)

		if failedBatches > 0 {
			workflow.GetLogger(ctx).Warn("Batch processing completed with some failures",
				"dataSource", dataSourceDB.Name,
				"successfulBatches", successfulBatches,
				"failedBatches", failedBatches,
				"totalBatches", getBatchesResponse.TotalBatches,
				"finalFailureRate", finalFailureRate)
		}

		if !dataSources[i].HasError {
			if finalFailureRate > 0.8 {
				dataSources[i].HasError = true
				errMsg := fmt.Sprintf("Too many batch failures: %d/%d batches failed", failedBatches, getBatchesResponse.TotalBatches)
				// Update result tracking
				if result, exists := dataSourceResults[dataSourceDB.ID]; exists {
					result.IndexingStatus = "failed"
					result.HasError = true
					result.ErrorMessage = errMsg
					result.DocumentsStored = totalDocumentsStored
				}
				w.publishIndexingStatus(ctx, indexingState, dataSources, &errMsg)
			} else {
				dataSources[i].IsIndexed = true
				dataSources[i].IndexProgress = 100

				// Update result tracking
				if result, exists := dataSourceResults[dataSourceDB.ID]; exists {
					result.IndexingStatus = "completed"
					result.IsIndexed = true
					result.HasError = false
					result.DocumentsStored = totalDocumentsStored
				}

				updateStateInput := UpdateDataSourceStateActivityInput{
					DataSourceID: dataSourceDB.ID,
					IsIndexed:    true,
					HasError:     false,
				}
				err = workflow.ExecuteActivity(ctx, w.UpdateDataSourceStateActivity, updateStateInput).Get(ctx, nil)
				if err != nil {
					workflow.GetLogger(ctx).Error("Failed to update data source state", "error", err)
				}
			}
		}
	}

	w.publishIndexingStatus(
		ctx,
		model.IndexingStateCompleted,
		dataSources,
		nil,
	)

	// Categorize results
	var processedSources []DataSourceResult
	var failedSources []DataSourceResult
	var skippedSources []DataSourceResult
	successfulCount := 0
	failedCount := 0
	skippedCount := 0

	for _, result := range dataSourceResults {
		if result.ProcessingSkipped || result.IndexingSkipped {
			skippedSources = append(skippedSources, *result)
			skippedCount++
		} else if result.HasError {
			failedSources = append(failedSources, *result)
			failedCount++
		} else {
			processedSources = append(processedSources, *result)
			successfulCount++
		}
	}

	message := fmt.Sprintf("Initialization completed. Processed: %d, Failed: %d, Skipped: %d",
		successfulCount, failedCount, skippedCount)

	return InitializeWorkflowResponse{
		Message:           message,
		ProcessedSources:  processedSources,
		FailedSources:     failedSources,
		SkippedSources:    skippedSources,
		TotalSources:      len(dataSourceResults),
		SuccessfulSources: successfulCount,
		FailedCount:       failedCount,
		SkippedCount:      skippedCount,
	}, nil
}

func (w *DataProcessingWorkflows) publishIndexingStatus(
	ctx workflow.Context,
	state model.IndexingState,
	dataSources []*model.DataSource,
	error *string,
) {
	status := &model.IndexingStatus{
		Status:      state,
		DataSources: dataSources,
		Error:       error,
	}
	statusJson, _ := json.Marshal(status)
	subject := "indexing_data"

	input := PublishIndexingStatusInput{
		Subject: subject,
		Data:    statusJson,
	}
	err := workflow.ExecuteActivity(ctx, w.PublishIndexingStatus, input).Get(ctx, nil)
	if err != nil {
		workflow.GetLogger(ctx).
			Error("Failed to publish indexing status", "error", err, "subject", subject)
	}
}

type FetchDataSourcesActivityInput struct{}

type FetchDataSourcesActivityResponse struct {
	DataSources []*db.DataSource `json:"dataSources"`
}

func (w *DataProcessingWorkflows) FetchDataSourcesActivity(
	ctx context.Context,
	input FetchDataSourcesActivityInput,
) (FetchDataSourcesActivityResponse, error) {
	w.Logger.Debug("FetchDataSourcesActivity started")

	dataSources, err := w.Store.GetUnindexedDataSources(ctx)
	if err != nil {
		w.Logger.Error("Failed to get unindexed data sources", "error", err)
		return FetchDataSourcesActivityResponse{}, err
	}

	w.Logger.Info("FetchDataSourcesActivity completed", "count", len(dataSources))

	for i, ds := range dataSources {
		hasError := "NULL"
		if ds.HasError != nil {
			hasError = fmt.Sprintf("%v", *ds.HasError)
		}
		isIndexed := "NULL"
		if ds.IsIndexed != nil {
			isIndexed = fmt.Sprintf("%v", *ds.IsIndexed)
		}
		w.Logger.Info("Found data source",
			"index", i,
			"id", ds.ID,
			"name", ds.Name,
			"path", ds.Path,
			"state", ds.State,
			"hasError", hasError,
			"isIndexed", isIndexed,
		)
	}

	return FetchDataSourcesActivityResponse{DataSources: dataSources}, nil
}

type ProcessDataActivityInput struct {
	DataSourceName string `json:"dataSourceName"`
	SourcePath     string `json:"sourcePath"`
	Username       string `json:"username"`
	DataSourceID   string `json:"dataSourceID"`
}

type ProcessDataActivityResponse struct {
	Success bool `json:"success"`
}

func (w *DataProcessingWorkflows) ProcessDataActivity(
	ctx context.Context,
	input ProcessDataActivityInput,
) (ProcessDataActivityResponse, error) {
	dataprocessingService := dataprocessing.NewDataProcessingService(w.OpenAIService, w.Config.CompletionsModel, w.Store, w.Memory, w.Logger)

	isNewFormat := w.isNewFormatProcessor(input.DataSourceName)

	if isNewFormat {
		// New format processors (telegram, whatsapp, etc.) store documents directly in memory
		// No file is created, so we use a special marker path
		success, err := dataprocessingService.ProcessSource(
			ctx,
			input.DataSourceName,
			input.SourcePath,
			"", // Empty output path - no file will be created
		)
		if err != nil {
			w.Logger.Error(
				"Failed to process data source",
				"error",
				err,
				"dataSource",
				input.DataSourceName,
			)
			return ProcessDataActivityResponse{}, err
		}

		// Use a special marker to indicate direct processing (no file)
		processedPath := fmt.Sprintf("direct://%s_%s", input.DataSourceName, input.DataSourceID)
		err = w.Store.UpdateDataSourceProcessedPath(ctx, input.DataSourceID, processedPath)
		if err != nil {
			return ProcessDataActivityResponse{}, err
		}

		return ProcessDataActivityResponse{Success: success}, nil
	} else {
		// Old format processors create JSONL files
		outputPath := fmt.Sprintf(
			"%s/%s_%s.jsonl",
			w.Config.AppDataPath,
			input.DataSourceName,
			input.DataSourceID,
		)

		success, err := dataprocessingService.ProcessSource(
			ctx,
			input.DataSourceName,
			input.SourcePath,
			outputPath,
		)
		if err != nil {
			w.Logger.Error(
				"Failed to process data source",
				"error",
				err,
				"dataSource",
				input.DataSourceName,
			)
			return ProcessDataActivityResponse{}, err
		}

		err = w.Store.UpdateDataSourceProcessedPath(ctx, input.DataSourceID, outputPath)
		if err != nil {
			return ProcessDataActivityResponse{}, err
		}

		return ProcessDataActivityResponse{Success: success}, nil
	}
}

type UpdateDataSourceStateActivityInput struct {
	DataSourceID string `json:"dataSourceId"`
	IsIndexed    bool   `json:"isIndexed"`
	HasError     bool   `json:"hasError"`
}

type UpdateDataSourceStateActivityResponse struct {
	Success bool `json:"success"`
}

func (w *DataProcessingWorkflows) UpdateDataSourceStateActivity(
	ctx context.Context,
	input UpdateDataSourceStateActivityInput,
) (UpdateDataSourceStateActivityResponse, error) {
	_, err := w.Store.UpdateDataSourceState(
		ctx,
		input.DataSourceID,
		input.IsIndexed,
		input.HasError,
	)
	if err != nil {
		return UpdateDataSourceStateActivityResponse{}, err
	}
	return UpdateDataSourceStateActivityResponse{Success: true}, nil
}

type GetBatchesActivityInput struct {
	DataSourceID   string `json:"dataSourceId"`
	DataSourceName string `json:"dataSourceName"`
	ProcessedPath  string `json:"processedPath"`
	BatchSize      int    `json:"batchSize"`
}

type GetBatchesActivityResponse struct {
	TotalBatches int `json:"totalBatches"`
}

func (w *DataProcessingWorkflows) GetBatchesActivity(
	ctx context.Context,
	input GetBatchesActivityInput,
) (GetBatchesActivityResponse, error) {
	if input.BatchSize <= 0 {
		return GetBatchesActivityResponse{}, errors.New("batch size must be positive")
	}
	if input.ProcessedPath == "" {
		return GetBatchesActivityResponse{}, errors.New("processed path cannot be empty")
	}

	// Check if this is a direct processing marker (documents already stored in memory)
	if strings.HasPrefix(input.ProcessedPath, "direct://") {
		// Documents were stored directly in memory during processing, no indexing needed
		return GetBatchesActivityResponse{TotalBatches: 0}, nil
	}

	isNewFormat := w.isNewFormatProcessor(input.DataSourceName)

	var itemCount int
	var err error

	if isNewFormat {
		// New format: JSON array - count array items
		itemCount, err = w.countNewFormatItems(input.ProcessedPath)
	} else {
		// Old format: JSONL - count lines
		itemCount, err = helpers.CountJSONLLines(input.ProcessedPath)
	}

	if err != nil {
		return GetBatchesActivityResponse{}, err
	}

	totalBatches := (itemCount + input.BatchSize - 1) / input.BatchSize
	return GetBatchesActivityResponse{TotalBatches: totalBatches}, nil
}

type IndexBatchActivityInput struct {
	DataSourceID   string `json:"dataSourceId"`
	DataSourceName string `json:"dataSourceName"`
	ProcessedPath  string `json:"processedPath"`
	BatchIndex     int    `json:"batchIndex"`
	BatchSize      int    `json:"batchSize"`
	TotalBatches   int    `json:"totalBatches"`
}

type IndexBatchActivityResponse struct {
	BatchIndex      int  `json:"batchIndex"`
	DocumentsStored int  `json:"documentsStored"`
	Success         bool `json:"success"`
}

func (w *DataProcessingWorkflows) IndexBatchActivity(
	ctx context.Context,
	input IndexBatchActivityInput,
) (IndexBatchActivityResponse, error) {
	if input.BatchIndex < 0 {
		return IndexBatchActivityResponse{}, errors.New("batch index cannot be negative")
	}
	if input.BatchSize <= 0 {
		return IndexBatchActivityResponse{}, errors.New("batch size must be positive")
	}
	if input.ProcessedPath == "" {
		return IndexBatchActivityResponse{}, errors.New("processed path cannot be empty")
	}

	// Check if this is a new format processor (JSON array) or old format (JSONL)
	isNewFormat := w.isNewFormatProcessor(input.DataSourceName)

	var documents []memory.Document
	var err error

	if isNewFormat {
		// New format: JSONL of ConversationDocument - read entire file and batch in memory
		documents, err = w.readNewFormatBatch(input.ProcessedPath, input.BatchIndex, input.BatchSize)
	} else {
		// Old format: JSONL of types.Record - read batch from file
		startIdx := input.BatchIndex * input.BatchSize
		records, err := helpers.ReadJSONLBatch(input.ProcessedPath, startIdx, input.BatchSize)
		if err != nil {
			return IndexBatchActivityResponse{}, err
		}

		if len(records) == 0 {
			return IndexBatchActivityResponse{
				BatchIndex:      input.BatchIndex,
				DocumentsStored: 0,
				Success:         true,
			}, nil
		}

		dataprocessingService := dataprocessing.NewDataProcessingService(
			w.OpenAIService,
			w.Config.CompletionsModel,
			w.Store,
			w.Memory,
			w.Logger,
		)

		documents, _ = dataprocessingService.ToDocuments(ctx, input.DataSourceName, records)
	}

	if err != nil {
		return IndexBatchActivityResponse{}, err
	}

	w.Logger.Info("Read documents", "documents", len(documents))

	if len(documents) == 0 {
		return IndexBatchActivityResponse{
			BatchIndex:      input.BatchIndex,
			DocumentsStored: 0,
			Success:         true,
		}, nil
	}

	progressCallback := func(processed, total int) {
		w.Logger.Info("Batch progress",
			"dataSource", input.DataSourceName,
			"batch", input.BatchIndex+1,
			"totalBatches", input.TotalBatches,
			"processed", processed,
			"total", total)
	}

	err = w.Memory.Store(ctx, documents, progressCallback)
	if err != nil {
		return IndexBatchActivityResponse{}, err
	}

	return IndexBatchActivityResponse{
		BatchIndex:      input.BatchIndex,
		DocumentsStored: len(documents),
		Success:         true,
	}, nil
}

// isNewFormatProcessor checks if the data source uses the new ConversationDocument format.
func (w *DataProcessingWorkflows) isNewFormatProcessor(dataSourceName string) bool {
	return constants.IsNewFormatProcessor(dataSourceName)
}

// readNewFormatBatch reads a batch from either JSON array format or JSONL format (ConversationDocument[]).
func (w *DataProcessingWorkflows) readNewFormatBatch(filePath string, batchIndex, batchSize int) ([]memory.Document, error) {
	// Read the entire file first
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Detect format: JSON array or JSONL
	isJSONL := w.isJSONLFormat(data)

	var conversationDocs []memory.ConversationDocument

	if isJSONL {
		// JSONL format: each line is a separate JSON object
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			var doc memory.ConversationDocument
			if err := json.Unmarshal([]byte(line), &doc); err != nil {
				return nil, fmt.Errorf("failed to unmarshal JSONL line: %w", err)
			}
			conversationDocs = append(conversationDocs, doc)
		}
	} else {
		// JSON array format
		if err := json.Unmarshal(data, &conversationDocs); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ConversationDocument array: %w", err)
		}
	}

	// Calculate batch boundaries
	startIdx := batchIndex * batchSize
	endIdx := startIdx + batchSize

	if startIdx >= len(conversationDocs) {
		return []memory.Document{}, nil
	}

	if endIdx > len(conversationDocs) {
		endIdx = len(conversationDocs)
	}

	// Convert to Document interface
	documents := make([]memory.Document, endIdx-startIdx)
	for i := startIdx; i < endIdx; i++ {
		// Create a copy to avoid pointer issues
		doc := conversationDocs[i]
		documents[i-startIdx] = &doc
	}

	return documents, nil
}

// isJSONLFormat detects if the data is in JSONL format (each line is a JSON object)
// versus JSON array format (the entire file is a JSON array).
func (w *DataProcessingWorkflows) isJSONLFormat(data []byte) bool {
	// Trim whitespace and check first character
	trimmed := strings.TrimSpace(string(data))
	if len(trimmed) == 0 {
		return false
	}

	// If it starts with '[', it's likely a JSON array
	if trimmed[0] == '[' {
		return false
	}

	// If it starts with '{', it's likely JSONL
	if trimmed[0] == '{' {
		return true
	}

	return false
}

// countNewFormatItems counts items in either JSON array format or JSONL format.
func (w *DataProcessingWorkflows) countNewFormatItems(filePath string) (int, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to read file: %w", err)
	}

	// Detect format: JSON array or JSONL
	isJSONL := w.isJSONLFormat(data)

	if isJSONL {
		// JSONL format: count non-empty lines
		lines := strings.Split(string(data), "\n")
		count := 0
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				count++
			}
		}
		return count, nil
	} else {
		// JSON array format
		var conversationDocs []memory.ConversationDocument
		if err := json.Unmarshal(data, &conversationDocs); err != nil {
			return 0, fmt.Errorf("failed to unmarshal ConversationDocument array: %w", err)
		}
		return len(conversationDocs), nil
	}
}

type PublishIndexingStatusInput struct {
	Subject string
	Data    []byte
}

func (w *DataProcessingWorkflows) PublishIndexingStatus(
	ctx context.Context,
	input PublishIndexingStatusInput,
) error {
	if w.Nc == nil {
		return errors.New("NATS connection is nil")
	}

	if !w.Nc.IsConnected() {
		return errors.New("NATS connection is not connected")
	}

	err := w.Nc.Publish(input.Subject, input.Data)
	if err != nil {
		w.Logger.Error("Failed to publish indexing status",
			"error", err,
			"subject", input.Subject,
			"connected", w.Nc.IsConnected(),
			"status", w.Nc.Status().String())
		return err
	}

	return nil
}

type CleanupStaleProcessingInput struct {
	StaleAfterMinutes int `json:"staleAfterMinutes"`
}

type CleanupStaleProcessingResponse struct {
	CleanedCount int `json:"cleanedCount"`
}

func (w *DataProcessingWorkflows) CleanupStaleProcessingActivity(
	ctx context.Context,
	input CleanupStaleProcessingInput,
) (CleanupStaleProcessingResponse, error) {
	count, err := w.Store.CleanupStaleProcessingDataSources(ctx, input.StaleAfterMinutes)
	if err != nil {
		return CleanupStaleProcessingResponse{}, err
	}
	return CleanupStaleProcessingResponse{CleanedCount: count}, nil
}

type ClaimDataSourceInput struct {
	DataSourceID string `json:"dataSourceId"`
	WorkflowID   string `json:"workflowId"`
}

type ClaimDataSourceResponse struct {
	Claimed bool `json:"claimed"`
}

func (w *DataProcessingWorkflows) ClaimDataSourceForProcessingActivity(
	ctx context.Context,
	input ClaimDataSourceInput,
) (ClaimDataSourceResponse, error) {
	claimed, err := w.Store.ClaimDataSourceForProcessing(ctx, input.DataSourceID, input.WorkflowID)
	if err != nil {
		return ClaimDataSourceResponse{}, err
	}
	return ClaimDataSourceResponse{Claimed: claimed}, nil
}

func (w *DataProcessingWorkflows) ClaimDataSourceForIndexingActivity(
	ctx context.Context,
	input ClaimDataSourceInput,
) (ClaimDataSourceResponse, error) {
	claimed, err := w.Store.ClaimDataSourceForIndexing(ctx, input.DataSourceID, input.WorkflowID)
	if err != nil {
		return ClaimDataSourceResponse{}, err
	}
	return ClaimDataSourceResponse{Claimed: claimed}, nil
}

type ReleaseDataSourceInput struct {
	DataSourceID string `json:"dataSourceId"`
	WorkflowID   string `json:"workflowId"`
}

func (w *DataProcessingWorkflows) ReleaseDataSourceActivity(
	ctx context.Context,
	input ReleaseDataSourceInput,
) error {
	return w.Store.ReleaseDataSourceFromProcessing(ctx, input.DataSourceID, input.WorkflowID)
}

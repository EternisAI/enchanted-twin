package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/EternisAI/enchanted-twin/graph/model"
	dataprocessing "github.com/EternisAI/enchanted-twin/pkg/dataprocessing"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/helpers"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
	"github.com/EternisAI/enchanted-twin/pkg/db"
)

type InitializeWorkflowInput struct{}

type InitializeWorkflowResponse struct{}

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
		StartToCloseTimeout: 120 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second * 2,
			MaximumInterval:    time.Minute * 10,
			BackoffCoefficient: 4,
			MaximumAttempts:    1,
		},
	})

	indexingState := model.IndexingStateNotStarted
	dataSources := []*model.DataSource{}
	w.publishIndexingStatus(ctx, indexingState, dataSources, 0, 0, nil)

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

	var fetchDataSourcesResponse FetchDataSourcesActivityResponse
	err = workflow.ExecuteActivity(ctx, w.FetchDataSourcesActivity, FetchDataSourcesActivityInput{}).
		Get(ctx, &fetchDataSourcesResponse)
	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to fetch data sources", "error", err)
		indexingState = model.IndexingStateFailed
		errMsg := err.Error()
		w.publishIndexingStatus(ctx, indexingState, dataSources, 0, 0, &errMsg)
		return InitializeWorkflowResponse{}, errors.Wrap(err, "failed to fetch data sources")
	}

	if len(fetchDataSourcesResponse.DataSources) == 0 {
		workflow.GetLogger(ctx).Info("No data sources found")
		indexingState = model.IndexingStateFailed
		errMsg := "No data sources found"
		w.publishIndexingStatus(ctx, indexingState, dataSources, 0, 0, &errMsg)
		return InitializeWorkflowResponse{}, errors.New(errMsg)
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
	}

	indexingState = model.IndexingStateProcessingData
	w.publishIndexingStatus(ctx, indexingState, dataSources, 0, 0, nil)

	for i, dataSource := range fetchDataSourcesResponse.DataSources {
		w.publishIndexingStatus(ctx, indexingState, dataSources, 0, 0, nil)

		processDataActivityInput := ProcessDataActivityInput{
			DataSourceName: dataSource.Name,
			SourcePath:     dataSource.Path,
			DataSourceID:   dataSource.ID,
			Username:       "xxx",
		}
		var processDataResponse ProcessDataActivityResponse
		err = workflow.ExecuteActivity(ctx, w.ProcessDataActivity, processDataActivityInput).
			Get(ctx, &processDataResponse)

		progress := int32((i + 1) * 100 / len(fetchDataSourcesResponse.DataSources))
		dataSources[i].UpdatedAt = time.Now().Format(time.RFC3339)
		if err != nil {
			workflow.GetLogger(ctx).Error("Failed to process data source",
				"error", err,
				"dataSource", dataSource.Name)

			dataSources[i].IsProcessed = false
			dataSources[i].HasError = true
			errMsg := err.Error()
			w.publishIndexingStatus(ctx, indexingState, dataSources, progress, 0, &errMsg)
		} else {
			dataSources[i].IsProcessed = true
			dataSources[i].HasError = false
			dataSources[i].IsIndexed = false
			w.publishIndexingStatus(ctx, indexingState, dataSources, progress, 0, nil)
		}

		w.publishIndexingStatus(ctx, indexingState, dataSources, progress, 0, nil)
	}

	indexingState = model.IndexingStateIndexingData
	w.publishIndexingStatus(ctx, indexingState, dataSources, 100, 0, nil)

	var indexDataResponse IndexDataActivityResponse

	err = workflow.ExecuteActivity(ctx, w.IndexDataActivity, IndexDataActivityInput{DataSourcesInput: dataSources, IndexingState: indexingState}).
		Get(ctx, &indexDataResponse)
	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to index data", "error", err)
		errMsg := err.Error()
		indexingState = model.IndexingStateFailed
		w.publishIndexingStatus(ctx, indexingState, dataSources, 100, 0, &errMsg)
		return InitializeWorkflowResponse{}, errors.Wrap(err, "failed to index data")
	}

	w.publishIndexingStatus(
		ctx,
		model.IndexingStateCompleted,
		indexDataResponse.DataSourcesResponse,
		100,
		100,
		nil,
	)

	return InitializeWorkflowResponse{}, nil
}

func (w *DataProcessingWorkflows) publishIndexingStatus(
	ctx workflow.Context,
	state model.IndexingState,
	dataSources []*model.DataSource,
	processingProgress, indexingProgress int32,
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
	dataSources, err := w.Store.GetUnindexedDataSources(ctx)
	if err != nil {
		return FetchDataSourcesActivityResponse{}, err
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
	outputPath := fmt.Sprintf(
		"%s/%s_%s.jsonl",
		w.Config.AppDataPath,
		input.DataSourceName,
		input.DataSourceID,
	)
	dataprocessingService := dataprocessing.NewDataProcessingService(w.OpenAIService, w.Config.CompletionsModel, w.Store)
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

type IndexDataActivityInput struct {
	DataSourcesInput []*model.DataSource `json:"dataSources"`
	IndexingState    model.IndexingState `json:"indexingState"`
}

type IndexDataActivityResponse struct {
	DataSourcesResponse []*model.DataSource `json:"dataSources"`
}

func publishIndexingStatus(
	w *DataProcessingWorkflows,
	dataSources []*model.DataSource,
	state model.IndexingState,
	error *string,
) {
	status := &model.IndexingStatus{
		Status:      state,
		DataSources: dataSources,
		Error:       error,
	}
	statusJson, _ := json.Marshal(status)
	subject := "indexing_data"

	if w.Nc == nil {
		w.Logger.Error("NATS connection is nil")
		return
	}

	if !w.Nc.IsConnected() {
		w.Logger.Error("NATS connection is not connected")
		return
	}

	w.Logger.Info("Publishing indexing status",
		"subject", subject,
		"data", string(statusJson),
		"connected", w.Nc.IsConnected(),
		"status", w.Nc.Status().String())

	err := w.Nc.Publish(subject, statusJson)
	if err != nil {
		w.Logger.Error("Failed to publish indexing status",
			"error", err,
			"subject", subject,
			"connected", w.Nc.IsConnected(),
			"status", w.Nc.Status().String())
	}
}

func (w *DataProcessingWorkflows) IndexDataActivity(
	ctx context.Context,
	input IndexDataActivityInput,
) (IndexDataActivityResponse, error) {
	dataSourcesDB, err := w.Store.GetUnindexedDataSources(ctx)
	if err != nil {
		return IndexDataActivityResponse{}, fmt.Errorf(
			"failed to get unindexed data sources: %w",
			err,
		)
	}

	dataSourcesResponse := make([]*model.DataSource, len(input.DataSourcesInput))

	copy(dataSourcesResponse, input.DataSourcesInput)

	var wg sync.WaitGroup
	resultChan := make(chan struct{})
	for i, dataSourceDB := range dataSourcesDB {
		if dataSourceDB.ProcessedPath == nil {
			w.Logger.Error("Processed path is nil", "dataSource", dataSourceDB.Name)
			continue
		}

		records, err := helpers.ReadJSONL[types.Record](*dataSourceDB.ProcessedPath)
		if err != nil {
			return IndexDataActivityResponse{}, err
		}

		if len(records) == 0 {
			w.Logger.Warn("No records found for data source", "dataSource", dataSourceDB.Name)
			dataSourcesResponse[i].IsIndexed = true
			continue
		}

		dataprocessingService := dataprocessing.NewDataProcessingService(w.OpenAIService, w.Config.CompletionsModel, w.Store)

		batchSize := 20
		totalBatches := (len(records) + batchSize - 1) / batchSize

		for j := 0; j < len(records); j += batchSize {
			end := j + batchSize
			if end > len(records) {
				end = len(records)
			}

			batchRecords := records[j:end]
			documents, err := dataprocessingService.ToDocuments(ctx, dataSourceDB.Name, batchRecords)
			if err != nil {
				return IndexDataActivityResponse{}, err
			}

			batchNum := j/batchSize + 1
			batchProgressCallback := func(processed, total int) {
				batchProgress := float64(batchNum-1) / float64(totalBatches) * 100
				if total > 0 {
					withinBatchProgress := float64(processed) / float64(total) * (100.0 / float64(totalBatches))
					batchProgress += withinBatchProgress
				}
				dataSourcesResponse[i].IndexProgress = int32(batchProgress)
				publishIndexingStatus(w, dataSourcesResponse, input.IndexingState, nil)
			}

			err = w.Memory.Store(ctx, documents, batchProgressCallback)
			if err != nil {
				return IndexDataActivityResponse{}, err
			}
		}

		dataSourcesResponse[i].IsIndexed = true
	}
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	for _, dataSource := range dataSourcesResponse {
		_, err = w.Store.UpdateDataSourceState(
			ctx,
			dataSource.ID,
			dataSource.IsIndexed,
			dataSource.HasError,
		)
		if err != nil {
			return IndexDataActivityResponse{}, err
		}
	}

	return IndexDataActivityResponse{DataSourcesResponse: dataSourcesResponse}, nil
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

	w.Logger.Info("Publishing indexing status",
		"subject", input.Subject,
		"data", string(input.Data),
		"connected", w.Nc.IsConnected(),
		"status", w.Nc.Status().String())

	err := w.Nc.Publish(input.Subject, input.Data)
	if err != nil {
		w.Logger.Error("Failed to publish indexing status",
			"error", err,
			"subject", input.Subject,
			"connected", w.Nc.IsConnected(),
			"status", w.Nc.Status().String())
		return err
	}

	w.Logger.Info("Successfully published indexing status", "subject", input.Subject)
	return nil
}

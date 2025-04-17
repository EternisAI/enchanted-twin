package indexing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/EternisAI/enchanted-twin/graph/model"
	dataprocessing "github.com/EternisAI/enchanted-twin/pkg/dataprocessing"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type IndexWorkflowInput struct{}

type IndexWorkflowResponse struct{}

type IndexingState string

const (
	NOT_STARTED       IndexingState = "NOT_STARTED"
	DOWNLOADING_MODEL IndexingState = "DOWNLOADING_MODEL"
	PROCESSING_DATA   IndexingState = "PROCESSING_DATA"
	INDEXING_DATA     IndexingState = "INDEXING_DATA"
	COMPLETED         IndexingState = "COMPLETED"
	FAILED            IndexingState = "FAILED"
)

type IndexingStateQuery struct {
	State IndexingState
}

const OUTPUT_PATH = "./output/"

func (w *IndexingWorkflow) IndexWorkflow(ctx workflow.Context, input IndexWorkflowInput) (IndexWorkflowResponse, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 3 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second * 2,
			MaximumInterval:    time.Minute * 10,
			BackoffCoefficient: 4,
			MaximumAttempts:    2,
		},
	})

	indexingState := NOT_STARTED
	dataSources := []*model.DataSource{}
	w.publishIndexingStatus(ctx, model.IndexingStateNotStarted, dataSources, 0, 0)

	err := workflow.SetQueryHandler(ctx, "getIndexingState", func() (IndexingStateQuery, error) {
		return IndexingStateQuery{State: indexingState}, nil
	})
	if err != nil {
		return IndexWorkflowResponse{}, err
	}

	var fetchDataSourcesResponse FetchDataSourcesActivityResponse
	err = workflow.ExecuteActivity(ctx, w.FetchDataSourcesActivity, FetchDataSourcesActivityInput{}).Get(ctx, &fetchDataSourcesResponse)
	if err != nil {
		indexingState = FAILED
		return IndexWorkflowResponse{}, err
	}

	if len(fetchDataSourcesResponse.DataSources) == 0 {
		indexingState = FAILED
		return IndexWorkflowResponse{}, errors.New("no data sources found")
	}

	for _, dataSource := range fetchDataSourcesResponse.DataSources {
		dataSources = append(dataSources, &model.DataSource{
			ID:          dataSource.ID,
			Name:        dataSource.Name,
			Path:        dataSource.Path,
			IsProcessed: false,
			IsIndexed:   false,
		})
	}

	indexingState = PROCESSING_DATA
	w.publishIndexingStatus(ctx, model.IndexingStateProcessingData, dataSources, 0, 0)

	for i, dataSource := range fetchDataSourcesResponse.DataSources {

		w.publishIndexingStatus(ctx, model.IndexingStateProcessingData, dataSources, 0, 0)

		processDataActivityInput := ProcessDataActivityInput{
			DataSourceName: dataSource.Name,
			SourcePath:     dataSource.Path,
			Username:       "",
		}
		var processDataResponse ProcessDataActivityResponse
		err = workflow.ExecuteActivity(ctx, w.ProcessDataActivity, processDataActivityInput).Get(ctx, &processDataResponse)
		if err != nil {
			indexingState = FAILED
			w.publishIndexingStatus(ctx, model.IndexingStateProcessingData, dataSources, 0, 0)
			return IndexWorkflowResponse{}, err
		}

		dataSources[i].IsProcessed = true
		dataSources[i].UpdatedAt = time.Now().Format(time.RFC3339)
		progress := int32((i + 1) * 100 / len(fetchDataSourcesResponse.DataSources))
		w.publishIndexingStatus(ctx, model.IndexingStateProcessingData, dataSources, progress, 0)
	}

	indexingState = INDEXING_DATA
	w.publishIndexingStatus(ctx, model.IndexingStateIndexingData, dataSources, 100, 0)

	var indexDataResponse IndexDataActivityResponse
	err = workflow.ExecuteActivity(ctx, w.IndexDataActivity, IndexDataActivityInput{}).Get(ctx, &indexDataResponse)
	if err != nil {
		indexingState = FAILED
		w.publishIndexingStatus(ctx, model.IndexingStateIndexingData, dataSources, 100, 0)
		return IndexWorkflowResponse{}, err
	}

	var completeResponse CompleteActivityResponse
	err = workflow.ExecuteActivity(ctx, w.CompleteActivity, CompleteActivityInput{
		DataSources: dataSources,
	}).Get(ctx, &completeResponse)
	if err != nil {
		indexingState = FAILED
		return IndexWorkflowResponse{}, err
	}

	for i := range dataSources {
		dataSources[i].IsIndexed = true
		dataSources[i].UpdatedAt = time.Now().Format(time.RFC3339)
	}

	indexingState = COMPLETED
	fmt.Println("completed")
	fmt.Println(dataSources[0].IsIndexed)

	w.publishIndexingStatus(ctx, model.IndexingStateCompleted, dataSources, 100, 100)
	return IndexWorkflowResponse{}, nil
}

func (w *IndexingWorkflow) publishIndexingStatus(ctx workflow.Context, state model.IndexingState, dataSources []*model.DataSource, processingProgress, indexingProgress int32) {
	status := &model.IndexingStatus{
		Status:                 state,
		ProcessingDataProgress: processingProgress,
		IndexingDataProgress:   indexingProgress,
		DataSources:            dataSources,
	}
	statusJson, _ := json.Marshal(status)
	subject := "indexing_data"

	input := PublishIndexingStatusInput{
		Subject: subject,
		Data:    statusJson,
	}
	err := workflow.ExecuteActivity(ctx, w.PublishIndexingStatus, input).Get(ctx, nil)
	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to publish indexing status", "error", err, "subject", subject)
	}
}

type FetchDataSourcesActivityInput struct{}

type FetchDataSourcesActivityResponse struct {
	DataSources []*db.DataSource `json:"dataSources"`
}

func (w *IndexingWorkflow) FetchDataSourcesActivity(ctx context.Context, input FetchDataSourcesActivityInput) (FetchDataSourcesActivityResponse, error) {
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
}

type ProcessDataActivityResponse struct {
	Success bool `json:"success"`
}

func (w *IndexingWorkflow) ProcessDataActivity(ctx context.Context, input ProcessDataActivityInput) (ProcessDataActivityResponse, error) {
	// TODO: replace username parameter
	success, err := dataprocessing.ProcessSource(input.DataSourceName, input.SourcePath, OUTPUT_PATH+input.DataSourceName+".jsonl", "xxx", "")
	if err != nil {
		fmt.Println(err)
		return ProcessDataActivityResponse{}, err
	}
	return ProcessDataActivityResponse{Success: success}, nil
}

type DownloadModelActivityInput struct {
	ModelName string `json:"modelName"`
}

type DownloadModelActivityResponse struct{}

func (w *IndexingWorkflow) DownloadModelActivity(ctx context.Context, input DownloadModelActivityInput) (DownloadModelActivityResponse, error) {
	return DownloadModelActivityResponse{}, nil
}

type IndexDataActivityInput struct{}

type IndexDataActivityResponse struct{}

func (w *IndexingWorkflow) IndexDataActivity(ctx context.Context, input IndexDataActivityInput) (IndexDataActivityResponse, error) {
	return IndexDataActivityResponse{}, nil
}

type CompleteActivityInput struct {
	DataSources []*model.DataSource `json:"dataSources"`
}

type CompleteActivityResponse struct{}

func (w *IndexingWorkflow) CompleteActivity(ctx context.Context, input CompleteActivityInput) (CompleteActivityResponse, error) {
	for _, dataSource := range input.DataSources {
		_, err := w.Store.UpdateDataSource(ctx, dataSource.ID, true)
		if err != nil {
			return CompleteActivityResponse{}, err
		}

	}
	return CompleteActivityResponse{}, nil
}

type PublishIndexingStatusInput struct {
	Subject string
	Data    []byte
}

func (w *IndexingWorkflow) PublishIndexingStatus(ctx context.Context, input PublishIndexingStatusInput) error {
	if w.Nc == nil {
		return fmt.Errorf("NATS connection is nil")
	}

	if !w.Nc.IsConnected() {
		return fmt.Errorf("NATS connection is not connected")
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

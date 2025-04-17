package workflows

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/dataimport"
	"github.com/google/uuid"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type IndexWorkflowInput struct {
	DataSourceName string `json:"dataSourceName"`
	SourcePath     string `json:"sourcePath"`
	Username       string `json:"username"`
}

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

func (w *TemporalWorkflows) IndexWorkflow(ctx workflow.Context, input IndexWorkflowInput) (IndexWorkflowResponse, error) {
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

	err := workflow.SetQueryHandler(ctx, "getIndexingState", func() (IndexingStateQuery, error) {
		return IndexingStateQuery{State: indexingState}, nil
	})
	if err != nil {
		return IndexWorkflowResponse{}, err
	}

	if input.DataSourceName == "" {
		return IndexWorkflowResponse{}, errors.New("dataSourceName is required")
	}
	if input.SourcePath == "" {
		return IndexWorkflowResponse{}, errors.New("sourcePath is required")
	}

	var createDataSourceResponse CreateDataSourceActivityResponse

	dataSourceId := uuid.New().String()
	err = workflow.ExecuteActivity(ctx, w.CreateDataSourceActivity, CreateDataSourceActivityInput{
		DataSourceID:   dataSourceId,
		DataSourceName: input.DataSourceName,
	}).Get(ctx, &createDataSourceResponse)
	if err != nil {
		indexingState = FAILED
		return IndexWorkflowResponse{}, err
	}

	indexingState = PROCESSING_DATA

	var processDataResponse ProcessDataActivityResponse
	err = workflow.ExecuteActivity(ctx, w.ProcessDataActivity, ProcessDataActivityInput(input)).Get(ctx, &processDataResponse)
	if err != nil {
		indexingState = FAILED
		return IndexWorkflowResponse{}, err
	}

	indexingState = INDEXING_DATA

	var indexDataResponse IndexDataActivityResponse
	err = workflow.ExecuteActivity(ctx, w.IndexDataActivity, IndexDataActivityInput{}).Get(ctx, &indexDataResponse)
	if err != nil {
		indexingState = FAILED
		return IndexWorkflowResponse{}, err
	}

	var completeResponse CompleteActivityResponse
	err = workflow.ExecuteActivity(ctx, w.CompleteActivity, CompleteActivityInput{
		DataSourceID: dataSourceId,
	}).Get(ctx, &completeResponse)
	if err != nil {
		indexingState = FAILED
		return IndexWorkflowResponse{}, err
	}

	indexingState = COMPLETED
	return IndexWorkflowResponse{}, nil
}

type CreateDataSourceActivityInput struct {
	DataSourceID   string `json:"dataSourceID"`
	DataSourceName string `json:"dataSourceName"`
}

type CreateDataSourceActivityResponse struct{}

func (w *TemporalWorkflows) CreateDataSourceActivity(ctx context.Context, input CreateDataSourceActivityInput) (CreateDataSourceActivityResponse, error) {
	_, err := w.Store.CreateDataSource(ctx, input.DataSourceID, input.DataSourceName, "")
	if err != nil {
		return CreateDataSourceActivityResponse{}, err
	}
	return CreateDataSourceActivityResponse{}, nil
}

type ProcessDataActivityInput struct {
	DataSourceName string `json:"dataSourceName"`
	SourcePath     string `json:"sourcePath"`
	Username       string `json:"username"`
}

type ProcessDataActivityResponse struct {
	Success bool `json:"success"`
}

func (w *TemporalWorkflows) ProcessDataActivity(ctx context.Context, input ProcessDataActivityInput) (ProcessDataActivityResponse, error) {
	success, err := dataimport.ProcessSource(input.DataSourceName, input.SourcePath, "./output/"+input.DataSourceName+".json", input.Username, "")
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

func (w *TemporalWorkflows) DownloadModelActivity(ctx context.Context, input DownloadModelActivityInput) (DownloadModelActivityResponse, error) {
	return DownloadModelActivityResponse{}, nil
}

type IndexDataActivityInput struct{}

type IndexDataActivityResponse struct{}

func (w *TemporalWorkflows) IndexDataActivity(ctx context.Context, input IndexDataActivityInput) (IndexDataActivityResponse, error) {
	return IndexDataActivityResponse{}, nil
}

type CleanUpActivityInput struct{}

type CleanUpActivityResponse struct{}

func (w *TemporalWorkflows) CleanUpActivity(ctx context.Context, input CleanUpActivityInput) (CleanUpActivityResponse, error) {
	return CleanUpActivityResponse{}, nil
}

type CompleteActivityInput struct {
	DataSourceID string `json:"dataSourceID"`
}

type CompleteActivityResponse struct{}

func (w *TemporalWorkflows) CompleteActivity(ctx context.Context, input CompleteActivityInput) (CompleteActivityResponse, error) {
	_, err := w.Store.UpdateDataSource(ctx, input.DataSourceID, true)
	if err != nil {
		return CompleteActivityResponse{}, err
	}
	return CompleteActivityResponse{}, nil
}

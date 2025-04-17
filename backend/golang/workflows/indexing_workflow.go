package workflows

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/dataimport"
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

	indexingState = PROCESSING_DATA

	for _, dataSource := range fetchDataSourcesResponse.DataSources {
		processDataActivityInput := ProcessDataActivityInput{
			DataSourceName: dataSource.Name,
			SourcePath:     dataSource.Path,
			Username:       "",
		}
		var processDataResponse ProcessDataActivityResponse
		err = workflow.ExecuteActivity(ctx, w.ProcessDataActivity, processDataActivityInput).Get(ctx, &processDataResponse)
		if err != nil {
			indexingState = FAILED
			return IndexWorkflowResponse{}, err
		}
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
		DataSources: fetchDataSourcesResponse.DataSources,
	}).Get(ctx, &completeResponse)
	if err != nil {
		indexingState = FAILED
		return IndexWorkflowResponse{}, err
	}

	indexingState = COMPLETED
	return IndexWorkflowResponse{}, nil
}

type FetchDataSourcesActivityInput struct{}

type FetchDataSourcesActivityResponse struct {
	DataSources []*db.DataSource `json:"dataSources"`
}

func (w *TemporalWorkflows) FetchDataSourcesActivity(ctx context.Context, input FetchDataSourcesActivityInput) (FetchDataSourcesActivityResponse, error) {
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

func (w *TemporalWorkflows) ProcessDataActivity(ctx context.Context, input ProcessDataActivityInput) (ProcessDataActivityResponse, error) {
	success, err := dataimport.ProcessSource(input.DataSourceName, input.SourcePath, "./output/"+input.DataSourceName+".json", "xxx", "")
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
	DataSources []*db.DataSource `json:"dataSources"`
}

type CompleteActivityResponse struct{}

func (w *TemporalWorkflows) CompleteActivity(ctx context.Context, input CompleteActivityInput) (CompleteActivityResponse, error) {
	for _, dataSource := range input.DataSources {
		_, err := w.Store.UpdateDataSource(ctx, dataSource.ID, true)
		if err != nil {
			return CompleteActivityResponse{}, err
		}
	}
	return CompleteActivityResponse{}, nil
}

package workflows

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/dataimport"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type IndexWorkflowInput struct {
	DataSourceName string `json:"dataSourceName"`
	SourcePath     string `json:"sourcePath"`
	Username       string `json:"username"`
}

type IndexWorkflowResponse struct{}

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

	if input.DataSourceName == "" {
		return IndexWorkflowResponse{}, errors.New("dataSourceName is required")
	}
	if input.SourcePath == "" {
		return IndexWorkflowResponse{}, errors.New("sourcePath is required")
	}

	fmt.Println("Indexing workflow started")

	var response IndexWorkflowResponse
	err := workflow.ExecuteActivity(ctx, w.ProcessDataActivity, ProcessDataActivityInput{
		DataSourceName: input.DataSourceName,
		SourcePath:     input.SourcePath,
		Username:       input.Username,
	}).Get(ctx, &response)
	if err != nil {
		return IndexWorkflowResponse{}, err
	}

	return response, nil
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

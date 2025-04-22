package indexing

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/EternisAI/enchanted-twin/graph/model"
	dataprocessing "github.com/EternisAI/enchanted-twin/pkg/dataprocessing"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/gmail"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/slack"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/telegram"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/x"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	ollamaapi "github.com/ollama/ollama/api"
	"github.com/pkg/errors"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type IndexWorkflowInput struct{}

type IndexWorkflowResponse struct{}

type IndexingStateQuery struct {
	State model.IndexingState
}

const (
	OLLAMA_COMPLETIONS_MODEL = "gemma3:1b"
	OLLAMA_EMBEDDING_MODEL   = "nomic-embed-text"
)

func (w *IndexingWorkflow) IndexWorkflow(ctx workflow.Context, input IndexWorkflowInput) (IndexWorkflowResponse, error) {
	if w.Store == nil {
		return IndexWorkflowResponse{}, errors.New("store is nil")
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

	var completeResponse CompleteActivityResponse
	err := workflow.SetQueryHandler(ctx, "getIndexingState", func() (IndexingStateQuery, error) {
		return IndexingStateQuery{State: indexingState}, nil
	})
	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to set query handler", "error", err)
		return IndexWorkflowResponse{}, errors.Wrap(err, "failed to set query handler")
	}

	var fetchDataSourcesResponse FetchDataSourcesActivityResponse
	err = workflow.ExecuteActivity(ctx, w.FetchDataSourcesActivity, FetchDataSourcesActivityInput{}).Get(ctx, &fetchDataSourcesResponse)
	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to fetch data sources", "error", err)
		indexingState = model.IndexingStateFailed
		errMsg := err.Error()
		w.publishIndexingStatus(ctx, indexingState, dataSources, 0, 0, &errMsg)
		return IndexWorkflowResponse{}, errors.Wrap(err, "failed to fetch data sources")
	}

	if len(fetchDataSourcesResponse.DataSources) == 0 {
		workflow.GetLogger(ctx).Info("No data sources found")
		indexingState = model.IndexingStateFailed
		errMsg := "No data sources found"
		w.publishIndexingStatus(ctx, indexingState, dataSources, 0, 0, &errMsg)
		return IndexWorkflowResponse{}, errors.New(errMsg)
	}

	indexingState = model.IndexingStateDownloadingModel
	w.publishIndexingStatus(ctx, indexingState, []*model.DataSource{}, 0, 0, nil)

	err = workflow.ExecuteActivity(ctx, w.DownloadOllamaModel, OLLAMA_COMPLETIONS_MODEL).Get(ctx, nil)
	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to download Ollama model", "error", err)
		indexingState = model.IndexingStateFailed
		errMsg := err.Error()
		w.publishIndexingStatus(ctx, indexingState, dataSources, 0, 0, &errMsg)
		return IndexWorkflowResponse{}, errors.Wrap(err, "failed to download Ollama model")
	}

	err = workflow.ExecuteActivity(ctx, w.DownloadOllamaModel, OLLAMA_EMBEDDING_MODEL).Get(ctx, nil)
	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to download Ollama model", "error", err)
		errMsg := err.Error()
		indexingState = model.IndexingStateFailed
		w.publishIndexingStatus(ctx, indexingState, dataSources, 0, 0, &errMsg)
		return IndexWorkflowResponse{}, errors.Wrap(err, "failed to download Ollama model")
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
		err = workflow.ExecuteActivity(ctx, w.ProcessDataActivity, processDataActivityInput).Get(ctx, &processDataResponse)

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
	err = workflow.ExecuteActivity(ctx, w.IndexDataActivity, IndexDataActivityInput{}).Get(ctx, &indexDataResponse)
	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to index data", "error", err)
		errMsg := err.Error()
		indexingState = model.IndexingStateFailed
		w.publishIndexingStatus(ctx, indexingState, dataSources, 100, 0, &errMsg)
		return IndexWorkflowResponse{}, errors.Wrap(err, "failed to index data")
	}

	err = workflow.ExecuteActivity(ctx, w.CompleteActivity, CompleteActivityInput{
		DataSources: dataSources,
	}).Get(ctx, &completeResponse)
	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to complete indexing", "error", err)
		errMsg := err.Error()
		w.publishIndexingStatus(ctx, model.IndexingStateFailed, dataSources, 100, 0, &errMsg)
		return IndexWorkflowResponse{}, fmt.Errorf("failed to complete indexing: %w", err)
	}

	for i := range dataSources {
		dataSources[i].IsIndexed = true
		dataSources[i].UpdatedAt = time.Now().Format(time.RFC3339)
	}

	indexingState = model.IndexingStateCompleted
	workflow.GetLogger(ctx).Info("Indexing completed successfully")
	w.publishIndexingStatus(ctx, model.IndexingStateCompleted, dataSources, 100, 100, nil)
	return IndexWorkflowResponse{}, nil
}

func (w *IndexingWorkflow) publishIndexingStatus(ctx workflow.Context, state model.IndexingState, dataSources []*model.DataSource, processingProgress, indexingProgress int32, error *string) {
	status := &model.IndexingStatus{
		Status:                 state,
		ProcessingDataProgress: processingProgress,
		IndexingDataProgress:   indexingProgress,
		DataSources:            dataSources,
		Error:                  error,
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
	DataSourceID   string `json:"dataSourceID"`
}

type ProcessDataActivityResponse struct {
	Success bool `json:"success"`
}

func (w *IndexingWorkflow) ProcessDataActivity(ctx context.Context, input ProcessDataActivityInput) (ProcessDataActivityResponse, error) {
	// TODO: replace username parameter
	outputPath := fmt.Sprintf("%s%s_%s.jsonl", w.Config.OutputPath, input.DataSourceName, input.DataSourceID)
	success, err := dataprocessing.ProcessSource(input.DataSourceName, input.SourcePath, outputPath, input.Username, "")
	if err != nil {
		w.Logger.Error("Failed to process data source", "error", err, "dataSource", input.DataSourceName)
		return ProcessDataActivityResponse{}, err
	}

	err = w.Store.UpdateDataSourceProcessedPath(ctx, input.DataSourceID, outputPath)
	if err != nil {
		return ProcessDataActivityResponse{}, err
	}

	return ProcessDataActivityResponse{Success: success}, nil
}

type IndexDataActivityInput struct{}

type IndexDataActivityResponse struct{}

func (w *IndexingWorkflow) IndexDataActivity(ctx context.Context) (IndexDataActivityResponse, error) {
	dataSources, err := w.Store.GetUnindexedDataSources(ctx)
	if err != nil {
		return IndexDataActivityResponse{}, err
	}

	for _, dataSource := range dataSources {
		w.Logger.Info("Indexing data source", "dataSource", dataSource.Name)
		if dataSource.ProcessedPath == nil {
			w.Logger.Error("Processed path is nil", "dataSource", dataSource.Name)
			continue
		}

		switch strings.ToLower(dataSource.Name) {
		case "slack":
			documents, err := slack.ToDocuments(*dataSource.ProcessedPath)
			if err != nil {
				return IndexDataActivityResponse{}, err
			}
			w.Logger.Info("Documents", "slack", len(documents))
			err = w.Memory.Store(ctx, documents)
			if err != nil {
				return IndexDataActivityResponse{}, err
			}
			w.Logger.Info("Indexed documents", "documents", len(documents))

		case "telegram":
			documents, err := telegram.ToDocuments(*dataSource.ProcessedPath)
			if err != nil {
				return IndexDataActivityResponse{}, err
			}
			w.Logger.Info("Documents", "telegram", len(documents))
			err = w.Memory.Store(ctx, documents)
			if err != nil {
				return IndexDataActivityResponse{}, err
			}
			w.Logger.Info("Indexed documents", "documents", len(documents))
		case "x":
			documents, err := x.ToDocuments(*dataSource.ProcessedPath)
			if err != nil {
				return IndexDataActivityResponse{}, err
			}
			w.Logger.Info("Documents", "x", len(documents))
			err = w.Memory.Store(ctx, documents)
			if err != nil {
				return IndexDataActivityResponse{}, err
			}
			w.Logger.Info("Indexed documents", "documents", len(documents))
		case "gmail":
			documents, err := gmail.ToDocuments(*dataSource.ProcessedPath)
			if err != nil {
				return IndexDataActivityResponse{}, err
			}
			w.Logger.Info("Documents", "gmail", len(documents))
			err = w.Memory.Store(ctx, documents)
			if err != nil {
				return IndexDataActivityResponse{}, err
			}
			w.Logger.Info("Indexed documents", "documents", len(documents))

		}

	}
	return IndexDataActivityResponse{}, nil
}

type CompleteActivityInput struct {
	DataSources []*model.DataSource `json:"dataSources"`
}

type CompleteActivityResponse struct{}

func (w *IndexingWorkflow) CompleteActivity(ctx context.Context, input CompleteActivityInput) (CompleteActivityResponse, error) {
	for _, dataSource := range input.DataSources {
		_, err := w.Store.UpdateDataSourceState(ctx, dataSource.ID, dataSource.IsIndexed, dataSource.HasError)
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

// This would be for a GraphQL subscription (optional)
type DownloadModelProgress struct {
	PercentageProgress float64
}

func (w *IndexingWorkflow) DownloadOllamaModel(ctx context.Context, modelName string) error {
	models, err := w.OllamaClient.List(context.Background())
	if err != nil {
		w.Logger.Error("Failed to list ollama models", "error", err)
		return err
	}

	modelFound := false
	for _, model := range models.Models {
		if model.Name == modelName {
			modelFound = true
			break
		}
	}

	if modelFound {
		w.Logger.Info("Model already downloaded", "modelName", modelName)
		return nil
	}

	req := &ollamaapi.PullRequest{
		Model: modelName,
	}

	pullProgressFunc := func(progress ollamaapi.ProgressResponse) error {
		if progress.Total == 0 {
			return nil
		}

		percentageProgress := float64(progress.Completed) / float64(progress.Total) * 100

		w.Logger.Info("Download progress", "percentageProgress", percentageProgress)
		userMessageJson, err := json.Marshal(DownloadModelProgress{
			PercentageProgress: percentageProgress,
		})
		if err != nil {
			return err
		}

		err = w.Nc.Publish("onboarding.download_model.progress", userMessageJson)
		if err != nil {
			return err
		}

		return nil
	}

	err = w.OllamaClient.Pull(context.Background(), req, pullProgressFunc)
	if err != nil {
		return err
	}

	w.Logger.Info("Model downloaded", "modelName", modelName)

	return nil
}

package indexing

import (
	"errors"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/EternisAI/enchanted-twin/graph/model"
	"github.com/EternisAI/enchanted-twin/internal/service/graphrag"
	dataprocessing "github.com/EternisAI/enchanted-twin/pkg/dataprocessing"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	ollamaapi "github.com/ollama/ollama/api"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type IndexWorkflowInput struct{}

type IndexWorkflowResponse struct{}

type IndexingStateQuery struct {
	State model.IndexingState
}

const (
	OUTPUT_PATH       = "./output/"
	GRAPHRAG_DATA_DIR = "./data/graphrag"
)

const (
	LOCAL_MODEL     = "gemma3:1b"
	EMBEDDING_MODEL = "nomic-embed-text"
)

func (w *IndexingWorkflow) IndexWorkflow(ctx workflow.Context, input IndexWorkflowInput) (IndexWorkflowResponse, error) {
	if w.Store == nil {
		return IndexWorkflowResponse{}, fmt.Errorf("store is nil")
	}

	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 120 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second * 2,
			MaximumInterval:    time.Minute * 10,
			BackoffCoefficient: 4,
			MaximumAttempts:    2,
		},
	})

	var completeResponse CompleteActivityResponse

	indexingState := model.IndexingStateNotStarted
	dataSources := []*model.DataSource{}
	w.publishIndexingStatus(ctx, model.IndexingStateNotStarted, dataSources, 0, 0, nil)

	err := workflow.SetQueryHandler(ctx, "getIndexingState", func() (IndexingStateQuery, error) {
		return IndexingStateQuery{State: indexingState}, nil
	})
	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to set query handler", "error", err)
		return IndexWorkflowResponse{}, fmt.Errorf("failed to set query handler: %w", err)
	}

	var fetchDataSourcesResponse FetchDataSourcesActivityResponse
	err = workflow.ExecuteActivity(ctx, w.FetchDataSourcesActivity, FetchDataSourcesActivityInput{}).Get(ctx, &fetchDataSourcesResponse)
	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to fetch data sources", "error", err)
		indexingState = model.IndexingStateFailed
		errMsg := err.Error()
		w.publishIndexingStatus(ctx, model.IndexingStateFailed, dataSources, 0, 0, &errMsg)
		return IndexWorkflowResponse{}, fmt.Errorf("failed to fetch data sources: %w", err)
	}

	if len(fetchDataSourcesResponse.DataSources) == 0 {
		workflow.GetLogger(ctx).Info("No data sources found")
		indexingState = model.IndexingStateFailed
		errMsg := "No data sources found"
		w.publishIndexingStatus(ctx, model.IndexingStateFailed, dataSources, 0, 0, &errMsg)
		return IndexWorkflowResponse{}, fmt.Errorf("no data sources found: %w", err)
	}

	indexingState = model.IndexingStateDownloadingModel
	w.publishIndexingStatus(ctx, model.IndexingStateDownloadingModel, []*model.DataSource{}, 0, 0, nil)

	err = workflow.ExecuteActivity(ctx, w.DownloadOllamaModel, LOCAL_MODEL).Get(ctx, nil)
	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to download Ollama model", "error", err)
		indexingState = model.IndexingStateFailed
		errMsg := err.Error()
		w.publishIndexingStatus(ctx, model.IndexingStateFailed, dataSources, 0, 0, &errMsg)
		return IndexWorkflowResponse{}, fmt.Errorf("failed to download Ollama model: %w", err)
	}

	err = workflow.ExecuteActivity(ctx, w.DownloadOllamaModel, EMBEDDING_MODEL).Get(ctx, nil)
	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to download Ollama model", "error", err)
		indexingState = model.IndexingStateFailed
		errMsg := err.Error()
		w.publishIndexingStatus(ctx, model.IndexingStateFailed, dataSources, 0, 0, &errMsg)
		return IndexWorkflowResponse{}, fmt.Errorf("failed to download Ollama model: %w", err)
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
	w.publishIndexingStatus(ctx, model.IndexingStateProcessingData, dataSources, 0, 0, nil)

	for i, dataSource := range fetchDataSourcesResponse.DataSources {
		w.publishIndexingStatus(ctx, model.IndexingStateProcessingData, dataSources, 0, 0, nil)

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
			w.publishIndexingStatus(ctx, model.IndexingStateProcessingData, dataSources, progress, 0, &errMsg)
		} else {
			dataSources[i].IsProcessed = true
			dataSources[i].HasError = false
			dataSources[i].IsIndexed = true // TODO: to update after indexing is implemented
			w.publishIndexingStatus(ctx, model.IndexingStateProcessingData, dataSources, progress, 0, nil)
		}

		w.publishIndexingStatus(ctx, model.IndexingStateProcessingData, dataSources, progress, 0, nil)
	}

	w.publishIndexingStatus(ctx, model.IndexingStateIndexingData, dataSources, 100, 0, nil)

	// Determine the input data directory from processed files
	outputDir := OUTPUT_PATH

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		workflow.GetLogger(ctx).Error("Failed to create output directory", "error", err)
		errMsg := err.Error()
		w.publishIndexingStatus(ctx, model.IndexingStateFailed, dataSources, 100, 0, &errMsg)
		return IndexWorkflowResponse{}, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Ensure graphRAG data directory exists
	graphRAGDataDir := GRAPHRAG_DATA_DIR
	// Create it if it doesn't exist
	if err := os.MkdirAll(graphRAGDataDir, 0755); err != nil {
		workflow.GetLogger(ctx).Error("Failed to create GraphRAG data directory", "error", err)
		errMsg := err.Error()
		w.publishIndexingStatus(ctx, model.IndexingStateFailed, dataSources, 100, 0, &errMsg)
		return IndexWorkflowResponse{}, fmt.Errorf("failed to create GraphRAG data directory: %w", err)
	}

	workflow.GetLogger(ctx).Info("Preparing for indexing",
		slog.String("outputDir", outputDir),
		slog.String("graphRAGDataDir", graphRAGDataDir))

	var indexDataResponse IndexDataActivityResponse
	err = workflow.ExecuteActivity(ctx, w.IndexDataActivity, IndexDataActivityInput{
		DataPath:        outputDir,
		GraphRAGDataDir: graphRAGDataDir,
	}).Get(ctx, &indexDataResponse)
	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to index data", "error", err)
		errMsg := err.Error()
		w.publishIndexingStatus(ctx, model.IndexingStateFailed, dataSources, 100, 0, &errMsg)
		return IndexWorkflowResponse{}, fmt.Errorf("failed to index data: %w", err)

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
	Success    bool   `json:"success"`
	OutputPath string `json:"outputPath"`
}

func (w *IndexingWorkflow) ProcessDataActivity(ctx context.Context, input ProcessDataActivityInput) (ProcessDataActivityResponse, error) {
	// Create a more descriptive output path with both source name and ID
	outputPath := OUTPUT_PATH + input.DataSourceName + "_" + input.DataSourceID + ".jsonl"

	// Use the provided username or a default if not available
	username := input.Username
	if username == "" {
		username = "system"
	}

	// Process the data source
	success, err := dataprocessing.ProcessSource(input.DataSourceName, input.SourcePath, outputPath, username, "")
	if err != nil {
		fmt.Println(err)
		return ProcessDataActivityResponse{}, err
	}
	return ProcessDataActivityResponse{Success: success, OutputPath: outputPath}, nil
}

type IndexDataActivityInput struct {
	DataPath        string `json:"dataPath"`        // Path to the processed data directory
	GraphRAGDataDir string `json:"graphRagDataDir"` // Base directory for GraphRAG data
}

type IndexDataActivityResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func (w *IndexingWorkflow) IndexDataActivity(ctx context.Context, input IndexDataActivityInput) (IndexDataActivityResponse, error) {
	w.Logger.Info("Starting GraphRAG indexing activity",
		slog.String("dataPath", input.DataPath))

	// Validate GraphRAG data directory
	graphRAGDataDir := input.GraphRAGDataDir
	if graphRAGDataDir == "" {
		errMsg := "GraphRAG data directory not provided"
		w.Logger.Error(errMsg)
		return IndexDataActivityResponse{Success: false, Message: errMsg}, errors.New(errMsg)
	}

	// Verify the directory exists
	if _, err := os.Stat(graphRAGDataDir); os.IsNotExist(err) {
		errMsg := fmt.Sprintf("GraphRAG data directory does not exist: %s", graphRAGDataDir)
		w.Logger.Error(errMsg)
		return IndexDataActivityResponse{Success: false, Message: errMsg}, errors.New(errMsg)
	}

	w.Logger.Info("Using GraphRAG data directory", slog.String("graphRAGDataDir", graphRAGDataDir))

	// Use the workflow's GraphRAG service if available, or create a new one if not
	var service *graphrag.GraphRAGService

	if w.GraphRAGService != nil {
		service = w.GraphRAGService
		w.Logger.Info("Using existing GraphRAG service")
	} else {
		// Create a new GraphRAG service for indexing
		w.Logger.Info("Creating new GraphRAG service")
		var err error
		service, err = graphrag.NewGraphRAGService(graphRAGDataDir, "batch", w.Logger)
		if err != nil {
			errMsg := fmt.Sprintf("Failed to create GraphRAG service: %v", err)
			w.Logger.Error(errMsg)
			return IndexDataActivityResponse{Success: false, Message: errMsg}, err
		}
		// Store the service in the workflow struct for future use
		w.GraphRAGService = service
	}

	// Run the indexing process
	if err := service.RunIndexing(ctx, input.DataPath); err != nil {
		errMsg := fmt.Sprintf("Failed to run GraphRAG indexing: %v", err)
		w.Logger.Error(errMsg)
		return IndexDataActivityResponse{Success: false, Message: errMsg}, err
	}

	w.Logger.Info("GraphRAG indexing completed successfully")
	return IndexDataActivityResponse{
		Success: true,
		Message: "GraphRAG indexing completed successfully",
	}, nil
}

type CompleteActivityInput struct {
	DataSources []*model.DataSource `json:"dataSources"`
}

type CompleteActivityResponse struct{}

func (w *IndexingWorkflow) CompleteActivity(ctx context.Context, input CompleteActivityInput) (CompleteActivityResponse, error) {
	// Cleanup GraphRAG service if it exists
	if w.GraphRAGService != nil {
		w.Logger.Info("Closing GraphRAG service")
		if err := w.GraphRAGService.Close(); err != nil {
			w.Logger.Error("Failed to close GraphRAG service", slog.Any("error", err))
		}
		w.GraphRAGService = nil
	}

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

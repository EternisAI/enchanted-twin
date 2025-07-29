package evolvingmemory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/charmbracelet/log"
)

// ContainerJinaClient implements ai.Embedding interface using containerized JinaAI service.
type ContainerJinaClient struct {
	baseURL    string
	httpClient *http.Client
	logger     *log.Logger
	model      string
}

// NewContainerJinaClient creates a new client for containerized JinaAI embeddings.
func NewContainerJinaClient(baseURL string, logger *log.Logger) *ContainerJinaClient {
	return &ContainerJinaClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
		model:  "jina-embeddings-v3", // Default model for container
	}
}

// GetEmbeddings implements ai.Embedding interface.
func (c *ContainerJinaClient) GetEmbeddings(ctx context.Context, texts []string, model string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("no texts provided for embedding")
	}

	// Use provided model or fall back to default
	embeddingModel := model
	if embeddingModel == "" {
		embeddingModel = c.model
	}

	c.logger.Debug("Getting embeddings from container",
		"texts_count", len(texts),
		"model", embeddingModel,
		"endpoint", c.baseURL,
	)

	// Prepare request payload compatible with JinaAI API
	payload := map[string]interface{}{
		"model": embeddingModel,
		"input": texts,
		"task":  "retrieval.query", // Task-specific embedding
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse response
	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
		Model string `json:"model"`
		Usage struct {
			PromptTokens int `json:"prompt_tokens"`
			TotalTokens  int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Data) != len(texts) {
		return nil, fmt.Errorf("expected %d embeddings, got %d", len(texts), len(result.Data))
	}

	// Extract embeddings
	embeddings := make([][]float32, len(result.Data))
	for i, d := range result.Data {
		embeddings[i] = d.Embedding
	}

	c.logger.Debug("Successfully retrieved embeddings",
		"count", len(embeddings),
		"model", result.Model,
		"tokens", result.Usage.TotalTokens,
	)

	return embeddings, nil
}

// ContainerONNXClient provides access to ONNX Runtime server.
type ContainerONNXClient struct {
	baseURL    string
	httpClient *http.Client
	logger     *log.Logger
}

// NewContainerONNXClient creates a new client for containerized ONNX Runtime.
func NewContainerONNXClient(baseURL string, logger *log.Logger) *ContainerONNXClient {
	return &ContainerONNXClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// ListModels returns available models from ONNX Runtime server.
func (c *ContainerONNXClient) ListModels(ctx context.Context) ([]string, error) {
	c.logger.Debug("Listing models from ONNX container", "endpoint", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/v2/models", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result struct {
		Models []struct {
			Name     string   `json:"name"`
			Versions []string `json:"versions,omitempty"`
			Platform string   `json:"platform,omitempty"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	models := make([]string, len(result.Models))
	for i, m := range result.Models {
		models[i] = m.Name
	}

	c.logger.Debug("Listed models from ONNX container", "count", len(models))
	return models, nil
}

// HealthCheck verifies the ONNX server is ready.
func (c *ContainerONNXClient) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/v2/health/ready", nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status: %d", resp.StatusCode)
	}

	return nil
}

// ContainerCompletionClient provides access to completion model server.
type ContainerCompletionClient struct {
	baseURL    string
	httpClient *http.Client
	logger     *log.Logger
}

// NewContainerCompletionClient creates a new client for containerized completion model.
func NewContainerCompletionClient(baseURL string, logger *log.Logger) *ContainerCompletionClient {
	return &ContainerCompletionClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 60 * time.Second, // Longer timeout for completions
		},
		logger: logger,
	}
}

// CreateChatCompletion creates a chat completion using the container model.
func (c *ContainerCompletionClient) CreateChatCompletion(ctx context.Context, messages []map[string]string, model string) (string, error) {
	c.logger.Debug("Creating chat completion via container",
		"messages_count", len(messages),
		"model", model,
		"endpoint", c.baseURL,
	)

	// Convert messages to the expected format
	formattedMessages := make([]map[string]string, len(messages))
	for i, msg := range messages {
		formattedMessages[i] = map[string]string{
			"role":    msg["role"],
			"content": msg["content"],
		}
	}

	// Prepare request payload
	payload := map[string]interface{}{
		"model":       model,
		"messages":    formattedMessages,
		"max_tokens":  200,
		"temperature": 0.7,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse response
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no completion choices returned")
	}

	content := result.Choices[0].Message.Content
	c.logger.Debug("Successfully created chat completion", "response_length", len(content))

	return content, nil
}

// HealthCheck verifies the completion server is ready.
func (c *ContainerCompletionClient) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/health", nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status: %d", resp.StatusCode)
	}

	return nil
}

// ContainerEmbeddingService wraps container clients to implement ai.Embedding.
type ContainerEmbeddingService struct {
	jinaClient *ContainerJinaClient
	onnxClient *ContainerONNXClient
	logger     *log.Logger
}

// NewContainerEmbeddingService creates embedding service using test containers.
func NewContainerEmbeddingService(containers *TestContainerSuite, logger *log.Logger) *ContainerEmbeddingService {
	return &ContainerEmbeddingService{
		jinaClient: NewContainerJinaClient(containers.JinaEndpoint, logger),
		onnxClient: NewContainerONNXClient(containers.ONNXEndpoint, logger),
		logger:     logger,
	}
}

// GetEmbeddings provides access to the underlying embedding functionality.
func (s *ContainerEmbeddingService) GetEmbeddings(ctx context.Context, texts []string, model string) ([][]float32, error) {
	return s.jinaClient.GetEmbeddings(ctx, texts, model)
}

// Embedding implements ai.Embedding interface for single text.
func (s *ContainerEmbeddingService) Embedding(ctx context.Context, input string, model string) ([]float64, error) {
	embeddings, err := s.jinaClient.GetEmbeddings(ctx, []string{input}, model)
	if err != nil {
		return nil, err
	}

	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	// Convert float32 to float64
	result := make([]float64, len(embeddings[0]))
	for i, val := range embeddings[0] {
		result[i] = float64(val)
	}

	return result, nil
}

// Embeddings implements ai.Embedding interface for multiple texts.
func (s *ContainerEmbeddingService) Embeddings(ctx context.Context, inputs []string, model string) ([][]float64, error) {
	embeddings, err := s.jinaClient.GetEmbeddings(ctx, inputs, model)
	if err != nil {
		return nil, err
	}

	// Convert [][]float32 to [][]float64
	result := make([][]float64, len(embeddings))
	for i, embedding := range embeddings {
		result[i] = make([]float64, len(embedding))
		for j, val := range embedding {
			result[i][j] = float64(val)
		}
	}

	return result, nil
}

// HealthCheck verifies all container services are healthy.
func (s *ContainerEmbeddingService) HealthCheck(ctx context.Context) error {
	// Check ONNX service
	if err := s.onnxClient.HealthCheck(ctx); err != nil {
		return fmt.Errorf("onnx service unhealthy: %w", err)
	}

	// Check Jina service by getting embeddings for a test text
	_, err := s.jinaClient.GetEmbeddings(ctx, []string{"health check"}, "")
	if err != nil {
		return fmt.Errorf("jina service unhealthy: %w", err)
	}

	s.logger.Debug("All container services are healthy")
	return nil
}

// ContainerAIService provides full AI service using test containers with real models.
type ContainerAIService struct {
	embeddingService *ContainerEmbeddingService
	completionClient *ContainerCompletionClient
	logger           *log.Logger
}

// NewContainerAIService creates a full AI service that uses containers for both embeddings and completions.
func NewContainerAIService(containers *TestContainerSuite, logger *log.Logger) *ContainerAIService {
	return &ContainerAIService{
		embeddingService: NewContainerEmbeddingService(containers, logger),
		completionClient: NewContainerCompletionClient(containers.CompletionEndpoint, logger),
		logger:           logger,
	}
}

// Embedding implements ai.Embedding interface for single text.
func (s *ContainerAIService) Embedding(ctx context.Context, input string, model string) ([]float64, error) {
	return s.embeddingService.Embedding(ctx, input, model)
}

// Embeddings implements ai.Embedding interface for multiple texts.
func (s *ContainerAIService) Embeddings(ctx context.Context, inputs []string, model string) ([][]float64, error) {
	return s.embeddingService.Embeddings(ctx, inputs, model)
}

// Note: Completions interface implementation will be added later
// For now, focusing on getting the real JinaAI embeddings working properly

// HealthCheck verifies all container services are healthy.
func (s *ContainerAIService) HealthCheck(ctx context.Context) error {
	// Check embedding service (primary focus)
	if err := s.embeddingService.HealthCheck(ctx); err != nil {
		return fmt.Errorf("embedding service unhealthy: %w", err)
	}

	s.logger.Debug("Container AI services are healthy (embeddings verified)")
	return nil
}

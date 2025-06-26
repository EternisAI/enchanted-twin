package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/charmbracelet/log"
)

type ClipEmbeddingService struct {
	baseURL string
	client  *http.Client
	logger  *log.Logger
}

type TextEmbeddingRequest struct {
	Text string `json:"text"`
}

type TextEmbeddingResponse struct {
	Embedding []float64 `json:"embedding"`
	Text      string    `json:"text"`
}

type ImageEmbeddingRequest struct {
	ImagePaths []string `json:"image_paths"`
}

type ImageEmbeddingResponse struct {
	Embeddings [][]float64 `json:"embeddings"`
	ImagePaths []string    `json:"image_paths"`
}

type TextToImageSearchRequest struct {
	Text  string `json:"text"`
	Limit int    `json:"limit"`
}

type TextToImageSearchResponse struct {
	ImagePaths []string `json:"image_paths"`
	TotalFound int      `json:"total_found"`
}

func NewClipEmbeddingService(logger *log.Logger, baseURL string) *ClipEmbeddingService {
	if baseURL == "" {
		baseURL = "http://localhost:8000" //need to be changed to ip
	}

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	return &ClipEmbeddingService{
		baseURL: baseURL,
		client:  httpClient,
		logger:  logger,
	}
}

func (s *ClipEmbeddingService) TextEmbeddings(
	ctx context.Context,
	inputs []string,
) ([][]float64, error) {

	s.logger.Info("starting batch text embeddings", "count", len(inputs))
	var allEmbeddings [][]float64

	for i, input := range inputs {
		s.logger.Debug("embedding text", "index", i, "text_length", len(input))
		embedding, err := s.TextEmbedding(ctx, input)
		if err != nil {
			s.logger.Error("failed to embed text", "index", i, "error", err)
			return nil, fmt.Errorf("failed to embed text '%s': %w", input, err)
		}
		allEmbeddings = append(allEmbeddings, embedding)
	}

	s.logger.Info("completed batch text embeddings", "count", len(allEmbeddings))
	return allEmbeddings, nil
}

func (s *ClipEmbeddingService) TextEmbedding(
	ctx context.Context,
	input string,
) ([]float64, error) {

	s.logger.Debug("creating text embedding request", "text_length", len(input))

	requestBody := TextEmbeddingRequest{
		Text: input,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		s.logger.Error("failed to marshal text embedding request", "error", err)
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.baseURL+"/embed/text", bytes.NewBuffer(jsonData))

	if err != nil {
		s.logger.Error("failed to create text embedding request", "error", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	s.logger.Debug("making text embedding request", "url", req.URL.String())
	resp, err := s.client.Do(req)
	if err != nil {
		s.logger.Error("failed to make text embedding request", "error", err)
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	s.logger.Debug("received text embedding response", "status", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		s.logger.Error("text embedding request failed", "status", resp.StatusCode)
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	var response TextEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		s.logger.Error("failed to decode text embedding response", "error", err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	s.logger.Debug("successfully created text embedding", "embedding_size", len(response.Embedding))
	return response.Embedding, nil
}

func (s *ClipEmbeddingService) ImageEmbeddings(
	ctx context.Context,
	imagePaths []string,
) ([][]float64, error) {

	s.logger.Info("starting batch image embeddings", "count", len(imagePaths))

	requestBody := ImageEmbeddingRequest{
		ImagePaths: imagePaths,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		s.logger.Error("failed to marshal image embedding request", "error", err)
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.baseURL+"/embed/images", bytes.NewBuffer(jsonData))
	if err != nil {
		s.logger.Error("failed to create image embedding request", "error", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	s.logger.Debug("making image embedding request", "url", req.URL.String(), "image_count", len(imagePaths))
	resp, err := s.client.Do(req)
	if err != nil {
		s.logger.Error("failed to make image embedding request", "error", err)
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	s.logger.Debug("received image embedding response", "status", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		s.logger.Error("image embedding request failed", "status", resp.StatusCode)
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	var response ImageEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		s.logger.Error("failed to decode image embedding response", "error", err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	s.logger.Info("completed batch image embeddings", "count", len(response.Embeddings))
	return response.Embeddings, nil
}

func (s *ClipEmbeddingService) ImageEmbedding(
	ctx context.Context,
	imagePath string,
) ([]float64, error) {

	s.logger.Debug("creating single image embedding", "image_path", imagePath)

	embeddings, err := s.ImageEmbeddings(ctx, []string{imagePath})
	if err != nil {
		s.logger.Error("failed to create single image embedding", "image_path", imagePath, "error", err)
		return nil, err
	}

	if len(embeddings) == 0 {
		s.logger.Error("no embedding returned for image", "image_path", imagePath)
		return nil, fmt.Errorf("no embedding returned for image")
	}

	s.logger.Debug("successfully created single image embedding", "embedding_size", len(embeddings[0]))
	return embeddings[0], nil
}

func (s *ClipEmbeddingService) ImageSearch(
	ctx context.Context,
	text string,
	limit int,
) (*TextToImageSearchResponse, error) {

	s.logger.Debug("creating image search request", "text", text, "limit", limit)

	requestBody := TextToImageSearchRequest{
		Text:  text,
		Limit: limit,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		s.logger.Error("failed to marshal image search request", "error", err)
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.baseURL+"/search/images", bytes.NewBuffer(jsonData))
	if err != nil {
		s.logger.Error("failed to create image search request", "error", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	s.logger.Debug("making image search request", "url", req.URL.String())
	resp, err := s.client.Do(req)
	if err != nil {
		s.logger.Error("failed to make image search request", "error", err)
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	s.logger.Debug("received image search response", "status", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		s.logger.Error("image search request failed", "status", resp.StatusCode)
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	var response TextToImageSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		s.logger.Error("failed to decode image search response", "error", err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	s.logger.Debug("successfully completed image search", "total_found", response.TotalFound, "image_count", len(response.ImagePaths))
	return &response, nil
}

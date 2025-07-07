package coreml

import (
	"context"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go"

	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

type Service struct {
	embeddingService   *EmbeddingService
	completionService  *CompletionService
	fallbackEmbedding  ai.Embeddings
	fallbackCompletion ai.Completions
	logger             *log.Logger
}

func NewService(logger *log.Logger, binaryPath, modelPath string, interactive bool, fallbackEmbedding ai.Embeddings, fallbackCompletion ai.Completions) *Service {
	return &Service{
		embeddingService:   NewEmbeddingService(binaryPath, modelPath, interactive),
		completionService:  NewCompletionService(binaryPath, modelPath, interactive),
		fallbackEmbedding:  fallbackEmbedding,
		fallbackCompletion: fallbackCompletion,
		logger:             logger,
	}
}

func (s *Service) Embeddings(ctx context.Context, inputs []string, model string) ([][]float64, error) {
	if s.embeddingService != nil {
		s.logger.Debug("Attempting CoreML embeddings", "input_count", len(inputs), "model", model)

		embeddings, err := s.embeddingService.Embeddings(ctx, inputs, model)
		if err != nil {
			s.logger.Warn("CoreML embeddings failed, falling back", "error", err, "input_count", len(inputs))
		} else {
			s.logger.Debug("CoreML embeddings succeeded", "input_count", len(inputs), "embedding_count", len(embeddings))
			return embeddings, nil
		}
	}

	s.logger.Debug("Using fallback embeddings service", "input_count", len(inputs), "model", model)
	return s.fallbackEmbedding.Embeddings(ctx, inputs, model)
}

func (s *Service) Embedding(ctx context.Context, input string, model string) ([]float64, error) {
	if s.embeddingService != nil {
		s.logger.Debug("Attempting CoreML embedding", "input_length", len(input), "model", model)

		embedding, err := s.embeddingService.Embedding(ctx, input, model)
		if err != nil {
			s.logger.Warn("CoreML embedding failed, falling back", "error", err, "input_length", len(input))
		} else {
			s.logger.Debug("CoreML embedding succeeded", "input_length", len(input), "embedding_dimension", len(embedding))
			return embedding, nil
		}
	}

	s.logger.Debug("Using fallback embedding service", "input_length", len(input), "model", model)
	return s.fallbackEmbedding.Embedding(ctx, input, model)
}

func (s *Service) Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) (openai.ChatCompletionMessage, error) {
	if s.completionService != nil {
		s.logger.Debug("Attempting CoreML completions", "message_count", len(messages), "model", model)

		completion, err := s.completionService.Completions(ctx, messages, tools, model)
		if err != nil {
			s.logger.Warn("CoreML completions failed, falling back", "error", err, "message_count", len(messages))
		} else {
			s.logger.Debug("CoreML completions succeeded", "message_count", len(messages))
			return completion, nil
		}
	}

	s.logger.Debug("Using fallback completions service", "message_count", len(messages), "model", model)
	return s.fallbackCompletion.Completions(ctx, messages, tools, model)
}

func (s *Service) Start(ctx context.Context) error {
	if s.embeddingService != nil {
		s.logger.Info("Starting CoreML embedding service")
		if err := s.embeddingService.Start(ctx); err != nil {
			s.logger.Warn("Failed to start CoreML embedding service", "error", err)
		}
	}

	if s.completionService != nil {
		s.logger.Info("Starting CoreML completion service")
		if err := s.completionService.Start(ctx); err != nil {
			s.logger.Warn("Failed to start CoreML completion service", "error", err)
		}
	}

	return nil
}

func (s *Service) Close() error {
	if s.embeddingService != nil {
		s.logger.Info("Closing CoreML embedding service")
		if err := s.embeddingService.Close(); err != nil {
			s.logger.Warn("Failed to close CoreML embedding service", "error", err)
		}
	}

	if s.completionService != nil {
		s.logger.Info("Closing CoreML completion service")
		if err := s.completionService.Close(); err != nil {
			s.logger.Warn("Failed to close CoreML completion service", "error", err)
		}
	}

	return nil
}

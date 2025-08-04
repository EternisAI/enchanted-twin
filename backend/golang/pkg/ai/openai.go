// Owner: august@eternis.ai
package ai

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
)

type Config struct {
	APIKey  string
	BaseUrl string
}

var _ Embedding = (*Service)(nil)

type Service struct {
	client             *openai.Client
	logger             *log.Logger
	getAccessToken     func() (string, error)
	opts               []option.RequestOption
	privateCompletions PrivateCompletions // Optional private completions service
	defaultPriority    Priority           // Default priority for private completions
	rateLimiter        *RateLimiter       // Rate limiter for API calls
}

func NewOpenAIService(logger *log.Logger, apiKey string, baseUrl string) *Service {
	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(baseUrl),
	)

	// Create rate limiter: 60 requests per minute (1 per second average)
	rateLimiter := NewRateLimiter(60, time.Minute)

	return &Service{
		client:      &client,
		logger:      logger,
		rateLimiter: rateLimiter,
	}
}

func NewOpenAIServiceProxy(logger *log.Logger, getFirebaseToken func() (string, error), proxyUrl string, baseUrl string) *Service {
	opts := []option.RequestOption{
		option.WithBaseURL(proxyUrl),
		option.WithHeader("X-BASE-URL", baseUrl),
	}

	client := openai.NewClient(opts...)

	// Create rate limiter: 60 requests per minute (1 per second average)
	rateLimiter := NewRateLimiter(60, time.Minute)

	return &Service{
		client:         &client,
		logger:         logger,
		getAccessToken: getFirebaseToken,
		opts:           opts,
		rateLimiter:    rateLimiter,
	}
}

func (s *Service) ParamsCompletions(ctx context.Context, params openai.ChatCompletionNewParams) (openai.ChatCompletionMessage, error) {
	// Apply rate limiting before making API call
	if s.rateLimiter != nil {
		if err := s.rateLimiter.Wait(ctx); err != nil {
			return openai.ChatCompletionMessage{}, fmt.Errorf("rate limit wait failed: %w", err)
		}
	}

	opts := s.opts
	s.logger.Info("ParamsCompletions", "opts", opts)
	if s.getAccessToken != nil {
		firebaseToken, err := s.getAccessToken()
		if err != nil {
			return openai.ChatCompletionMessage{}, err
		}
		opts = append(opts, option.WithHeader("Authorization", "Bearer "+firebaseToken))
	}

	completion, err := s.client.Chat.Completions.New(ctx, params, opts...)
	if err != nil {
		return openai.ChatCompletionMessage{}, err
	}

	if len(completion.Choices) == 0 {
		return openai.ChatCompletionMessage{}, fmt.Errorf("OpenAI returned no completion choices")
	}

	return completion.Choices[0].Message, nil
}

// Completions provides the main completion interface - now always returns PrivateCompletionResult.
func (s *Service) Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string, priority Priority) (PrivateCompletionResult, error) {
	// Always use private completions (with fallback for backward compatibility)
	if s.privateCompletions != nil {
		return s.privateCompletions.Completions(ctx, messages, tools, model, priority)
	}

	// Fallback to regular completion and wrap in PrivateCompletionResult
	message, err := s.ParamsCompletions(ctx, openai.ChatCompletionNewParams{
		Messages:    messages,
		Model:       model,
		Tools:       tools,
		Temperature: param.Opt[float64]{Value: 1.0},
	})
	if err != nil {
		return PrivateCompletionResult{}, err
	}

	return PrivateCompletionResult{
		Message:          message,
		ReplacementRules: make(map[string]string), // Empty rules when not using private completions
	}, nil
}

// RawCompletions bypasses private completions and calls the underlying completion service directly.
// This is used internally by the private completions service to avoid circular dependencies.
func (s *Service) RawCompletions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) (PrivateCompletionResult, error) {
	// Always call the raw completion method, bypassing private completions
	message, err := s.ParamsCompletions(ctx, openai.ChatCompletionNewParams{
		Messages:    messages,
		Model:       model,
		Tools:       tools,
		Temperature: param.Opt[float64]{Value: 1.0},
	})
	if err != nil {
		return PrivateCompletionResult{}, err
	}

	return PrivateCompletionResult{
		Message:          message,
		ReplacementRules: make(map[string]string), // Empty rules when not using private completions
	}, nil
}

// EnablePrivateCompletions configures the service to use private completions.
func (s *Service) EnablePrivateCompletions(privateCompletions PrivateCompletions, defaultPriority Priority) {
	s.privateCompletions = privateCompletions
	s.defaultPriority = defaultPriority
}

// CompletionsStreamWithPrivacy provides streaming with private completions support.
func (s *Service) CompletionsStreamWithPrivacy(ctx context.Context, conversationID string, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string, onDelta func(StreamDelta)) (PrivateCompletionResult, error) {
	// Always use private completions if available
	if s.privateCompletions != nil {
		return s.privateCompletions.CompletionsStreamWithContext(ctx, conversationID, messages, tools, model, s.defaultPriority, onDelta)
	}

	// Fallback to regular streaming (without anonymization)
	stream := s.CompletionsStream(ctx, messages, tools, model)

	var accumulatedContent strings.Builder
	var finalMessage openai.ChatCompletionMessage
	var toolCalls []openai.ChatCompletionMessageToolCall

fallbackLoop:
	for {
		select {
		case delta, ok := <-stream.Content:
			if !ok {
				stream.Content = nil
			} else {
				accumulatedContent.WriteString(delta.ContentDelta)
				currentAccumulated := accumulatedContent.String()

				// Call onDelta with regular content (no anonymization)
				if onDelta != nil {
					onDelta(StreamDelta{
						ContentDelta:                   delta.ContentDelta,
						IsCompleted:                    delta.IsCompleted,
						AccumulatedAnonymizedMessage:   currentAccumulated, // Same content
						AccumulatedDeanonymizedMessage: currentAccumulated, // Same content
					})
				}

				if delta.IsCompleted {
					finalMessage.Content = currentAccumulated
					finalMessage.Role = "assistant"
				}
			}

		case toolCall, ok := <-stream.ToolCalls:
			if !ok {
				stream.ToolCalls = nil
			} else {
				toolCalls = append(toolCalls, toolCall)
			}

		case err, ok := <-stream.Err:
			if !ok {
				stream.Err = nil
			} else {
				if err != nil {
					return PrivateCompletionResult{}, err
				}
			}

		case <-ctx.Done():
			return PrivateCompletionResult{}, ctx.Err()
		}

		// Check if all channels are closed
		if stream.Content == nil && stream.ToolCalls == nil && stream.Err == nil {
			break fallbackLoop
		}
	}

	if len(toolCalls) > 0 {
		finalMessage.ToolCalls = toolCalls
	}

	return PrivateCompletionResult{
		Message:          finalMessage,
		ReplacementRules: make(map[string]string), // Empty rules when not using private completions
	}, nil
}

func (s *Service) Embeddings(ctx context.Context, inputs []string, model string) ([][]float64, error) {
	// Apply rate limiting before making API call
	if s.rateLimiter != nil {
		if err := s.rateLimiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limit wait failed: %w", err)
		}
	}

	opts := s.opts
	if s.getAccessToken != nil {
		firebaseToken, err := s.getAccessToken()
		if err != nil {
			return nil, err
		}
		opts = append(opts, option.WithHeader("Authorization", "Bearer "+firebaseToken))
	}

	embedding, err := s.client.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Model: model,
		Input: openai.EmbeddingNewParamsInputUnion{
			OfArrayOfStrings: inputs,
		},
	}, opts...)
	if err != nil {
		return nil, err
	}
	var embeddings [][]float64
	for _, embedding := range embedding.Data {
		embeddings = append(embeddings, embedding.Embedding)
	}
	return embeddings, nil
}

func (s *Service) Embedding(ctx context.Context, input string, model string) ([]float64, error) {
	// Apply rate limiting before making API call
	if s.rateLimiter != nil {
		if err := s.rateLimiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limit wait failed: %w", err)
		}
	}

	opts := s.opts
	if s.getAccessToken != nil {
		firebaseToken, err := s.getAccessToken()
		if err != nil {
			return nil, err
		}
		opts = append(opts, option.WithHeader("Authorization", "Bearer "+firebaseToken))
	}

	embedding, err := s.client.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Model: model,
		Input: openai.EmbeddingNewParamsInputUnion{
			OfString: param.Opt[string]{
				Value: input,
			},
		},
	}, opts...)
	if err != nil {
		return nil, err
	}
	return embedding.Data[0].Embedding, nil
}

// Close stops the rate limiter and cleans up resources.
func (s *Service) Close() {
	if s.rateLimiter != nil {
		s.rateLimiter.Stop()
	}
}

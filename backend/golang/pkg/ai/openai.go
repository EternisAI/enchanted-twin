// Owner: august@eternis.ai
package ai

import (
	"context"
	"fmt"
	"net/http"
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

type Service struct {
	client *openai.Client
	logger *log.Logger
}

func NewOpenAIService(logger *log.Logger, apiKey string, baseUrl string) *Service {
	httpTimeout := 20 * time.Minute

	httpClient := &http.Client{
		Timeout: httpTimeout,
	}

	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(baseUrl),
		option.WithHTTPClient(httpClient),
	)
	return &Service{
		client: &client,
		logger: logger,
	}
}

func NewOpenAIServiceProxy(logger *log.Logger, proxyUrl string, getToken func() string, baseUrl string) *Service {
	client := openai.NewClient(option.WithAPIKey(getToken()), option.WithBaseURL(proxyUrl), option.WithHeader("X-BASE-URL", baseUrl))
	return &Service{
		client: &client,
		logger: logger,
	}
}

func NewOpenAIServiceProxy(logger *log.Logger, proxyUrl string, getToken func() string, baseUrl string) *Service {
	client := openai.NewClient(option.WithAPIKey(getToken()), option.WithBaseURL(proxyUrl), option.WithHeader("X-BASE-URL", baseUrl))
	return &Service{
		client: &client,
		logger: logger,
	}
}

func (s *Service) ParamsCompletions(ctx context.Context, params openai.ChatCompletionNewParams) (openai.ChatCompletionMessage, error) {
	completion, err := s.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return openai.ChatCompletionMessage{}, err
	}

	if len(completion.Choices) == 0 {
		return openai.ChatCompletionMessage{}, fmt.Errorf("OpenAI returned no completion choices")
	}

	return completion.Choices[0].Message, nil
}

func (s *Service) Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) (openai.ChatCompletionMessage, error) {
	return s.ParamsCompletions(ctx, openai.ChatCompletionNewParams{
		Messages:    messages,
		Model:       model,
		Tools:       tools,
		Temperature: param.Opt[float64]{Value: 1.0},
	})
}

// CompletionsWithMessages executes a completion using our internal message format.
func (s *Service) CompletionsWithMessages(ctx context.Context, messages []Message, tools []openai.ChatCompletionToolParam, model string) (Message, error) {
	// Convert our messages to OpenAI format
	openaiMessages := ToOpenAIMessages(messages)

	// Execute the completion
	completion, err := s.Completions(ctx, openaiMessages, tools, model)
	if err != nil {
		return Message{}, err
	}

	// Convert result back to our format
	return FromOpenAIMessage(completion), nil
}

func (s *Service) Embeddings(ctx context.Context, inputs []string, model string) ([][]float64, error) {
	embedding, err := s.client.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Model: model,
		Input: openai.EmbeddingNewParamsInputUnion{
			OfArrayOfStrings: inputs,
		},
	})
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
	embedding, err := s.client.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Model: model,
		Input: openai.EmbeddingNewParamsInputUnion{
			OfString: param.Opt[string]{
				Value: input,
			},
		},
	})
	if err != nil {
		return nil, err
	}
	return embedding.Data[0].Embedding, nil
}

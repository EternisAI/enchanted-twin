// Owner: august@eternis.ai
package ai

import (
	"context"
	"fmt"

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
	client      *openai.Client
	logger      *log.Logger
	getApiToken func() string
}

func NewOpenAIService(logger *log.Logger, apiKey string, baseUrl string) *Service {
	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(baseUrl),
	)
	return &Service{
		client: &client,
		logger: logger,
	}
}

func NewOpenAIServiceProxy(logger *log.Logger, proxyUrl string, getApiToken func() string, baseUrl string) *Service {
	opts := []option.RequestOption{
		option.WithBaseURL(proxyUrl),
		option.WithHeader("X-BASE-URL", baseUrl),
	}

	client := openai.NewClient(opts...)
	return &Service{
		client:      &client,
		logger:      logger,
		getApiToken: getApiToken,
	}
}

func (s *Service) ParamsCompletions(ctx context.Context, params openai.ChatCompletionNewParams) (openai.ChatCompletionMessage, error) {
	var opts []option.RequestOption
	if s.getApiToken != nil {
		opts = append(opts, option.WithAPIKey(s.getApiToken()))
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

func (s *Service) Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) (openai.ChatCompletionMessage, error) {
	return s.ParamsCompletions(ctx, openai.ChatCompletionNewParams{
		Messages:    messages,
		Model:       model,
		Tools:       tools,
		Temperature: param.Opt[float64]{Value: 1.0},
	})
}

func (s *Service) Embeddings(ctx context.Context, inputs []string, model string) ([][]float64, error) {
	var opts []option.RequestOption
	if s.getApiToken != nil {
		opts = append(opts, option.WithAPIKey(s.getApiToken()))
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
	var opts []option.RequestOption
	if s.getApiToken != nil {
		opts = append(opts, option.WithAPIKey(s.getApiToken()))
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

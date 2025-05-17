// Owner: august@eternis.ai
package ai

import (
	"context"
	"errors"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
)

type Service struct {
	client *openai.Client
	logger *log.Logger
}

func NewOpenAIService(logger *log.Logger, apiKey string, baseUrl string) *Service {
	client := openai.NewClient(option.WithAPIKey(apiKey), option.WithBaseURL(baseUrl))
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
		s.logger.Error("no choices returned", "params", params)
		return openai.ChatCompletionMessage{}, errors.New("no choices returned")
	}
	return completion.Choices[0].Message, nil
}

func (s *Service) Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) (openai.ChatCompletionMessage, error) {
	return s.ParamsCompletions(ctx, openai.ChatCompletionNewParams{
		Messages: messages,
		Model:    model,
		Tools:    tools,
	})
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

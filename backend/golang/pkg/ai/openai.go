// Owner: august@eternis.ai
package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/tinfoilsh/tinfoil-go"
)

type Config struct {
	APIKey         string
	BaseUrl        string
	EnclaveAndRepo string
	UseTinfoil     bool
}

type Service struct {
	client *openai.Client
}

func NewOpenAIService(apiKey string, baseUrl string, enclaveAndRepo string, useTinfoilTEE bool) (*Service, error) {
	var client openai.Client

	if useTinfoilTEE {
		enclaveAndRepoParts := strings.Split(enclaveAndRepo, ",")
		fmt.Println("enclave", enclaveAndRepoParts[0])
		fmt.Println("repo", enclaveAndRepoParts[1])
		if len(enclaveAndRepoParts) != 2 {
			return nil, errors.New("enclaveAndRepo must be in the format <enclave>,<repo>")
		}
		enclave := enclaveAndRepoParts[0]
		repo := enclaveAndRepoParts[1]
		client, err := tinfoil.NewClientWithParams(enclave, repo,
			option.WithAPIKey(apiKey),
		)
		if err != nil {
			return nil, err
		}
		completion, err := client.Client.Chat.Completions.New(
			context.Background(),
			openai.ChatCompletionNewParams{
				Model: "llama3-3-70b",
				Messages: []openai.ChatCompletionMessageParamUnion{
					openai.UserMessage("Hello!"),
				},
			},
		)
		if err != nil {
			panic(err)
		}

		fmt.Println(completion.Choices[0].Message.Content)

		stream := client.Chat.Completions.NewStreaming(
			context.Background(),
			openai.ChatCompletionNewParams{
				Model: "llama3-3-70b",
				Messages: []openai.ChatCompletionMessageParamUnion{
					openai.UserMessage("Hello!"),
				},
			},
		)
		for stream.Next() {
			chunk := stream.Current()
			fmt.Println(chunk)

		}
		return &Service{
			client: client.Client,
		}, nil
	}

	client = openai.NewClient(option.WithAPIKey(apiKey), option.WithBaseURL(baseUrl))
	return &Service{
		client: &client,
	}, nil
}

func (s *Service) ParamsCompletions(ctx context.Context, params openai.ChatCompletionNewParams) (openai.ChatCompletionMessage, error) {
	fmt.Println("model_here", params.Model)
	completion, err := s.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return openai.ChatCompletionMessage{}, err
	}
	return completion.Choices[0].Message, nil
}

func (s *Service) Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) (openai.ChatCompletionMessage, error) {
	fmt.Println("model_here", model)
	return s.ParamsCompletions(ctx, openai.ChatCompletionNewParams{
		Messages: messages,
		Model:    model,
		Tools:    tools,
	})
}

// CompletionsWithMessages executes a completion using our internal message format.
func (s *Service) CompletionsWithMessages(ctx context.Context, messages []Message, tools []openai.ChatCompletionToolParam, model string) (Message, error) {
	fmt.Println("model_here_with_message", model)
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

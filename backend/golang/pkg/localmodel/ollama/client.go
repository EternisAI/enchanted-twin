package ollama

import (
	"context"
	"encoding/json"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"

	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

var _ ai.Completion = (*OllamaClient)(nil)

type OllamaClient struct {
	client *openai.Client
}

func NewOllamaClient(baseURL string) *OllamaClient {
	if baseURL == "" {
		baseURL = "http://localhost:11434/v1"
	}
	client := openai.NewClient(
		option.WithAPIKey(""),
		option.WithBaseURL(baseURL),
	)
	return &OllamaClient{client: &client}
}

func (c *OllamaClient) Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) (openai.ChatCompletionMessage, error) {
	completion, err := c.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: messages,
		Model:    model,
	})
	return completion.Choices[0].Message, err
}

func (c *OllamaClient) Anonymize(ctx context.Context, prompt string) (map[string]string, error) {
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage("you are an anonymizer, return only in JSON"),
		openai.UserMessage(prompt),
	}

	response, err := c.Completions(ctx, messages, nil, "")
	if err != nil {
		return nil, err
	}

	var result map[string]string
	if err := json.Unmarshal([]byte(response.Content), &result); err != nil {
		return nil, err
	}

	return result, nil
}

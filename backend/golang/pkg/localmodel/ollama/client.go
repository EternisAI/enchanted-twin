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
	model  string
}

func NewOllamaClient(baseURL string, model string) OllamaClient {
	client := openai.NewClient(
		option.WithAPIKey(""),
		option.WithBaseURL(baseURL),
	)
	return OllamaClient{client: &client, model: model}
}

func (c OllamaClient) Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) (openai.ChatCompletionMessage, error) {
	completion, err := c.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: messages,
		Model:    model,
	})
	if err != nil {
		return openai.ChatCompletionMessage{}, err
	}
	return completion.Choices[0].Message, err
}

func (c OllamaClient) Anonymize(ctx context.Context, prompt string) (map[string]string, error) {
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(`/no_think You are an anonymizer.
Return ONLY <json>{"orig": "replacement", …}</json>.
Example
user: "John Doe is a software engineer at Google"
assistant: <json>{"John Doe":"Dave Smith","Google":"TechCorp"}</json>
anonymize this:`),
		openai.UserMessage(prompt),
	}

	response, err := c.Completions(ctx, messages, nil, c.model)
	if err != nil {
		return nil, err
	}

	var result map[string]string
	if err := json.Unmarshal([]byte(response.Content), &result); err != nil {
		return nil, err
	}

	return result, nil
}

func (c OllamaClient) Close() error {
	return nil
}

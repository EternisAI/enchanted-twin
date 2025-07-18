package ollama

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"

	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

var _ ai.Completion = (*OllamaClient)(nil)

type OllamaClient struct {
	client *openai.Client
	model  string
	logger *log.Logger
}

func NewOllamaClient(baseURL string, model string, logger *log.Logger) *OllamaClient {
	client := openai.NewClient(
		option.WithAPIKey(""),
		option.WithBaseURL(baseURL),
	)
	return &OllamaClient{client: &client, model: model, logger: logger}
}

func prettifyConnectionError(err error) error {
	if err == nil {
		return nil
	}

	var netErr *net.OpError
	if errors.As(err, &netErr) {
		if netErr.Op == "dial" {
			return errors.New("anonymiser is not running")
		}
	}

	if strings.Contains(err.Error(), "connection refused") {
		return errors.New("anonymiser is not running")
	}

	return err
}

func (c *OllamaClient) Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) (openai.ChatCompletionMessage, error) {
	completion, err := c.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: messages,
		Model:    model,
	})
	if err != nil {
		return openai.ChatCompletionMessage{}, prettifyConnectionError(err)
	}
	return completion.Choices[0].Message, err
}

func (c *OllamaClient) deserializeAnonymizationResponse(content string) (map[string]string, error) {
	startIndex := strings.Index(content, "{")
	if startIndex == -1 {
		return nil, fmt.Errorf("no JSON object found in response")
	}

	endIndex := strings.LastIndex(content, "}")
	if endIndex == -1 || endIndex <= startIndex {
		return nil, fmt.Errorf("malformed JSON object in response")
	}

	jsonStr := strings.TrimSpace(content[startIndex : endIndex+1])
	jsonStr = strings.ReplaceAll(jsonStr, "'", "\"")

	var result map[string]string
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return result, nil
}

func (c *OllamaClient) Anonymize(ctx context.Context, prompt string) (map[string]string, error) {
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(`/no_think You are an anonymizer.
Return ONLY <json>{"orig": "replacement", …}</json>.
Example
user: "John Doe is a software engineer at Google"
assistant: <json>{"John Doe":"Dave Smith","Google":"TechCorp"}</json>
anonymize this:`),
		openai.UserMessage(prompt),
	}

	const maxRetries = 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		response, err := c.Completions(ctx, messages, nil, c.model)
		if err != nil {
			lastErr = prettifyConnectionError(err)
			c.logger.Warn("Anonymization completion failed", "attempt", attempt, "error", err)
			continue
		}

		c.logger.Info("Local anonymizer response", "attempt", attempt, "anonymizer", response.Content, "original", prompt)

		result, err := c.deserializeAnonymizationResponse(response.Content)
		if err != nil {
			lastErr = err
			c.logger.Warn("Anonymization deserialization failed", "attempt", attempt, "error", err)
			continue
		}
		c.logger.Info("Anonymization result", "result", result)

		return result, nil
	}

	return nil, fmt.Errorf("anonymization failed after %d attempts: %w", maxRetries, lastErr)
}

func (c *OllamaClient) Close() error {
	return nil
}

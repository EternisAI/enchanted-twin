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
	"github.com/openai/openai-go/packages/param"

	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

var _ ai.Completion = (*OllamaClient)(nil)

var replaceEntitiesTool = openai.ChatCompletionToolParam{
	Type: "function",
	Function: openai.FunctionDefinitionParam{
		Name: "replace_entities",
		Description: param.NewOpt(
			"Replace PII entities in the text with semantically equivalent alternatives that preserve context.",
		),
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"replacements": map[string]any{
					"type":        "array",
					"description": "List of replacements to make. Each item has the PII text and its anonymised version.",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"original": map[string]any{
								"type":        "string",
								"description": "PII text to replace",
							},
							"replacement": map[string]any{
								"type":        "string",
								"description": "Anonymised text",
							},
						},
						"required": []string{"original", "replacement"},
					},
				},
			},
			"required": []string{"replacements"},
		},
	},
}

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
		Tools:    tools,
		ToolChoice: openai.ChatCompletionToolChoiceOptionUnionParam{
			OfAuto: openai.String("required"),
		},
		Temperature: param.NewOpt(0.0),
		MaxTokens:   param.NewOpt(int64(256)),
	})
	if err != nil {
		return openai.ChatCompletionMessage{}, prettifyConnectionError(err)
	}
	return completion.Choices[0].Message, err
}

func (c *OllamaClient) Anonymize(ctx context.Context, prompt string) (map[string]string, error) {
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage("You are an anonymizer. Replace personally identifiable information (PII) entities with semantically equivalent alternatives that preserve the context needed for a good response. If no PII is found, return an empty replacements list."),
		openai.UserMessage(prompt),
	}

	response, err := c.Completions(ctx, messages, []openai.ChatCompletionToolParam{replaceEntitiesTool}, c.model)
	if err != nil {
		return nil, prettifyConnectionError(err)
	}

	c.logger.Info("Local anonymizer response", "original", prompt)

	if len(response.ToolCalls) == 0 {
		c.logger.Warn("Anonymization deserialization failed", "error", "no tool calls found in response")
		return nil, fmt.Errorf("no tool calls found in response")
	}

	toolCall := response.ToolCalls[0]
	c.logger.Info("Tool call", "toolCall", toolCall)
	c.logger.Debug("Tool arguments", "arguments", toolCall.Function.Arguments)

	var result struct {
		Replacements []struct {
			Original    string `json:"original"`
			Replacement string `json:"replacement"`
		} `json:"replacements"`
	}

	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &result); err != nil {
		c.logger.Warn("Anonymization deserialization failed", "error", err)
		return nil, fmt.Errorf("anonymization deserialization failed: %w", err)
	}

	c.logger.Debug("Parsed result", "replacements_count", len(result.Replacements))

	replacements := make(map[string]string)
	for _, replacement := range result.Replacements {
		replacements[replacement.Original] = replacement.Replacement
	}

	c.logger.Info("Anonymization result", "result", replacements)
	return replacements, nil
}

func (c *OllamaClient) Close() error {
	return nil
}

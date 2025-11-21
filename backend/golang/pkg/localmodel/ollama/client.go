package ollama

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/param"

	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

var _ ai.Completion = (*OllamaClient)(nil)

var replaceEntitiesTool = openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
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
})

type OllamaClient struct {
	client  *openai.Client
	model   string
	logger  *log.Logger
	baseURL string
}

func NewOllamaClient(baseURL string, model string, logger *log.Logger) *OllamaClient {
	// Ensure baseURL includes OpenAI-compatible /v1 path
	normalizedBaseURL := ensureV1Suffix(baseURL)
	logger.Info("Initializing local anonymizer client", "baseURL", normalizedBaseURL, "model", model)

	client := openai.NewClient(
		option.WithAPIKey(""),
		option.WithBaseURL(normalizedBaseURL),
	)
	return &OllamaClient{client: &client, model: model, logger: logger, baseURL: normalizedBaseURL}
}

func ensureV1Suffix(baseURL string) string {
	trimmed := strings.TrimRight(baseURL, "/")
	if strings.HasSuffix(trimmed, "/v1") {
		return trimmed
	}
	return trimmed + "/v1"
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

	if strings.Contains(strings.ToLower(err.Error()), "404") || strings.Contains(strings.ToLower(err.Error()), "not found") {
		return errors.New("anonymiser endpoint not found (did you include '/v1' in the base URL?)")
	}

	return err
}

func (c *OllamaClient) Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolUnionParam, model string) (openai.ChatCompletionMessage, error) {
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

// Ping performs a lightweight health check against the OpenAI-compatible server.
// It queries the models list endpoint and returns an error if the server is not reachable.
func (c *OllamaClient) Ping(ctx context.Context) error {
	endpoint := strings.TrimRight(c.baseURL, "/") + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return prettifyConnectionError(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("anonymiser health check failed: GET %s returned %d", endpoint, resp.StatusCode)
	}
	c.logger.Info("Anonymizer health check OK", "endpoint", endpoint)
	return nil
}

func (c *OllamaClient) Anonymize(ctx context.Context, prompt string) (map[string]string, error) {
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(`You are an anonymizer. Your task is to identify and replace personally identifiable information (PII) in the given text.
Replace PII entities with semantically equivalent alternatives that preserve the context needed for a good response.
If no PII is found or replacement is not needed, return an empty replacements list.

REPLACEMENT RULES:
• Personal names: Replace private or small-group individuals. Pick same culture + gender + era; keep surnames aligned across family members. DO NOT replace globally recognized public figures (heads of state, Nobel laureates, A-list entertainers, Fortune-500 CEOs, etc.).
• Companies / organizations: Replace private, niche, employer & partner orgs. Invent a fictitious org in the same industry & size tier; keep legal suffix. Keep major public companies (anonymity set ≥ 1,000,000).
• Projects / codenames / internal tools: Always replace with a neutral two-word alias of similar length.
• Locations: Replace street addresses, buildings, villages & towns < 100k pop with a same-level synthetic location inside the same state/country. Keep big cities (≥ 1M), states, provinces, countries, iconic landmarks.
• Dates & times: Replace birthdays, meeting invites, exact timestamps. Shift all dates in the prompt by one deterministic Δdays so ordering is preserved. DO NOT shift public holidays or famous historic dates ("July 4 1776", "Christmas Day", "9/11/2001", etc.). Keep years, fiscal quarters, decade references.
• Identifiers: (emails, phone #s, IDs, URLs, account #s) Always replace with format-valid dummies; keep domain class (.com big-tech, .edu, .gov).
• Monetary values: Replace personal income, invoices, bids by × [0.8 – 1.25] to keep order-of-magnitude. Keep public list prices & market caps.
• Quotes / text snippets: If the quote contains PII, swap only the embedded tokens; keep the rest verbatim.
/no_think`),
		openai.UserMessage(prompt + "\n/no_think"),
	}

	response, err := c.Completions(ctx, messages, []openai.ChatCompletionToolUnionParam{replaceEntitiesTool}, c.model)
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

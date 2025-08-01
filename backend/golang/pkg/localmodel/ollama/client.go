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

var userPrompt = `
[BEGIN OF TASK INSTRUCTION]
You are an anonymizer. Your task is to identify and replace personally identifiable information (PII) in the given text.
Replace PII entities with semantically equivalent alternatives that preserve the context needed for a good response.
If no PII is found or replacement is not needed, return an empty replacements list.

REPLACEMENT RULES:
• Personal names: Replace private or small-group individuals. Pick same culture + gender + era; keep surnames aligned across family members. DO NOT replace globally recognised public figures (heads of state, Nobel laureates, A-list entertainers, Fortune-500 CEOs, etc.).
• Companies / organisations: Replace private, niche, employer & partner orgs. Invent a fictitious org in the same industry & size tier; keep legal suffix. Keep major public companies (anonymity set ≥ 1,000,000).
• Projects / codenames / internal tools: Always replace with a neutral two-word alias of similar length.
• Locations: Replace street addresses, buildings, villages & towns < 100k pop with a same-level synthetic location inside the same state/country. Keep big cities (≥ 1M), states, provinces, countries, iconic landmarks.
• Dates & times: Replace birthdays, meeting invites, exact timestamps. Shift all dates in the prompt by one deterministic Δdays so ordering is preserved. DO NOT shift public holidays or famous historic dates ("July 4 1776", "Christmas Day", "9/11/2001", etc.). Keep years, fiscal quarters, decade references.
• Identifiers: (emails, phone #s, IDs, URLs, account #s) Always replace with format-valid dummies; keep domain class (.com big-tech, .edu, .gov).
• Monetary values: Replace personal income, invoices, bids by × [0.8 – 1.25] to keep order-of-magnitude. Keep public list prices & market caps.
• Quotes / text snippets: If the quote contains PII, swap only the embedded tokens; keep the rest verbatim.
""".strip()

FORMAT_INSTRUCTION = """
Use the replace_entities tool to specify replacements. Your response must use the tool call wrapper format:

<|tool_call|>{"name": "replace_entities", "arguments": {"replacements": [{"original": "PII_TEXT", "replacement": "ANONYMIZED_TEXT"}, ...]}}</|tool_call|>

If no replacements are needed, use:
<|tool_call|>{"name": "replace_entities", "arguments": {"replacements": []}}</|tool_call|>

Remember to wrap your entire tool call in <|tool_call|> and </|tool_call|> tags.
[END OF TASK INSTRUCTION]

[BEGIN OF AVAILABLE TOOLS]
[{\"name\": \"replace_entities\", \"description\": \"Replace PII entities in the text with semantically equivalent alternatives that preserve context.\", \"parameters\": {\"replacements\": {\"description\": \"List of replacements to make. Each replacement has an 'original' field with the PII text and a 'replacement' field with the anonymized version.\", \"type\": \"array\", \"items\": {\"type\": \"object\", \"properties\": {\"original\": {\"description\": \"The original PII text to replace\", \"type\": \"str\"}, \"replacement\": {\"description\": \"The anonymized replacement text\", \"type\": \"str\"}}, \"required\": [\"original\", \"replacement\"]}}}}]
[END OF AVAILABLE TOOLS]

[BEGIN OF FORMAT INSTRUCTION]
Use the replace_entities tool to specify replacements. Your response must use the tool call wrapper format:

<|tool_call|>{"name": "replace_entities", "arguments": {"replacements": [{"original": "PII_TEXT", "replacement": "ANONYMIZED_TEXT"}, ...]}}</|tool_call|>

If no replacements are needed, use:
<|tool_call|>{"name": "replace_entities", "arguments": {"replacements": []}}</|tool_call|>

Remember to wrap your entire tool call in <|tool_call|> and </|tool_call|> tags.
[END OF FORMAT INSTRUCTION]

[BEGIN OF QUERY]
{query}
[END OF QUERY]
`

func createUserPrompt(query string) string {
	return strings.ReplaceAll(userPrompt, "{query}", query)
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
	})
	if err != nil {
		return openai.ChatCompletionMessage{}, prettifyConnectionError(err)
	}
	return completion.Choices[0].Message, err
}

func decodeAnonymizationResponse(logger *log.Logger, content string) (map[string]string, error) {
	fullToolCall := `<|tool_call|>{"name":"replace_entities","arguments":{"replacements":[` + content

	logger.Info("Full tool call before parsing", "fullToolCall", fullToolCall)

	// Find the start tag
	startTag := "<|tool_call|>"
	endTag := "</|tool_call|>"

	startIdx := strings.Index(fullToolCall, startTag)
	if startIdx == -1 {
		return nil, fmt.Errorf("tool_call start tag not found")
	}

	endIdx := strings.Index(fullToolCall, endTag)
	if endIdx == -1 {
		return nil, fmt.Errorf("tool_call end tag not found")
	}

	jsonStr := fullToolCall[startIdx+len(startTag) : endIdx]

	logger.Info("Tool call", "jsonStr", jsonStr)

	var toolCall struct {
		Name      string `json:"name"`
		Arguments struct {
			Replacements []struct {
				Original    string `json:"original"`
				Replacement string `json:"replacement"`
			} `json:"replacements"`
		} `json:"arguments"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &toolCall); err != nil {
		return nil, fmt.Errorf("failed to parse tool call JSON: %w", err)
	}

	replacements := make(map[string]string)
	for _, replacement := range toolCall.Arguments.Replacements {
		replacements[replacement.Original] = replacement.Replacement
	}

	return replacements, nil
}

func (c *OllamaClient) Anonymize(ctx context.Context, prompt string) (map[string]string, error) {
	c.logger.Info("Anonymizing prompt", "prompt", prompt)
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage(createUserPrompt(prompt)),
		openai.AssistantMessage(`<think> Let me output exact tool call ONLY </think> <|tool_call|>{"name":"replace_entities","arguments":{"replacements":[`),
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

		c.logger.Info("Local anonymizer response", "response", response.Content)
		c.logger.Info("Local anonymizer response", "attempt", attempt, "original", prompt)

		replacements, err := decodeAnonymizationResponse(c.logger, response.Content)
		if err != nil {
			lastErr = fmt.Errorf("failed to decode anonymization response: %w", err)
			c.logger.Warn("Failed to decode anonymization response", "attempt", attempt, "error", err)
			continue
		}

		c.logger.Debug("Parsed result", "replacements_count", len(replacements))
		c.logger.Info("Anonymization result", "result", replacements)
		return replacements, nil
	}

	return nil, fmt.Errorf("anonymization failed after %d attempts: %w", maxRetries, lastErr)
}

func (c *OllamaClient) Close() error {
	return nil
}
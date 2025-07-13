package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
)

type EntityReplacement struct {
	Original    string `json:"original"`
	Replacement string `json:"replacement"`
}

type AnonymizationResponse struct {
	Replacements []EntityReplacement `json:"replacements"`
}

type LLMAnonymizer struct {
	aiService         CompletionsService
	model             string
	logger            *log.Logger
	mu                sync.RWMutex
	conversationDicts map[string]map[string]string
	hasher            MessageHasher
}

var anonymizationTool = openai.ChatCompletionToolParam{
	Type: "function",
	Function: openai.FunctionDefinitionParam{
		Name:        "replace_entities",
		Description: param.NewOpt("Replace personally identifiable information (PII) entities in text with anonymized values while preserving semantic meaning and context."),
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"replacements": map[string]any{
					"type":        "array",
					"description": "List of entity replacements to apply",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"original": map[string]any{
								"type":        "string",
								"description": "The original text to be replaced",
							},
							"replacement": map[string]any{
								"type":        "string",
								"description": "The anonymized replacement text",
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

func NewLLMAnonymizer(aiService CompletionsService, model string, logger *log.Logger) *LLMAnonymizer {
	return &LLMAnonymizer{
		aiService:         aiService,
		model:             model,
		logger:            logger,
		conversationDicts: make(map[string]map[string]string),
		hasher:            *NewMessageHasher(),
	}
}

func (l *LLMAnonymizer) AnonymizeMessages(ctx context.Context, conversationID string, messages []openai.ChatCompletionMessageParamUnion, existingDict map[string]string, interruptChan <-chan struct{}) ([]openai.ChatCompletionMessageParamUnion, map[string]string, map[string]string, error) {
	select {
	case <-ctx.Done():
		return nil, nil, nil, ctx.Err()
	case <-interruptChan:
		return nil, nil, nil, fmt.Errorf("anonymization interrupted")
	default:
	}

	// Extract text content from messages
	var textContents []string
	for _, msg := range messages {
		content := l.extractMessageContent(msg)
		if content != "" {
			textContents = append(textContents, content)
		}
	}

	if len(textContents) == 0 {
		return messages, existingDict, make(map[string]string), nil
	}

	// Create prompt for anonymization
	combinedText := strings.Join(textContents, "\n\n")

	existingDictJSON, _ := json.Marshal(existingDict)

	systemPrompt := fmt.Sprintf(`You are a privacy-focused AI assistant. Your task is to identify personally identifiable information (PII) in the provided text and replace it with anonymized values.

Guidelines:
1. Identify PII including: names, locations, organizations, email addresses, phone numbers, account numbers, addresses, dates of birth, and other sensitive data
2. Create placeholder tokens following these patterns:
   - People: PERSON_001, PERSON_002, etc.
   - Locations: LOCATION_001, LOCATION_002, etc.
   - Organizations: COMPANY_001, COMPANY_002, etc.
   - Email addresses: EMAIL_001, EMAIL_002, etc.
   - Phone numbers: PHONE_001, PHONE_002, etc.
   - Other sensitive data: SENSITIVE_001, SENSITIVE_002, etc.
3. Preserve existing mappings from the current dictionary: %s
4. Only create new tokens for NEW information not already in the existing dictionary
5. Be conservative - when in doubt, anonymize

Use the replace_entities tool to provide your response.`, string(existingDictJSON))

	userPrompt := fmt.Sprintf("Please analyze the following text and create an anonymization dictionary:\n\n%s", combinedText)

	llmMessages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
		openai.UserMessage(userPrompt),
	}

	// Call AI service with tool calling
	// Use RawCompletions if available to avoid circular dependency in private completions
	var result PrivateCompletionResult
	var err error

	if service, ok := l.aiService.(*Service); ok {
		// Use RawCompletions to bypass private completions and avoid circular dependency
		result, err = service.RawCompletions(ctx, llmMessages, []openai.ChatCompletionToolParam{anonymizationTool}, l.model)
	} else {
		// Fallback to regular completions for other implementations
		result, err = l.aiService.Completions(ctx, llmMessages, []openai.ChatCompletionToolParam{anonymizationTool}, l.model, Background)
	}

	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get anonymization dictionary: %w", err)
	}

	response := result.Message
	l.logger.Debug("LLM anonymization response", "response", response)

	if len(response.ToolCalls) == 0 {
		l.logger.Warn("No tool calls returned from LLM, using existing dictionary")
		return l.applyAnonymization(messages, existingDict), existingDict, make(map[string]string), nil
	}

	// Parse tool call response
	toolCall := response.ToolCalls[0]
	if toolCall.Function.Name != "replace_entities" {
		return nil, nil, nil, fmt.Errorf("unexpected tool call: %s", toolCall.Function.Name)
	}

	var anonymizationResult AnonymizationResponse
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &anonymizationResult); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to parse anonymization response: %w", err)
	}

	// Merge with existing dictionary
	updatedDict := make(map[string]string)
	newRules := make(map[string]string)

	// Copy existing dictionary
	for k, v := range existingDict {
		updatedDict[k] = v
	}

	// Add new rules from replacements
	for _, replacement := range anonymizationResult.Replacements {
		if _, exists := existingDict[replacement.Replacement]; !exists {
			newRules[replacement.Replacement] = replacement.Original
		}
		updatedDict[replacement.Replacement] = replacement.Original
	}

	// Apply anonymization to messages
	anonymizedMessages := l.applyAnonymization(messages, updatedDict)

	return anonymizedMessages, updatedDict, newRules, nil
}

func (l *LLMAnonymizer) extractMessageContent(message openai.ChatCompletionMessageParamUnion) string {
	messageBytes, err := json.Marshal(message)
	if err != nil {
		return ""
	}

	var messageMap map[string]interface{}
	if err := json.Unmarshal(messageBytes, &messageMap); err != nil {
		return ""
	}

	if content, exists := messageMap["content"]; exists {
		if contentStr, ok := content.(string); ok {
			return contentStr
		}
	}

	return ""
}

func (l *LLMAnonymizer) applyAnonymization(messages []openai.ChatCompletionMessageParamUnion, rules map[string]string) []openai.ChatCompletionMessageParamUnion {
	anonymizedMessages := make([]openai.ChatCompletionMessageParamUnion, len(messages))

	for i, message := range messages {
		anonymizedMsg, err := l.anonymizeMessage(message, rules)
		if err != nil {
			l.logger.Error("Failed to anonymize message", "error", err)
			anonymizedMessages[i] = message // Keep original on error
		} else {
			anonymizedMessages[i] = anonymizedMsg
		}
	}

	return anonymizedMessages
}

func (l *LLMAnonymizer) anonymizeMessage(message openai.ChatCompletionMessageParamUnion, rules map[string]string) (openai.ChatCompletionMessageParamUnion, error) {
	messageBytes, err := json.Marshal(message)
	if err != nil {
		return message, fmt.Errorf("failed to marshal message: %w", err)
	}

	var messageMap map[string]interface{}
	if err := json.Unmarshal(messageBytes, &messageMap); err != nil {
		return message, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	// Anonymize content field if it exists
	if content, exists := messageMap["content"]; exists {
		if contentStr, ok := content.(string); ok {
			anonymizedContent := l.anonymizeText(contentStr, rules)
			messageMap["content"] = anonymizedContent
		}
	}

	// Convert back to original message type
	anonymizedBytes, err := json.Marshal(messageMap)
	if err != nil {
		return message, fmt.Errorf("failed to marshal anonymized message: %w", err)
	}

	var anonymizedMessage openai.ChatCompletionMessageParamUnion
	if err := json.Unmarshal(anonymizedBytes, &anonymizedMessage); err != nil {
		return message, fmt.Errorf("failed to unmarshal anonymized message: %w", err)
	}

	return anonymizedMessage, nil
}

func (l *LLMAnonymizer) anonymizeText(text string, rules map[string]string) string {
	anonymized := text

	// Create sorted list of replacements by original length (longest first)
	type replacement struct {
		token    string
		original string
	}

	var replacements []replacement
	for token, original := range rules {
		replacements = append(replacements, replacement{token: token, original: original})
	}

	// Sort by original length descending (longest first)
	sort.Slice(replacements, func(i, j int) bool {
		return len(replacements[i].original) > len(replacements[j].original)
	})

	// Apply replacements (original -> token)
	for _, repl := range replacements {
		anonymized = strings.ReplaceAll(anonymized, repl.original, repl.token)
	}

	return anonymized
}

func (l *LLMAnonymizer) DeAnonymize(anonymized string, rules map[string]string) string {
	restored := anonymized

	// Create sorted list of tokens by length (longest first)
	type tokenReplacement struct {
		token    string
		original string
	}

	var tokens []tokenReplacement
	for token, original := range rules {
		tokens = append(tokens, tokenReplacement{token: token, original: original})
	}

	// Sort by token length descending (longest first)
	sort.Slice(tokens, func(i, j int) bool {
		return len(tokens[i].token) > len(tokens[j].token)
	})

	// Apply rules in reverse (token -> original)
	for _, tokenRepl := range tokens {
		restored = strings.ReplaceAll(restored, tokenRepl.token, tokenRepl.original)
	}

	return restored
}

func (l *LLMAnonymizer) LoadConversationDict(conversationID string) (map[string]string, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if dict, exists := l.conversationDicts[conversationID]; exists {
		// Return a copy to prevent external modification
		result := make(map[string]string)
		for k, v := range dict {
			result[k] = v
		}
		return result, nil
	}

	return make(map[string]string), nil
}

func (l *LLMAnonymizer) SaveConversationDict(conversationID string, dict map[string]string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Store a copy to prevent external modification
	dictCopy := make(map[string]string)
	for k, v := range dict {
		dictCopy[k] = v
	}

	l.conversationDicts[conversationID] = dictCopy
	return nil
}

func (l *LLMAnonymizer) GetMessageHash(message openai.ChatCompletionMessageParamUnion) string {
	return l.hasher.GetMessageHash(message)
}

func (l *LLMAnonymizer) IsMessageAnonymized(conversationID, messageHash string) (bool, error) {
	// LLM anonymizer doesn't track individual message anonymization status
	// This would require additional persistence layer
	return false, nil
}

func (l *LLMAnonymizer) Shutdown() {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Clear conversation dictionaries
	l.conversationDicts = make(map[string]map[string]string)
}

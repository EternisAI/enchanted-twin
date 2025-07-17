package ai

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

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
	aiService CompletionsService
	model     string
	logger    *log.Logger
	store     ConversationStore
	hasher    *MessageHasher
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

func NewLLMAnonymizer(aiService CompletionsService, model string, db *sql.DB, logger *log.Logger) *LLMAnonymizer {
	return &LLMAnonymizer{
		aiService: aiService,
		model:     model,
		logger:    logger,
		store:     NewSQLiteConversationStore(db, logger),
		hasher:    NewMessageHasher(),
	}
}

func (l *LLMAnonymizer) AnonymizeMessages(ctx context.Context, conversationID string, messages []openai.ChatCompletionMessageParamUnion, existingDict map[string]string, interruptChan <-chan struct{}) ([]openai.ChatCompletionMessageParamUnion, map[string]string, map[string]string, error) {
	if conversationID == "" {
		// Memory-only mode
		return l.anonymizeInMemory(ctx, messages, existingDict, interruptChan)
	}

	// Persistent mode
	return l.anonymizeWithPersistence(ctx, conversationID, messages, existingDict, interruptChan)
}

func (l *LLMAnonymizer) anonymizeInMemory(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, existingDict map[string]string, interruptChan <-chan struct{}) ([]openai.ChatCompletionMessageParamUnion, map[string]string, map[string]string, error) {
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

	systemPrompt := `You are an anonymizer. Return data in the following format:
Example
user: "John Doe is a software engineer at Google"
assistant: <json>{"John Doe":"Dave Smith","Google":"TechCorp"}</json>

----------------  REPLACEMENT RULES  ----------------
- Goal After deanonymising, final answer must equal answer on original text.

ENTITY-CLASS
- Personal names Replace private / small-group.  Choose same culture+gender+era; share surnames if originals do.  Keep public figures.
- Companies/orgs Replace private, niche, employer, partners.  Fake org same industry & size, keep legal suffix.  Keep majors (anon-set ≥ 1 M).
- Projects / codenames Always replace with neutral two-word alias.
- Locations Replace addresses/buildings/towns < 100 k pop with same-level synthetic in same state/country.  Keep big cities, states, countries.
- Dates/times Replace birthdays, invites, exact timestamps.  Shift all mentioned dates by same Δdays; preserve order & granularity.  Keep years, quarters, decades.
- Identifiers (email, phone, ID, URL) Always replace with format-valid dummy; keep domain class.
- Money Replace personal amounts, invoices, bids by ×[0.8–1.25].  Keep public list prices & market caps.
- Quotes If quote embeds PII, swap only those tokens; else keep.
- DO NOT REPLACE POPULAR PEOPLE NAMES

PRACTICAL EDGE CASES
– Nicknames → same-length short name.
– Honorifics kept unless identifying.
– Preserve script (Kanji→Kanji etc.).
– Handles with digits keep digit pattern.
– Chained: "John at Google in Mountain View" → "Lena at TechCorp in Mountain View".
– Ambiguous? KEEP (precision > recall).
– Maintain original specificity: coarse = keep, too-fine = replace with diff element of same coarse class.

WHY KEEP BIG CITIES Pop ≥ 1 M already gives anonymity; replacing hurts context.

IMPORTANT Attackers may join many anonymized queries—choose replacements deterministically for same token across session.

Use the replace_entities tool to provide your response. The content you're anonymising below is`

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

func (l *LLMAnonymizer) anonymizeWithPersistence(ctx context.Context, conversationID string, messages []openai.ChatCompletionMessageParamUnion, existingDict map[string]string, interruptChan <-chan struct{}) ([]openai.ChatCompletionMessageParamUnion, map[string]string, map[string]string, error) {
	// Load existing conversation dictionary
	conversationDict, err := l.store.GetConversationDict(conversationID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load conversation dict: %w", err)
	}

	// Merge with provided existing dictionary (provided dict takes precedence)
	workingDict := make(map[string]string)
	for k, v := range conversationDict {
		workingDict[k] = v
	}
	for k, v := range existingDict {
		workingDict[k] = v
	}

	// Identify new messages that need anonymization
	newMessages := make([]openai.ChatCompletionMessageParamUnion, 0)
	messageMap := make(map[string]openai.ChatCompletionMessageParamUnion)

	for _, message := range messages {
		messageHash := l.hasher.GetMessageHash(message)
		messageMap[messageHash] = message

		isAnonymized, err := l.store.IsMessageAnonymized(conversationID, messageHash)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to check message anonymization status: %w", err)
		}

		if !isAnonymized {
			newMessages = append(newMessages, message)
		}
	}

	l.logger.Debug("Persistent LLM anonymization analysis", "conversationID", conversationID, "totalMessages", len(messages), "newMessages", len(newMessages))

	// Process new messages only if there are any
	newRules := make(map[string]string)
	if len(newMessages) > 0 {
		// Check for context cancellation and interruption
		select {
		case <-ctx.Done():
			return nil, nil, nil, ctx.Err()
		case <-interruptChan:
			return nil, nil, nil, fmt.Errorf("anonymization interrupted")
		default:
		}

		// Use in-memory processing for new messages with working dictionary
		_, tempDict, msgRules, err := l.anonymizeInMemory(ctx, newMessages, workingDict, interruptChan)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to anonymize new messages: %w", err)
		}

		// Merge new rules and update working dictionary
		for token, original := range msgRules {
			newRules[token] = original
			workingDict[token] = original
		}

		// Update working dictionary with any new discoveries from temp processing
		for k, v := range tempDict {
			if _, exists := workingDict[k]; !exists {
				workingDict[k] = v
			}
		}

		// Mark new messages as anonymized
		for _, message := range newMessages {
			messageHash := l.hasher.GetMessageHash(message)
			if err := l.store.MarkMessageAnonymized(conversationID, messageHash); err != nil {
				return nil, nil, nil, fmt.Errorf("failed to mark message as anonymized: %w", err)
			}
		}
	}

	// Save updated dictionary
	if err := l.store.SaveConversationDict(conversationID, workingDict); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to save conversation dict: %w", err)
	}

	// Reconstruct full anonymized message list
	anonymizedMessages := make([]openai.ChatCompletionMessageParamUnion, len(messages))
	for i, message := range messages {
		anonymizedMsg, err := l.anonymizeMessage(message, workingDict)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to anonymize message %d: %w", i, err)
		}
		anonymizedMessages[i] = anonymizedMsg
	}

	l.logger.Debug("Persistent LLM anonymization complete", "conversationID", conversationID, "totalMessages", len(messages), "newRulesCount", len(newRules))
	return anonymizedMessages, workingDict, newRules, nil
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
	// Build original -> token mapping from token -> original rules
	originalToToken := make(map[string]string)
	for token, original := range rules {
		originalToToken[original] = token
	}

	// Use anonymization replacement (preserving token case)
	return ApplyAnonymization(text, originalToToken)
}

func (l *LLMAnonymizer) DeAnonymize(anonymized string, rules map[string]string) string {
	// Use simple de-anonymization (restore original case)
	return ApplyDeAnonymization(anonymized, rules)
}

func (l *LLMAnonymizer) LoadConversationDict(conversationID string) (map[string]string, error) {
	return l.store.GetConversationDict(conversationID)
}

func (l *LLMAnonymizer) SaveConversationDict(conversationID string, dict map[string]string) error {
	return l.store.SaveConversationDict(conversationID, dict)
}

func (l *LLMAnonymizer) GetMessageHash(message openai.ChatCompletionMessageParamUnion) string {
	return l.hasher.GetMessageHash(message)
}

func (l *LLMAnonymizer) IsMessageAnonymized(conversationID, messageHash string) (bool, error) {
	return l.store.IsMessageAnonymized(conversationID, messageHash)
}

func (l *LLMAnonymizer) Shutdown() {
	if l.store != nil {
		_ = l.store.Close()
	}
}

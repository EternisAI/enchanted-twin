package ai

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go"
)

type PersistentAnonymizer struct {
	store        ConversationStore
	hasher       *MessageHasher
	logger       *log.Logger
	tokenCounter map[string]int // Track token counters per conversation
}

func NewPersistentAnonymizer(db *sql.DB, logger *log.Logger) *PersistentAnonymizer {
	return &PersistentAnonymizer{
		store:        NewSQLiteConversationStore(db, logger),
		hasher:       NewMessageHasher(),
		logger:       logger,
		tokenCounter: make(map[string]int),
	}
}

func (p *PersistentAnonymizer) AnonymizeMessages(ctx context.Context, conversationID string, messages []openai.ChatCompletionMessageParamUnion, existingDict map[string]string, interruptChan <-chan struct{}) ([]openai.ChatCompletionMessageParamUnion, map[string]string, map[string]string, error) {
	if conversationID == "" {
		// Memory-only mode
		return p.anonymizeInMemory(ctx, messages, existingDict, interruptChan)
	}

	// Persistent mode
	return p.anonymizeWithPersistence(ctx, conversationID, messages, existingDict, interruptChan)
}

func (p *PersistentAnonymizer) anonymizeInMemory(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, existingDict map[string]string, interruptChan <-chan struct{}) ([]openai.ChatCompletionMessageParamUnion, map[string]string, map[string]string, error) {
	// Start with existing dictionary or empty map
	workingDict := make(map[string]string)
	for k, v := range existingDict {
		workingDict[k] = v
	}

	// Process all messages (no persistence checks)
	anonymizedMessages := make([]openai.ChatCompletionMessageParamUnion, len(messages))
	newRules := make(map[string]string)

	for i, message := range messages {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return nil, nil, nil, ctx.Err()
		case <-interruptChan:
			return nil, nil, nil, fmt.Errorf("anonymization interrupted")
		default:
		}

		anonymizedMsg, msgRules, err := p.anonymizeMessage(ctx, message, workingDict)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to anonymize message %d: %w", i, err)
		}

		anonymizedMessages[i] = anonymizedMsg

		// Merge new rules
		for token, original := range msgRules {
			newRules[token] = original
			workingDict[token] = original
		}
	}

	p.logger.Debug("Memory-only anonymization complete", "messageCount", len(messages), "newRulesCount", len(newRules))
	return anonymizedMessages, workingDict, newRules, nil
}

func (p *PersistentAnonymizer) anonymizeWithPersistence(ctx context.Context, conversationID string, messages []openai.ChatCompletionMessageParamUnion, existingDict map[string]string, interruptChan <-chan struct{}) ([]openai.ChatCompletionMessageParamUnion, map[string]string, map[string]string, error) {
	// Load existing conversation dictionary
	conversationDict, err := p.store.GetConversationDict(conversationID)
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
		messageHash := p.hasher.GetMessageHash(message)
		messageMap[messageHash] = message

		isAnonymized, err := p.store.IsMessageAnonymized(conversationID, messageHash)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to check message anonymization status: %w", err)
		}

		if !isAnonymized {
			newMessages = append(newMessages, message)
		}
	}

	p.logger.Debug("Persistent anonymization analysis", "conversationID", conversationID, "totalMessages", len(messages), "newMessages", len(newMessages))

	// Process new messages only
	newRules := make(map[string]string)
	for _, message := range newMessages {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return nil, nil, nil, ctx.Err()
		case <-interruptChan:
			return nil, nil, nil, fmt.Errorf("anonymization interrupted")
		default:
		}

		_, msgRules, err := p.anonymizeMessage(ctx, message, workingDict)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to anonymize new message: %w", err)
		}

		// Merge new rules
		for token, original := range msgRules {
			newRules[token] = original
			workingDict[token] = original
		}

		// Mark message as anonymized
		messageHash := p.hasher.GetMessageHash(message)
		if err := p.store.MarkMessageAnonymized(conversationID, messageHash); err != nil {
			return nil, nil, nil, fmt.Errorf("failed to mark message as anonymized: %w", err)
		}
	}

	// Save updated dictionary
	if err := p.store.SaveConversationDict(conversationID, workingDict); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to save conversation dict: %w", err)
	}

	// Reconstruct full anonymized message list
	anonymizedMessages := make([]openai.ChatCompletionMessageParamUnion, len(messages))
	for i, message := range messages {
		anonymizedMsg, _, err := p.anonymizeMessage(ctx, message, workingDict)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to anonymize message %d: %w", i, err)
		}
		anonymizedMessages[i] = anonymizedMsg
	}

	p.logger.Debug("Persistent anonymization complete", "conversationID", conversationID, "totalMessages", len(messages), "newRulesCount", len(newRules))
	return anonymizedMessages, workingDict, newRules, nil
}

func (p *PersistentAnonymizer) anonymizeMessage(ctx context.Context, message openai.ChatCompletionMessageParamUnion, workingDict map[string]string) (openai.ChatCompletionMessageParamUnion, map[string]string, error) {
	// Convert message to JSON to extract content
	messageBytes, err := json.Marshal(message)
	if err != nil {
		return message, nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	var messageMap map[string]interface{}
	if err := json.Unmarshal(messageBytes, &messageMap); err != nil {
		return message, nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	rules := make(map[string]string)

	// Anonymize content field if it exists
	if content, exists := messageMap["content"]; exists {
		if contentStr, ok := content.(string); ok {
			anonymizedContent, contentRules, err := p.anonymizeContent(ctx, contentStr, workingDict)
			if err != nil {
				return message, nil, fmt.Errorf("failed to anonymize content: %w", err)
			}

			messageMap["content"] = anonymizedContent

			// Merge content rules
			for token, original := range contentRules {
				rules[token] = original
				workingDict[token] = original
			}
		}
	}

	// Convert back to original message type
	anonymizedBytes, err := json.Marshal(messageMap)
	if err != nil {
		return message, nil, fmt.Errorf("failed to marshal anonymized message: %w", err)
	}

	var anonymizedMessage openai.ChatCompletionMessageParamUnion
	if err := json.Unmarshal(anonymizedBytes, &anonymizedMessage); err != nil {
		return message, nil, fmt.Errorf("failed to unmarshal anonymized message: %w", err)
	}

	return anonymizedMessage, rules, nil
}

func (p *PersistentAnonymizer) anonymizeContent(ctx context.Context, content string, workingDict map[string]string) (string, map[string]string, error) {
	// Use the existing trie-based replacement for efficient anonymization
	trie := NewReplacementTrie()

	// Build replacement map from working dictionary (reverse mapping)
	replacements := make(map[string]string)
	for token, original := range workingDict {
		replacements[original] = token
	}

	// Add predefined replacements that aren't in working dictionary
	predefinedReplacements := p.getPredefinedReplacements()
	for original, token := range predefinedReplacements {
		if _, exists := replacements[original]; !exists {
			replacements[original] = token
		}
	}

	// Insert all replacements into trie
	for original, token := range replacements {
		trie.Insert(original, token)
	}

	// Apply replacements
	anonymized, appliedRules := trie.ReplaceAll(content)

	// Return only new rules (not already in working dictionary)
	newRules := make(map[string]string)
	for token, original := range appliedRules {
		if _, exists := workingDict[token]; !exists {
			newRules[token] = original
		}
	}

	return anonymized, newRules, nil
}

func (p *PersistentAnonymizer) getPredefinedReplacements() map[string]string {
	// Use same predefined replacements as mock anonymizer
	return map[string]string{
		// Common names
		"John":    "PERSON_001",
		"Jane":    "PERSON_002",
		"Alice":   "PERSON_003",
		"Bob":     "PERSON_004",
		"Charlie": "PERSON_005",
		"David":   "PERSON_006",
		"Emma":    "PERSON_007",
		"Frank":   "PERSON_008",

		// Company names
		"OpenAI":    "COMPANY_001",
		"Microsoft": "COMPANY_002",
		"Google":    "COMPANY_003",
		"Apple":     "COMPANY_004",
		"Tesla":     "COMPANY_005",
		"Amazon":    "COMPANY_006",

		// Locations
		"New York":      "LOCATION_001",
		"London":        "LOCATION_002",
		"Tokyo":         "LOCATION_003",
		"Paris":         "LOCATION_004",
		"Berlin":        "LOCATION_005",
		"San Francisco": "LOCATION_006",
	}
}

func (p *PersistentAnonymizer) DeAnonymize(anonymized string, rules map[string]string) string {
	// Use the same de-anonymization logic as mock anonymizer
	restored := anonymized

	// Create a sorted list of tokens by length (longest first)
	type tokenReplacement struct {
		token    string
		original string
	}

	var sortedTokens []tokenReplacement
	for token, original := range rules {
		sortedTokens = append(sortedTokens, tokenReplacement{token: token, original: original})
	}

	// Sort by token length descending (longest first)
	sort.Slice(sortedTokens, func(i, j int) bool {
		return len(sortedTokens[i].token) > len(sortedTokens[j].token)
	})

	// Apply rules in reverse (anonymized token -> original) with longest tokens first
	for _, tokenRepl := range sortedTokens {
		restored = strings.ReplaceAll(restored, tokenRepl.token, tokenRepl.original)
	}

	return restored
}

func (p *PersistentAnonymizer) LoadConversationDict(conversationID string) (map[string]string, error) {
	return p.store.GetConversationDict(conversationID)
}

func (p *PersistentAnonymizer) SaveConversationDict(conversationID string, dict map[string]string) error {
	return p.store.SaveConversationDict(conversationID, dict)
}

func (p *PersistentAnonymizer) GetMessageHash(message openai.ChatCompletionMessageParamUnion) string {
	return p.hasher.GetMessageHash(message)
}

func (p *PersistentAnonymizer) IsMessageAnonymized(conversationID, messageHash string) (bool, error) {
	return p.store.IsMessageAnonymized(conversationID, messageHash)
}

func (p *PersistentAnonymizer) Shutdown() {
	if p.store != nil {
		_ = p.store.Close()
	}
}

func (p *PersistentAnonymizer) DeleteConversation(conversationID string) error {
	return p.store.DeleteConversation(conversationID)
}

func (p *PersistentAnonymizer) ListConversations() ([]string, error) {
	return p.store.ListConversations()
}

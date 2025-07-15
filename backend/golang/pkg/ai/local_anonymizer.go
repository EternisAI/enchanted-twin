package ai

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go"
)

// LlamaAnonymizerInterface defines the interface for Llama-based anonymization.
type LlamaAnonymizerInterface interface {
	Anonymize(ctx context.Context, text string) (map[string]string, error)
	Close() error
}

// LocalAnonymizer is an adapter that wraps LlamaAnonymizerInterface to implement the Anonymizer interface.
type LocalAnonymizer struct {
	llama  LlamaAnonymizerInterface
	store  ConversationStore
	hasher *MessageHasher
	logger *log.Logger
}

func NewLocalAnonymizer(llama LlamaAnonymizerInterface, db *sql.DB, logger *log.Logger) *LocalAnonymizer {
	return &LocalAnonymizer{
		llama:  llama,
		store:  NewSQLiteConversationStore(db, logger),
		hasher: NewMessageHasher(),
		logger: logger,
	}
}

func (l *LocalAnonymizer) AnonymizeMessages(ctx context.Context, conversationID string, messages []openai.ChatCompletionMessageParamUnion, existingDict map[string]string, interruptChan <-chan struct{}) ([]openai.ChatCompletionMessageParamUnion, map[string]string, map[string]string, error) {
	if conversationID == "" {
		// Memory-only mode
		return l.anonymizeInMemory(ctx, messages, existingDict, interruptChan)
	}

	// Persistent mode
	return l.anonymizeWithPersistence(ctx, conversationID, messages, existingDict, interruptChan)
}

func (l *LocalAnonymizer) anonymizeInMemory(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, existingDict map[string]string, interruptChan <-chan struct{}) ([]openai.ChatCompletionMessageParamUnion, map[string]string, map[string]string, error) {
	// Start with existing dictionary
	workingDict := make(map[string]string)
	for k, v := range existingDict {
		workingDict[k] = v
	}

	// Process all messages
	anonymizedMessages := make([]openai.ChatCompletionMessageParamUnion, len(messages))
	newRules := make(map[string]string)

	for i, message := range messages {
		// Check for interruption
		select {
		case <-ctx.Done():
			return nil, nil, nil, ctx.Err()
		case <-interruptChan:
			return nil, nil, nil, fmt.Errorf("anonymization interrupted")
		default:
		}

		// Get message content
		content := l.extractMessageContent(message)
		if content == "" {
			anonymizedMessages[i] = message
			continue
		}

		// Use local LLM to find new names/entities
		nameReplacements, err := l.llama.Anonymize(ctx, content)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to anonymize message content: %w", err)
		}

		// Update dictionaries with new discoveries
		for original, replacement := range nameReplacements {
			if original != "" && replacement != "" && original != replacement {
				// Only add if not already in dictionary
				if _, exists := workingDict[original]; !exists {
					workingDict[original] = replacement
					newRules[original] = replacement
				}
			}
		}

		// Apply all known replacements to the message
		anonymizedContent := l.applyReplacements(content, workingDict)
		anonymizedMessages[i] = l.replaceMessageContent(message, anonymizedContent)
	}

	l.logger.Debug("Memory-only local anonymization complete", "messageCount", len(messages), "newRulesCount", len(newRules))
	return anonymizedMessages, workingDict, newRules, nil
}

func (l *LocalAnonymizer) anonymizeWithPersistence(ctx context.Context, conversationID string, messages []openai.ChatCompletionMessageParamUnion, existingDict map[string]string, interruptChan <-chan struct{}) ([]openai.ChatCompletionMessageParamUnion, map[string]string, map[string]string, error) {
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

	l.logger.Debug("Persistent local anonymization analysis", "conversationID", conversationID, "totalMessages", len(messages), "newMessages", len(newMessages))

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
		for originalValue, replacement := range msgRules {
			newRules[originalValue] = replacement
			workingDict[originalValue] = replacement
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
		content := l.extractMessageContent(message)
		if content == "" {
			anonymizedMessages[i] = message
			continue
		}

		// Apply all known replacements to the message
		anonymizedContent := l.applyReplacements(content, workingDict)
		anonymizedMessages[i] = l.replaceMessageContent(message, anonymizedContent)
	}

	l.logger.Debug("Persistent local anonymization complete", "conversationID", conversationID, "totalMessages", len(messages), "newRulesCount", len(newRules))
	return anonymizedMessages, workingDict, newRules, nil
}

// DeAnonymize implements the Anonymizer interface.
func (l *LocalAnonymizer) DeAnonymize(anonymized string, rules map[string]string) string {
	// Create reverse mapping
	reverseRules := make(map[string]string)
	for original, replacement := range rules {
		reverseRules[replacement] = original
	}

	return l.applyReplacements(anonymized, reverseRules)
}

// LoadConversationDict implements the Anonymizer interface.
func (l *LocalAnonymizer) LoadConversationDict(conversationID string) (map[string]string, error) {
	return l.store.GetConversationDict(conversationID)
}

// SaveConversationDict implements the Anonymizer interface.
func (l *LocalAnonymizer) SaveConversationDict(conversationID string, dict map[string]string) error {
	return l.store.SaveConversationDict(conversationID, dict)
}

// GetMessageHash implements the Anonymizer interface.
func (l *LocalAnonymizer) GetMessageHash(message openai.ChatCompletionMessageParamUnion) string {
	return l.hasher.GetMessageHash(message)
}

// IsMessageAnonymized implements the Anonymizer interface.
func (l *LocalAnonymizer) IsMessageAnonymized(conversationID, messageHash string) (bool, error) {
	return l.store.IsMessageAnonymized(conversationID, messageHash)
}

// Shutdown implements the Anonymizer interface.
func (l *LocalAnonymizer) Shutdown() {
	if l.llama != nil {
		_ = l.llama.Close()
	}
	if l.store != nil {
		_ = l.store.Close()
	}
}

// Helper methods

func (l *LocalAnonymizer) extractMessageContent(message openai.ChatCompletionMessageParamUnion) string {
	// Use JSON marshaling to handle complex union types
	messageBytes, err := json.Marshal(message)
	if err != nil {
		return ""
	}

	var messageMap map[string]interface{}
	if err := json.Unmarshal(messageBytes, &messageMap); err != nil {
		return ""
	}

	// Extract content field if it exists
	if content, exists := messageMap["content"]; exists {
		if contentStr, ok := content.(string); ok {
			return contentStr
		}
	}

	return ""
}

func (l *LocalAnonymizer) replaceMessageContent(message openai.ChatCompletionMessageParamUnion, newContent string) openai.ChatCompletionMessageParamUnion {
	// Use JSON marshaling to handle complex union types
	messageBytes, err := json.Marshal(message)
	if err != nil {
		return message
	}

	var messageMap map[string]interface{}
	if err := json.Unmarshal(messageBytes, &messageMap); err != nil {
		return message
	}

	// Replace content field if it exists
	if _, exists := messageMap["content"]; exists {
		messageMap["content"] = newContent
	}

	// Convert back to original message type
	anonymizedBytes, err := json.Marshal(messageMap)
	if err != nil {
		return message
	}

	var anonymizedMessage openai.ChatCompletionMessageParamUnion
	if err := json.Unmarshal(anonymizedBytes, &anonymizedMessage); err != nil {
		return message
	}

	return anonymizedMessage
}

func (l *LocalAnonymizer) applyReplacements(text string, rules map[string]string) string {
	result := text
	// Sort replacements by length (longest first) to avoid partial replacements
	var sortedRules []struct {
		original    string
		replacement string
	}
	for original, replacement := range rules {
		sortedRules = append(sortedRules, struct {
			original    string
			replacement string
		}{original, replacement})
	}

	// Sort by length descending
	for i := 0; i < len(sortedRules); i++ {
		for j := i + 1; j < len(sortedRules); j++ {
			if len(sortedRules[i].original) < len(sortedRules[j].original) {
				sortedRules[i], sortedRules[j] = sortedRules[j], sortedRules[i]
			}
		}
	}

	// Apply replacements
	for _, rule := range sortedRules {
		result = strings.ReplaceAll(result, rule.original, rule.replacement)
	}

	return result
}

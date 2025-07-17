package ai

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

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
	// Start with existing dictionary (convert token -> original to original -> token for internal use)
	workingDict := make(map[string]string)
	for token, original := range existingDict {
		workingDict[original] = token
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
					newRules[replacement] = original // Store as token -> original
				}
			}
		}

		// Apply all known replacements to the message
		anonymizedContent := l.applyReplacements(content, workingDict)
		anonymizedMessages[i] = l.replaceMessageContent(message, anonymizedContent)
	}

	// Convert workingDict to standard format (token -> original)
	updatedDict := make(map[string]string)
	for original, token := range workingDict {
		updatedDict[token] = original
	}

	l.logger.Debug("Memory-only local anonymization complete", "messageCount", len(messages), "newRulesCount", len(newRules))
	return anonymizedMessages, updatedDict, newRules, nil
}

func (l *LocalAnonymizer) anonymizeWithPersistence(ctx context.Context, conversationID string, messages []openai.ChatCompletionMessageParamUnion, existingDict map[string]string, interruptChan <-chan struct{}) ([]openai.ChatCompletionMessageParamUnion, map[string]string, map[string]string, error) {
	// Load existing conversation dictionary
	conversationDict, err := l.store.GetConversationDict(conversationID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load conversation dict: %w", err)
	}

	// Merge with provided existing dictionary (provided dict takes precedence)
	// Convert token -> original to original -> token for internal use
	workingDict := make(map[string]string)
	for token, original := range conversationDict {
		workingDict[original] = token
	}
	for token, original := range existingDict {
		workingDict[original] = token
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

		// Convert workingDict to standard format for anonymizeInMemory
		existingDictForInMemory := make(map[string]string)
		for original, token := range workingDict {
			existingDictForInMemory[token] = original
		}

		// Use in-memory processing for new messages with working dictionary
		_, tempDict, msgRules, err := l.anonymizeInMemory(ctx, newMessages, existingDictForInMemory, interruptChan)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to anonymize new messages: %w", err)
		}

		// Merge new rules and update working dictionary
		for token, original := range msgRules {
			newRules[token] = original
			workingDict[original] = token
		}

		// Update working dictionary with any new discoveries from temp processing
		for token, original := range tempDict {
			if _, exists := workingDict[original]; !exists {
				workingDict[original] = token
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

	// Convert workingDict to standard format for storage
	updatedDict := make(map[string]string)
	for original, token := range workingDict {
		updatedDict[token] = original
	}

	// Save updated dictionary
	if err := l.store.SaveConversationDict(conversationID, updatedDict); err != nil {
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
	return anonymizedMessages, updatedDict, newRules, nil
}

// DeAnonymize implements the Anonymizer interface.
func (l *LocalAnonymizer) DeAnonymize(anonymized string, rules map[string]string) string {
	// Use simple de-anonymization (restore original case)
	return ApplyDeAnonymization(anonymized, rules)
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
	// Use anonymization replacement (preserving token case)
	return ApplyAnonymization(text, rules)
}

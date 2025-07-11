package ai

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/openai/openai-go"

	"github.com/EternisAI/enchanted-twin/pkg/localmodel/llama1b"
)

// LocalAnonymizer is an adapter that wraps llama1b.LlamaAnonymizer to implement the Anonymizer interface.
type LocalAnonymizer struct {
	llama *llama1b.LlamaAnonymizer

	// In-memory storage for conversation dictionaries and message hashes
	// In a production system, this would be backed by a database
	conversationDicts map[string]map[string]string
	messageHashes     map[string]map[string]bool // conversationID -> messageHash -> anonymized
}

// NewLocalAnonymizer creates a new LocalAnonymizer instance.
func NewLocalAnonymizer(llama *llama1b.LlamaAnonymizer) *LocalAnonymizer {
	return &LocalAnonymizer{
		llama:             llama,
		conversationDicts: make(map[string]map[string]string),
		messageHashes:     make(map[string]map[string]bool),
	}
}

// AnonymizeMessages implements the Anonymizer interface.
func (l *LocalAnonymizer) AnonymizeMessages(ctx context.Context, conversationID string, messages []openai.ChatCompletionMessageParamUnion, existingDict map[string]string, interruptChan <-chan struct{}) (anonymizedMessages []openai.ChatCompletionMessageParamUnion, updatedDict map[string]string, newRules map[string]string, err error) {
	// Load existing conversation dictionary
	conversationDict, err := l.LoadConversationDict(conversationID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load conversation dictionary: %w", err)
	}

	// Merge with existing dictionary
	for k, v := range existingDict {
		conversationDict[k] = v
	}

	// Track new rules discovered in this call
	newRules = make(map[string]string)

	// Process each message
	anonymizedMessages = make([]openai.ChatCompletionMessageParamUnion, len(messages))
	for i, message := range messages {
		// Check for interruption
		select {
		case <-interruptChan:
			return nil, nil, nil, context.Canceled
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
				if _, exists := conversationDict[original]; !exists {
					conversationDict[original] = replacement
					newRules[original] = replacement
				}
			}
		}

		// Apply all known replacements to the message
		anonymizedContent := l.applyReplacements(content, conversationDict)
		anonymizedMessages[i] = l.replaceMessageContent(message, anonymizedContent)

		// Mark message as anonymized
		messageHash := l.GetMessageHash(message)
		if l.messageHashes[conversationID] == nil {
			l.messageHashes[conversationID] = make(map[string]bool)
		}
		l.messageHashes[conversationID][messageHash] = true
	}

	// Save updated dictionary
	if err := l.SaveConversationDict(conversationID, conversationDict); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to save conversation dictionary: %w", err)
	}

	return anonymizedMessages, conversationDict, newRules, nil
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

// SaveConversationDict implements the Anonymizer interface.
func (l *LocalAnonymizer) SaveConversationDict(conversationID string, dict map[string]string) error {
	// Create a copy to prevent external modification
	l.conversationDicts[conversationID] = make(map[string]string)
	for k, v := range dict {
		l.conversationDicts[conversationID][k] = v
	}
	return nil
}

// GetMessageHash implements the Anonymizer interface.
func (l *LocalAnonymizer) GetMessageHash(message openai.ChatCompletionMessageParamUnion) string {
	// Create a hash based on message content
	content := l.extractMessageContent(message)
	hash := md5.Sum([]byte(content))
	return fmt.Sprintf("%x", hash)
}

// IsMessageAnonymized implements the Anonymizer interface.
func (l *LocalAnonymizer) IsMessageAnonymized(conversationID, messageHash string) (bool, error) {
	if hashes, exists := l.messageHashes[conversationID]; exists {
		return hashes[messageHash], nil
	}
	return false, nil
}

// Shutdown implements the Anonymizer interface.
func (l *LocalAnonymizer) Shutdown() {
	if l.llama != nil {
		_ = l.llama.Close()
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

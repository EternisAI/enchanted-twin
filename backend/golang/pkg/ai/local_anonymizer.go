package ai

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go/v3"
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

		// Check if content contains only already anonymized names
		shouldAnonymize := l.shouldAnonymizeContent(content, workingDict)

		var nameReplacements map[string]string
		if shouldAnonymize {
			l.logger.Info("[Privacy] Anonymizing message locally")
			// Use local LLM to find new names/entities
			var err error
			nameReplacements, err = l.llama.Anonymize(ctx, content)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("failed to anonymize message content: %w", err)
			}
			l.logger.Info("[Privacy] Anonymizer replacements", "count", len(nameReplacements))
		} else {
			nameReplacements = make(map[string]string)
		}

		// Update dictionaries with new discoveries, but prevent re-anonymizing already anonymized names
		for original, replacement := range nameReplacements {
			if original != "" && replacement != "" && original != replacement {
				// Only add if not already in dictionary AND original is not already an anonymized name
				if _, exists := workingDict[original]; !exists {
					// Check if 'original' is actually an already-anonymized name (exists as a value in workingDict)
					isAlreadyAnonymized := false
					for _, anonymizedName := range workingDict {
						if strings.EqualFold(anonymizedName, original) {
							isAlreadyAnonymized = true
							l.logger.Debug("Prevented re-anonymization of already anonymized name",
								"already_anonymized", original,
								"would_become", replacement)
							break
						}
					}

					if !isAlreadyAnonymized {
						workingDict[original] = replacement
						newRules[replacement] = original // Store as token -> original
					}
				}
			}
		}

		// Apply all known replacements to the message
		anonymizedContent := l.applyReplacements(content, workingDict)
		if anonymizedContent != content {
			l.logger.Info("[Privacy] Applied anonymization to message")
		} else {
			l.logger.Info("[Privacy] No changes after anonymization (content unchanged)")
		}
		anonymizedMessages[i] = l.replaceMessageContent(message, anonymizedContent)
	}

	// Convert workingDict to standard format (token -> original) and resolve chains
	updatedDict := make(map[string]string)
	for original, token := range workingDict {
		updatedDict[token] = original
	}

	// Resolve any chain mappings in the dictionary
	updatedDict = l.resolveChainMappings(updatedDict)

	l.logger.Info("[Privacy] Local anonymization complete", "messageCount", len(messages), "newRulesCount", len(newRules))
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

	// Convert workingDict to standard format for storage and resolve chains
	updatedDict := make(map[string]string)
	for original, token := range workingDict {
		updatedDict[token] = original
	}

	// Resolve any chain mappings in the dictionary
	updatedDict = l.resolveChainMappings(updatedDict)

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

func (l *LocalAnonymizer) shouldAnonymizeContent(content string, workingDict map[string]string) bool {
	// Create a trie with reverse mappings (anonymized -> original) to detect already anonymized content
	reverseDict := make(map[string]string)
	for original, anonymized := range workingDict {
		reverseDict[anonymized] = original
	}

	if len(reverseDict) == 0 {
		// No existing anonymizations, should anonymize
		return true
	}

	// Use replacement trie to find all anonymized names in content
	trie := NewReplacementTrieFromRules(reverseDict)
	_, foundMappings := trie.ReplaceAll(content)

	// If we found anonymized names in the content, check if they cover most of the named entities
	if len(foundMappings) > 0 {
		// Extract potential names using simple heuristics (capitalized words)
		potentialNames := l.extractPotentialNames(content)

		// If most potential names are already anonymized, skip LLM anonymization
		anonymizedCount := 0
		for _, name := range potentialNames {
			if _, exists := reverseDict[name]; exists {
				anonymizedCount++
			}
		}

		// If more than 50% of potential names are already anonymized, don't re-anonymize
		if len(potentialNames) > 0 && float64(anonymizedCount)/float64(len(potentialNames)) > 0.5 {
			return false
		}
	}

	return true
}

func (l *LocalAnonymizer) extractPotentialNames(text string) []string {
	// Simple heuristic: find sequences of capitalized words that could be names
	// This is a basic implementation - could be improved with NLP libraries
	words := strings.Fields(text)
	var potentialNames []string

	for _, word := range words {
		// Remove punctuation and check if it starts with capital letter
		cleaned := strings.Trim(word, ".,!?;:")
		if len(cleaned) > 1 && cleaned[0] >= 'A' && cleaned[0] <= 'Z' {
			// Check if it's not a common word (basic check)
			if !l.isCommonWord(cleaned) {
				potentialNames = append(potentialNames, cleaned)
			}
		}
	}

	return potentialNames
}

func (l *LocalAnonymizer) isCommonWord(word string) bool {
	// Basic list of common words that shouldn't be considered names
	commonWords := map[string]bool{
		"I": true, "The": true, "This": true, "That": true, "What": true, "Who": true,
		"When": true, "Where": true, "How": true, "Why": true, "Can": true, "Will": true,
		"Should": true, "Could": true, "Would": true, "May": true, "Might": true,
		"Please": true, "Thank": true, "Thanks": true, "Hello": true, "Hi": true,
		"Yes": true, "No": true, "OK": true, "Okay": true,
	}
	return commonWords[word]
}

func (l *LocalAnonymizer) resolveChainMappings(dict map[string]string) map[string]string {
	resolved := make(map[string]string)

	// For each mapping in the dictionary
	for anonymized, original := range dict {
		// Trace back to find the true original
		trueOriginal := original
		visited := make(map[string]bool)

		// Follow the chain backwards until we find a non-anonymized original
		for {
			// Prevent infinite loops
			if visited[trueOriginal] {
				l.logger.Warn("Detected circular reference in anonymization dictionary",
					"anonymized", anonymized, "circular_original", trueOriginal)
				break
			}
			visited[trueOriginal] = true

			// Check if this "original" is actually an anonymized version of something else
			foundDeeperOriginal := false
			for anonKey, origValue := range dict {
				if anonKey == trueOriginal {
					trueOriginal = origValue
					foundDeeperOriginal = true
					break
				}
			}

			// If we didn't find a deeper original, we've reached the true original
			if !foundDeeperOriginal {
				break
			}
		}

		resolved[anonymized] = trueOriginal

		if trueOriginal != original {
			l.logger.Debug("Resolved chain mapping",
				"anonymized", anonymized,
				"chain_original", original,
				"true_original", trueOriginal)
		}
	}

	return resolved
}

func (l *LocalAnonymizer) applyReplacements(text string, rules map[string]string) string {
	// Use anonymization replacement (preserving token case)
	return ApplyAnonymization(text, rules)
}

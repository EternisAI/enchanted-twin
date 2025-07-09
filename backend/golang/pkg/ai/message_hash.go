package ai

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/openai/openai-go"
)

type MessageHasher struct{}

func NewMessageHasher() *MessageHasher {
	return &MessageHasher{}
}

func (h *MessageHasher) GetMessageHash(message openai.ChatCompletionMessageParamUnion) string {
	// Extract relevant fields for hashing
	hashData := h.extractHashableFields(message)

	// Create deterministic JSON representation
	jsonBytes, err := json.Marshal(hashData)
	if err != nil {
		// Fallback to string representation if JSON fails
		jsonBytes = []byte(fmt.Sprintf("%+v", hashData))
	}

	// Generate SHA256 hash
	hash := sha256.Sum256(jsonBytes)
	return fmt.Sprintf("%x", hash)
}

func (h *MessageHasher) extractHashableFields(message openai.ChatCompletionMessageParamUnion) map[string]interface{} {
	// Convert message to JSON to extract fields
	messageBytes, err := json.Marshal(message)
	if err != nil {
		return map[string]interface{}{
			"error":   err.Error(),
			"message": fmt.Sprintf("%+v", message),
		}
	}

	var messageMap map[string]interface{}
	if err := json.Unmarshal(messageBytes, &messageMap); err != nil {
		return map[string]interface{}{
			"error":   err.Error(),
			"message": string(messageBytes),
		}
	}

	// Extract and normalize relevant fields
	hashData := make(map[string]interface{})

	// Core fields that affect anonymization
	if role, exists := messageMap["role"]; exists {
		hashData["role"] = role
	}

	if content, exists := messageMap["content"]; exists {
		// Normalize content string
		if contentStr, ok := content.(string); ok {
			hashData["content"] = h.normalizeContent(contentStr)
		} else {
			hashData["content"] = content
		}
	}

	if name, exists := messageMap["name"]; exists {
		hashData["name"] = name
	}

	// Handle tool calls if present
	if toolCalls, exists := messageMap["tool_calls"]; exists {
		if toolCallsSlice, ok := toolCalls.([]interface{}); ok {
			normalizedToolCalls := make([]interface{}, len(toolCallsSlice))
			for i, tc := range toolCallsSlice {
				if tcMap, ok := tc.(map[string]interface{}); ok {
					normalizedToolCalls[i] = h.normalizeToolCall(tcMap)
				} else {
					normalizedToolCalls[i] = tc
				}
			}
			hashData["tool_calls"] = normalizedToolCalls
		} else {
			hashData["tool_calls"] = toolCalls
		}
	}

	// Handle function calls if present
	if functionCall, exists := messageMap["function_call"]; exists {
		if fcMap, ok := functionCall.(map[string]interface{}); ok {
			hashData["function_call"] = h.normalizeFunctionCall(fcMap)
		} else {
			hashData["function_call"] = functionCall
		}
	}

	// Handle tool call ID if present
	if toolCallID, exists := messageMap["tool_call_id"]; exists {
		hashData["tool_call_id"] = toolCallID
	}

	return hashData
}

func (h *MessageHasher) normalizeContent(content string) string {
	// Normalize whitespace and case for consistent hashing
	content = strings.TrimSpace(content)
	// Note: We don't normalize case as it might be semantically important
	return content
}

func (h *MessageHasher) normalizeToolCall(toolCall map[string]interface{}) map[string]interface{} {
	normalized := make(map[string]interface{})

	if id, exists := toolCall["id"]; exists {
		normalized["id"] = id
	}

	if tcType, exists := toolCall["type"]; exists {
		normalized["type"] = tcType
	}

	if function, exists := toolCall["function"]; exists {
		if funcMap, ok := function.(map[string]interface{}); ok {
			normalized["function"] = h.normalizeFunctionCall(funcMap)
		} else {
			normalized["function"] = function
		}
	}

	return normalized
}

func (h *MessageHasher) normalizeFunctionCall(functionCall map[string]interface{}) map[string]interface{} {
	normalized := make(map[string]interface{})

	if name, exists := functionCall["name"]; exists {
		normalized["name"] = name
	}

	if arguments, exists := functionCall["arguments"]; exists {
		// Normalize JSON arguments by parsing and re-serializing
		if argStr, ok := arguments.(string); ok {
			var argObj interface{}
			if err := json.Unmarshal([]byte(argStr), &argObj); err == nil {
				// Re-serialize to ensure consistent formatting
				if normalizedBytes, err := json.Marshal(argObj); err == nil {
					normalized["arguments"] = string(normalizedBytes)
				} else {
					normalized["arguments"] = argStr
				}
			} else {
				normalized["arguments"] = argStr
			}
		} else {
			normalized["arguments"] = arguments
		}
	}

	return normalized
}

func (h *MessageHasher) GetBatchHash(messages []openai.ChatCompletionMessageParamUnion) string {
	// Create hash for a batch of messages
	var messageHashes []string
	for _, message := range messages {
		messageHashes = append(messageHashes, h.GetMessageHash(message))
	}

	// Sort hashes for deterministic batch hash
	sort.Strings(messageHashes)

	batchString := strings.Join(messageHashes, "|")
	hash := sha256.Sum256([]byte(batchString))
	return fmt.Sprintf("%x", hash)
}

func (h *MessageHasher) GetConversationHash(conversationID string, messages []openai.ChatCompletionMessageParamUnion) string {
	// Create hash that combines conversation ID and message batch
	batchHash := h.GetBatchHash(messages)
	combinedString := fmt.Sprintf("%s:%s", conversationID, batchHash)

	hash := sha256.Sum256([]byte(combinedString))
	return fmt.Sprintf("%x", hash)
}

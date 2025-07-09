package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go"
)

type anonymizationRequest struct {
	ctx           context.Context
	messages      []openai.ChatCompletionMessageParamUnion
	interruptChan <-chan struct{}
	responseChan  chan anonymizationResponse
}

type anonymizationResponse struct {
	messages []openai.ChatCompletionMessageParamUnion
	rules    map[string]string
	err      error
}

type MockAnonymizer struct {
	Delay                  time.Duration
	PredefinedReplacements map[string]string

	requestChan chan anonymizationRequest
	done        chan struct{}
	logger      *log.Logger
}

var (
	mockAnonymizerInstance *MockAnonymizer
	mockAnonymizerOnce     sync.Once
)

// NoOpAnonymizer is a no-op implementation that passes messages through unchanged.
type NoOpAnonymizer struct {
	logger *log.Logger
}

func (n *NoOpAnonymizer) AnonymizeMessages(ctx context.Context, conversationID string, messages []openai.ChatCompletionMessageParamUnion, existingDict map[string]string, interruptChan <-chan struct{}) ([]openai.ChatCompletionMessageParamUnion, map[string]string, map[string]string, error) {
	// Return messages unchanged with empty replacement rules
	emptyDict := make(map[string]string)
	return messages, emptyDict, emptyDict, nil
}

func (n *NoOpAnonymizer) DeAnonymize(text string, rules map[string]string) string {
	// Return text unchanged
	return text
}

func (n *NoOpAnonymizer) Shutdown() {
	// No-op
}

func (n *NoOpAnonymizer) LoadConversationDict(conversationID string) (map[string]string, error) {
	// No-op anonymizer doesn't support persistence
	return make(map[string]string), nil
}

func (n *NoOpAnonymizer) SaveConversationDict(conversationID string, dict map[string]string) error {
	// No-op anonymizer doesn't support persistence
	return nil
}

func (n *NoOpAnonymizer) GetMessageHash(message openai.ChatCompletionMessageParamUnion) string {
	// Simple hash implementation for no-op
	hasher := NewMessageHasher()
	return hasher.GetMessageHash(message)
}

func (n *NoOpAnonymizer) IsMessageAnonymized(conversationID, messageHash string) (bool, error) {
	// No-op anonymizer doesn't support persistence
	return false, nil
}

var defaultReplacements = map[string]string{
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

// InitMockAnonymizer creates a mock anonymizer if enabled via ENABLE_MOCK_ANONYMIZER=true,
// otherwise returns a no-op anonymizer that passes messages through unchanged.
func InitMockAnonymizer(delay time.Duration, enabled bool, logger *log.Logger) Anonymizer {
	if !enabled {
		logger.Info("MockAnonymizer disabled, using no-op anonymizer")
		return &NoOpAnonymizer{logger: logger}
	}

	mockAnonymizerOnce.Do(func() {
		mockAnonymizerInstance = &MockAnonymizer{
			Delay:                  delay,
			PredefinedReplacements: defaultReplacements,
			requestChan:            make(chan anonymizationRequest, 10), // Buffer for requests
			done:                   make(chan struct{}),
			logger:                 logger,
		}

		// Start single-threaded processor goroutine
		go mockAnonymizerInstance.processRequests()

		logger.Info("MockAnonymizer singleton initialized", "delay", delay)
	})

	return mockAnonymizerInstance
}

func (m *MockAnonymizer) AnonymizeMessages(ctx context.Context, conversationID string, messages []openai.ChatCompletionMessageParamUnion, existingDict map[string]string, interruptChan <-chan struct{}) ([]openai.ChatCompletionMessageParamUnion, map[string]string, map[string]string, error) {
	responseChan := make(chan anonymizationResponse, 1)

	request := anonymizationRequest{
		ctx:           ctx,
		messages:      messages,
		interruptChan: interruptChan,
		responseChan:  responseChan,
	}

	// Send request to single-threaded processor
	select {
	case m.requestChan <- request:
	case <-ctx.Done():
		return nil, nil, nil, ctx.Err()
	case <-interruptChan:
		return nil, nil, nil, fmt.Errorf("anonymization interrupted before processing")
	}

	// Wait for response
	select {
	case response := <-responseChan:
		// For mock anonymizer, updatedDict is the same as rules (no persistence)
		return response.messages, response.rules, response.rules, response.err
	case <-ctx.Done():
		return nil, nil, nil, ctx.Err()
	case <-interruptChan:
		return nil, nil, nil, fmt.Errorf("anonymization interrupted while waiting for response")
	}
}

func (m *MockAnonymizer) processRequests() {
	for {
		select {
		case <-m.done:
			return
		case request := <-m.requestChan:
			response := m.processAnonymizationRequest(request)

			// Send response back, handling potential channel closure
			select {
			case request.responseChan <- response:
			case <-request.ctx.Done():
				// Request context was canceled, don't block
			case <-request.interruptChan:
				// Request was interrupted, don't block
			case <-m.done:
				// Anonymizer is shutting down
				return
			}
		}
	}
}

func (m *MockAnonymizer) processAnonymizationRequest(request anonymizationRequest) anonymizationResponse {
	allRules := make(map[string]string)
	anonymizedMessages := make([]openai.ChatCompletionMessageParamUnion, len(request.messages))

	for i, message := range request.messages {
		// Check for interruption during processing
		select {
		case <-request.interruptChan:
			return anonymizationResponse{
				messages: nil,
				rules:    nil,
				err:      fmt.Errorf("anonymization interrupted by scheduler"),
			}
		default:
		}

		// Apply message-level delay that can be interrupted
		if m.Delay > 0 {
			m.logger.Debug("Applying message anonymization delay", "delay", m.Delay, "message", i)
			select {
			case <-time.After(m.Delay):
				// Full delay completed
			case <-request.interruptChan:
				// Interrupted by scheduler
				return anonymizationResponse{
					messages: nil,
					rules:    nil,
					err:      fmt.Errorf("message anonymization interrupted by scheduler"),
				}
			case <-request.ctx.Done():
				// Context canceled
				return anonymizationResponse{
					messages: nil,
					rules:    nil,
					err:      request.ctx.Err(),
				}
			}
		}

		anonymizedMsg, rules, err := m.anonymizeMessage(request.ctx, message)
		if err != nil {
			return anonymizationResponse{
				messages: nil,
				rules:    nil,
				err:      fmt.Errorf("failed to anonymize message %d: %w", i, err),
			}
		}

		anonymizedMessages[i] = anonymizedMsg

		// Merge rules (handle conflicts by keeping first occurrence)
		for token, original := range rules {
			if existing, exists := allRules[token]; exists && existing != original {
				m.logger.Warn("Rule conflict detected", "token", token, "existing", existing, "new", original)
			}
			allRules[token] = original
		}
	}

	return anonymizationResponse{
		messages: anonymizedMessages,
		rules:    allRules,
		err:      nil,
	}
}

func (m *MockAnonymizer) anonymizeMessage(ctx context.Context, message openai.ChatCompletionMessageParamUnion) (openai.ChatCompletionMessageParamUnion, map[string]string, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return message, nil, ctx.Err()
	default:
	}

	// Convert message to JSON to extract content
	messageBytes, err := json.Marshal(message)
	if err != nil {
		return message, nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	var messageMap map[string]interface{}
	if err := json.Unmarshal(messageBytes, &messageMap); err != nil {
		return message, nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	// Anonymize content field if it exists
	if content, exists := messageMap["content"]; exists {
		if contentStr, ok := content.(string); ok {
			anonymizedContent, rules, err := m.anonymizeContent(ctx, contentStr)
			if err != nil {
				return message, nil, fmt.Errorf("failed to anonymize content: %w", err)
			}

			messageMap["content"] = anonymizedContent

			// Convert back to the original message type
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
	}

	// If no content field or not a string, return as-is
	return message, make(map[string]string), nil
}

func (m *MockAnonymizer) anonymizeContent(ctx context.Context, content string) (string, map[string]string, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return "", nil, ctx.Err()
	default:
	}

	anonymized := content
	rules := make(map[string]string)

	// Create a sorted list of replacements by length (longest first) to ensure longer strings are processed first
	type replacement struct {
		original string
		token    string
	}

	var sortedReplacements []replacement
	for original, token := range m.PredefinedReplacements {
		sortedReplacements = append(sortedReplacements, replacement{original: original, token: token})
	}

	// Sort by length descending (longest first)
	sort.Slice(sortedReplacements, func(i, j int) bool {
		return len(sortedReplacements[i].original) > len(sortedReplacements[j].original)
	})

	// Apply predefined replacements in order (longest first)
	for _, repl := range sortedReplacements {
		if strings.Contains(anonymized, repl.original) {
			anonymized = strings.ReplaceAll(anonymized, repl.original, repl.token)
			rules[repl.token] = repl.original // Store replacement -> original mapping
			m.logger.Debug("Applied anonymization", "original", repl.original, "replacement", repl.token)
		}
	}

	m.logger.Debug("Anonymization complete", "originalLength", len(content), "anonymizedLength", len(anonymized), "rulesCount", len(rules))

	return anonymized, rules, nil
}

func (m *MockAnonymizer) DeAnonymize(anonymized string, rules map[string]string) string {
	restored := anonymized

	// Create a sorted list of tokens by length (longest first) to ensure longer tokens are processed first
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

	m.logger.Debug("De-anonymization complete", "anonymizedLength", len(anonymized), "restoredLength", len(restored))

	return restored
}

func (m *MockAnonymizer) Shutdown() {
	close(m.done)
}

func (m *MockAnonymizer) LoadConversationDict(conversationID string) (map[string]string, error) {
	// Mock anonymizer doesn't support persistence
	return make(map[string]string), nil
}

func (m *MockAnonymizer) SaveConversationDict(conversationID string, dict map[string]string) error {
	// Mock anonymizer doesn't support persistence
	return nil
}

func (m *MockAnonymizer) GetMessageHash(message openai.ChatCompletionMessageParamUnion) string {
	// Simple hash implementation for mock
	hasher := NewMessageHasher()
	return hasher.GetMessageHash(message)
}

func (m *MockAnonymizer) IsMessageAnonymized(conversationID, messageHash string) (bool, error) {
	// Mock anonymizer doesn't support persistence
	return false, nil
}

// ResetMockAnonymizerForTesting resets the singleton instance for testing purposes.
// This should only be used in tests.
func ResetMockAnonymizerForTesting() {
	if mockAnonymizerInstance != nil {
		mockAnonymizerInstance.Shutdown()
	}
	mockAnonymizerInstance = nil
	mockAnonymizerOnce = sync.Once{}
}

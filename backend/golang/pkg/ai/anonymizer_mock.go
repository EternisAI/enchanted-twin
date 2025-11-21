package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go/v3"
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

// NoOpAnonymizer is a no-op implementation that passes messages through unchanged.
type NoOpAnonymizer struct {
	logger *log.Logger
}

// NewNoOpAnonymizer creates a new NoOpAnonymizer instance.
func NewNoOpAnonymizer(logger *log.Logger) *NoOpAnonymizer {
	return &NoOpAnonymizer{logger: logger}
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
	// Full names (processed first due to length sorting)
	"John Smith":    "PERSON_001",
	"Jane Doe":      "PERSON_002",
	"Alice Johnson": "PERSON_003",

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

// NewMockAnonymizer creates a new mock anonymizer instance.
// This replaces the old singleton pattern with dependency injection.
func NewMockAnonymizer(delay time.Duration, replacements map[string]string, logger *log.Logger) *MockAnonymizer {
	if replacements == nil {
		replacements = defaultReplacements
	}

	mockAnonymizer := &MockAnonymizer{
		Delay:                  delay,
		PredefinedReplacements: replacements,
		requestChan:            make(chan anonymizationRequest, 10),
		done:                   make(chan struct{}),
		logger:                 logger,
	}

	// Start single-threaded processor goroutine
	go mockAnonymizer.processRequests()

	logger.Info("MockAnonymizer created", "delay", delay)
	return mockAnonymizer
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
			m.logger.Warn("Anonymization interrupted by scheduler", "messageIndex", i)
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
				m.logger.Warn("Message anonymization interrupted by scheduler during delay", "messageIndex", i, "delay", m.Delay)
				return anonymizationResponse{
					messages: nil,
					rules:    nil,
					err:      fmt.Errorf("message anonymization interrupted by scheduler"),
				}
			case <-request.ctx.Done():
				// Context canceled
				m.logger.Info("Message anonymization canceled by context during delay", "messageIndex", i, "delay", m.Delay, "contextErr", request.ctx.Err())
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
		m.logger.Info("Message anonymization canceled by context", "contextErr", ctx.Err())
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

	// Use replacement trie to find actual matches and build rules
	trie := NewReplacementTrie()
	for original, token := range m.PredefinedReplacements {
		trie.Insert(original, token)
	}

	// Apply anonymization and get the actual replacement rules used
	anonymized, rules := trie.ReplaceAll(content)

	m.logger.Debug("Anonymization complete", "originalLength", len(content), "anonymizedLength", len(anonymized), "rulesCount", len(rules))

	return anonymized, rules, nil
}

func (m *MockAnonymizer) DeAnonymize(anonymized string, rules map[string]string) string {
	// Apply simple de-anonymization (restore original case)
	restored := ApplyDeAnonymization(anonymized, rules)

	m.logger.Debug("De-anonymization complete", "anonymizedLength", len(anonymized), "restoredLength", len(restored))

	return restored
}

func (m *MockAnonymizer) Shutdown() {
	select {
	case <-m.done:
		// Already closed
		return
	default:
		close(m.done)
	}
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

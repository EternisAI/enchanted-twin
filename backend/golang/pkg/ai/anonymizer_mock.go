package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go"
)

type MockAnonymizer struct {
	Delay                  time.Duration
	PredefinedReplacements map[string]string

	logger *log.Logger
}

var (
	mockAnonymizerInstance *MockAnonymizer
	mockAnonymizerOnce     sync.Once
)

func GetMockAnonymizer() *MockAnonymizer {
	if mockAnonymizerInstance == nil {
		panic("MockAnonymizer not initialized. Call InitMockAnonymizer first.")
	}
	return mockAnonymizerInstance
}

func InitMockAnonymizer(delay time.Duration, logger *log.Logger) *MockAnonymizer {
	mockAnonymizerOnce.Do(func() {
		mockAnonymizerInstance = &MockAnonymizer{
			Delay: delay,
			PredefinedReplacements: map[string]string{
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

				// Email patterns (will be handled by regex)
				"john@example.com":  "EMAIL_001",
				"alice@company.com": "EMAIL_002",

				// Phone patterns
				"+1-555-123-4567": "PHONE_001",
				"555-987-6543":    "PHONE_002",
			},
			logger: logger,
		}

		logger.Info("MockAnonymizer singleton initialized", "delay", delay)
	})

	return mockAnonymizerInstance
}

func NewMockAnonymizer(delay time.Duration, logger *log.Logger) *MockAnonymizer {
	return &MockAnonymizer{
		Delay: delay,
		PredefinedReplacements: map[string]string{
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

			// Email patterns (will be handled by regex)
			"john@example.com":  "EMAIL_001",
			"alice@company.com": "EMAIL_002",

			// Phone patterns
			"+1-555-123-4567": "PHONE_001",
			"555-987-6543":    "PHONE_002",
		},
		logger: logger,
	}
}

func (m *MockAnonymizer) AnonymizeMessages(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, interruptChan <-chan struct{}) ([]openai.ChatCompletionMessageParamUnion, map[string]string, error) {
	allRules := make(map[string]string)
	anonymizedMessages := make([]openai.ChatCompletionMessageParamUnion, len(messages))

	for i, message := range messages {
		// Check for interruption during processing
		select {
		case <-interruptChan:
			return nil, nil, fmt.Errorf("anonymization interrupted by scheduler")
		default:
		}

		// Apply message-level delay that can be interrupted
		if m.Delay > 0 {
			m.logger.Debug("Applying message anonymization delay", "delay", m.Delay, "message", i)
			select {
			case <-time.After(m.Delay):
				// Full delay completed
			case <-interruptChan:
				// Interrupted by scheduler
				return nil, nil, fmt.Errorf("message anonymization interrupted by scheduler")
			case <-ctx.Done():
				// Context canceled
				return nil, nil, ctx.Err()
			}
		}

		anonymizedMsg, rules, err := m.anonymizeMessage(ctx, message)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to anonymize message %d: %w", i, err)
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

	return anonymizedMessages, allRules, nil
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

	// Apply predefined replacements (case-insensitive)
	for original, replacement := range m.PredefinedReplacements {
		// Use case-insensitive replacement
		re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(original) + `\b`)
		matches := re.FindAllString(anonymized, -1)

		for _, match := range matches {
			if match != replacement { // Don't replace if it's already anonymized
				anonymized = strings.ReplaceAll(anonymized, match, replacement)
				rules[replacement] = match // Store replacement -> original mapping
				m.logger.Debug("Applied anonymization", "original", match, "replacement", replacement)
			}
		}
	}

	// Apply regex patterns for common sensitive data
	anonymized, additionalRules := m.anonymizePatterns(anonymized)

	// Merge additional rules
	for k, v := range additionalRules {
		rules[k] = v
	}

	m.logger.Debug("Anonymization complete", "originalLength", len(content), "anonymizedLength", len(anonymized), "rulesCount", len(rules))

	return anonymized, rules, nil
}

func (m *MockAnonymizer) DeAnonymize(anonymized string, rules map[string]string) string {
	restored := anonymized

	// Apply rules in reverse (anonymized token -> original)
	for token, original := range rules {
		restored = strings.ReplaceAll(restored, token, original)
	}

	m.logger.Debug("De-anonymization complete", "anonymizedLength", len(anonymized), "restoredLength", len(restored))

	return restored
}

func (m *MockAnonymizer) anonymizePatterns(content string) (string, map[string]string) {
	result := content
	rules := make(map[string]string)

	// Email pattern
	emailRegex := regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`)
	emailMatches := emailRegex.FindAllString(result, -1)
	for i, email := range emailMatches {
		token := fmt.Sprintf("EMAIL_%03d", i+100) // Start from 100 to avoid conflicts with predefined
		result = strings.ReplaceAll(result, email, token)
		rules[token] = email
	}

	// Phone number pattern (simple US format)
	phoneRegex := regexp.MustCompile(`\b(\+?1[-.\s]?)?(\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4})\b`)
	phoneMatches := phoneRegex.FindAllString(result, -1)
	for i, phone := range phoneMatches {
		token := fmt.Sprintf("PHONE_%03d", i+100)
		result = strings.ReplaceAll(result, phone, token)
		rules[token] = phone
	}

	// SSN pattern (XXX-XX-XXXX)
	ssnRegex := regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)
	ssnMatches := ssnRegex.FindAllString(result, -1)
	for i, ssn := range ssnMatches {
		token := fmt.Sprintf("SSN_%03d", i+100)
		result = strings.ReplaceAll(result, ssn, token)
		rules[token] = ssn
	}

	return result, rules
}

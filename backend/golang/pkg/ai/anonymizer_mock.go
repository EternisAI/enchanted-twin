package ai

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/log"
)

// MockAnonymizer provides a configurable mock implementation for testing and development
type MockAnonymizer struct {
	// Delay simulates processing time
	Delay time.Duration
	
	// PredefinedReplacements maps original terms to anonymized versions
	PredefinedReplacements map[string]string
	
	logger *log.Logger
}

// NewMockAnonymizer creates a new mock anonymizer with configurable settings
func NewMockAnonymizer(delay time.Duration, logger *log.Logger) *MockAnonymizer {
	return &MockAnonymizer{
		Delay: delay,
		PredefinedReplacements: map[string]string{
			// Common names
			"John":     "PERSON_001",
			"Jane":     "PERSON_002", 
			"Alice":    "PERSON_003",
			"Bob":      "PERSON_004",
			"Charlie":  "PERSON_005",
			"David":    "PERSON_006",
			"Emma":     "PERSON_007",
			"Frank":    "PERSON_008",
			
			// Company names
			"OpenAI":      "COMPANY_001",
			"Microsoft":   "COMPANY_002",
			"Google":      "COMPANY_003",
			"Apple":       "COMPANY_004",
			"Tesla":       "COMPANY_005",
			"Amazon":      "COMPANY_006",
			
			// Locations
			"New York":    "LOCATION_001",
			"London":      "LOCATION_002",
			"Tokyo":       "LOCATION_003",
			"Paris":       "LOCATION_004",
			"Berlin":      "LOCATION_005",
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

// Anonymize processes content and replaces sensitive information with anonymized tokens
func (m *MockAnonymizer) Anonymize(ctx context.Context, content string) (string, map[string]string, error) {
	if m.Delay > 0 {
		m.logger.Debug("Simulating anonymization delay", "delay", m.Delay)
		select {
		case <-time.After(m.Delay):
		case <-ctx.Done():
			return "", nil, ctx.Err()
		}
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

// DeAnonymize restores original content using replacement rules
func (m *MockAnonymizer) DeAnonymize(anonymized string, rules map[string]string) string {
	restored := anonymized
	
	// Apply rules in reverse (anonymized token -> original)
	for token, original := range rules {
		restored = strings.ReplaceAll(restored, token, original)
	}
	
	m.logger.Debug("De-anonymization complete", "anonymizedLength", len(anonymized), "restoredLength", len(restored))
	
	return restored
}

// anonymizePatterns applies regex-based anonymization for common patterns
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

// AddReplacement allows adding custom replacements at runtime
func (m *MockAnonymizer) AddReplacement(original, replacement string) {
	m.PredefinedReplacements[original] = replacement
	m.logger.Debug("Added custom replacement", "original", original, "replacement", replacement)
}

// SetDelay updates the processing delay
func (m *MockAnonymizer) SetDelay(delay time.Duration) {
	m.Delay = delay
	m.logger.Debug("Updated anonymization delay", "delay", delay)
}
package agent

import (
	"strings"
	"testing"

	"github.com/openai/openai-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
)

func TestAgentResponse_ReplacementRules(t *testing.T) {
	tests := []struct {
		name              string
		response          AgentResponse
		expectedRules     map[string]string
		expectedContent   string
		expectedToolCalls int
	}{
		{
			name: "response with replacement rules",
			response: AgentResponse{
				Content: "Hello PERSON_001, how are you?",
				ReplacementRules: map[string]string{
					"PERSON_001": "John Smith",
					"PERSON_002": "Jane Doe",
				},
				ToolCalls:   []openai.ChatCompletionMessageToolCall{},
				ToolResults: []types.ToolResult{},
				ImageURLs:   []string{},
			},
			expectedRules: map[string]string{
				"PERSON_001": "John Smith",
				"PERSON_002": "Jane Doe",
			},
			expectedContent:   "Hello PERSON_001, how are you?",
			expectedToolCalls: 0,
		},
		{
			name: "response with empty replacement rules",
			response: AgentResponse{
				Content:          "Hello, how are you?",
				ReplacementRules: map[string]string{},
				ToolCalls:        []openai.ChatCompletionMessageToolCall{},
				ToolResults:      []types.ToolResult{},
				ImageURLs:        []string{},
			},
			expectedRules:     map[string]string{},
			expectedContent:   "Hello, how are you?",
			expectedToolCalls: 0,
		},
		{
			name: "response with nil replacement rules",
			response: AgentResponse{
				Content:          "Hello, how are you?",
				ReplacementRules: nil,
				ToolCalls:        []openai.ChatCompletionMessageToolCall{},
				ToolResults:      []types.ToolResult{},
				ImageURLs:        []string{},
			},
			expectedRules:     nil,
			expectedContent:   "Hello, how are you?",
			expectedToolCalls: 0,
		},
		{
			name: "response with multiple types of replacements",
			response: AgentResponse{
				Content: "PERSON_001 works at COMPANY_001 in LOCATION_001",
				ReplacementRules: map[string]string{
					"PERSON_001":   "Alice Johnson",
					"COMPANY_001":  "Tech Corp",
					"LOCATION_001": "San Francisco",
					"EMAIL_001":    "alice@techcorp.com",
					"DATE_001":     "2025-07-13",
				},
				ToolCalls:   []openai.ChatCompletionMessageToolCall{},
				ToolResults: []types.ToolResult{},
				ImageURLs:   []string{},
			},
			expectedRules: map[string]string{
				"PERSON_001":   "Alice Johnson",
				"COMPANY_001":  "Tech Corp",
				"LOCATION_001": "San Francisco",
				"EMAIL_001":    "alice@techcorp.com",
				"DATE_001":     "2025-07-13",
			},
			expectedContent:   "PERSON_001 works at COMPANY_001 in LOCATION_001",
			expectedToolCalls: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that the response contains the expected replacement rules
			assert.Equal(t, tt.expectedRules, tt.response.ReplacementRules)
			assert.Equal(t, tt.expectedContent, tt.response.Content)
			assert.Equal(t, tt.expectedToolCalls, len(tt.response.ToolCalls))

			// Test that the response can be used to check for anonymization
			if len(tt.response.ReplacementRules) > 0 {
				assert.True(t, hasAnonymizedContent(tt.response), "Response should indicate anonymized content")
			} else {
				assert.False(t, hasAnonymizedContent(tt.response), "Response should not indicate anonymized content")
			}
		})
	}
}

func TestAgentResponse_String(t *testing.T) {
	tests := []struct {
		name     string
		response AgentResponse
		expected string
	}{
		{
			name: "response without images",
			response: AgentResponse{
				Content:          "Hello, world!",
				ReplacementRules: map[string]string{"PERSON_001": "John"},
				ImageURLs:        []string{},
			},
			expected: "Hello, world!",
		},
		{
			name: "response with images",
			response: AgentResponse{
				Content:          "Here's an image:",
				ReplacementRules: map[string]string{"PERSON_001": "John"},
				ImageURLs:        []string{"https://example.com/image1.jpg", "https://example.com/image2.jpg"},
			},
			expected: "Here's an image:\nImages:https://example.com/image1.jpg,https://example.com/image2.jpg",
		},
		{
			name: "response with empty content and images",
			response: AgentResponse{
				Content:          "",
				ReplacementRules: map[string]string{},
				ImageURLs:        []string{"https://example.com/image.jpg"},
			},
			expected: "\nImages:https://example.com/image.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.response.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAgentResponse_ReplacementRulesIntegration(t *testing.T) {
	// Test that demonstrates how replacement rules would be used in practice
	response := AgentResponse{
		Content: "Hello PERSON_001, your meeting with PERSON_002 is scheduled for DATE_001 at LOCATION_001",
		ReplacementRules: map[string]string{
			"PERSON_001":   "John Smith",
			"PERSON_002":   "Jane Doe",
			"DATE_001":     "2025-07-15",
			"LOCATION_001": "Conference Room A",
		},
	}

	// Test that we can extract the replacement rules
	rules := response.ReplacementRules
	require.NotNil(t, rules)
	assert.Equal(t, 4, len(rules))

	// Test that we can deanonymize the content using the rules
	deanonymizedContent := deanonymizeContent(response.Content, rules)
	expected := "Hello John Smith, your meeting with Jane Doe is scheduled for 2025-07-15 at Conference Room A"
	assert.Equal(t, expected, deanonymizedContent)

	// Test that the original content is still anonymized
	assert.Contains(t, response.Content, "PERSON_001")
	assert.Contains(t, response.Content, "PERSON_002")
	assert.Contains(t, response.Content, "DATE_001")
	assert.Contains(t, response.Content, "LOCATION_001")
}

// Helper function to test if response has anonymized content.
func hasAnonymizedContent(response AgentResponse) bool {
	return len(response.ReplacementRules) > 0
}

// Helper function to deanonymize content (for testing purposes).
func deanonymizeContent(content string, rules map[string]string) string {
	result := content
	for token, originalText := range rules {
		// Simple string replacement for testing
		result = strings.ReplaceAll(result, token, originalText)
	}
	return result
}

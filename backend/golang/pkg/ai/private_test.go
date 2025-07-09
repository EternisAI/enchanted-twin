package ai

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
)

func TestPrivateCompletionsMockAnonymizer(t *testing.T) {
	logger := log.New(nil)

	anonymizer := InitMockAnonymizer(0, true, logger)

	ctx := context.Background()
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Hello John, this is a test message from Alice at OpenAI."),
	}

	interruptChan := make(chan struct{})
	defer close(interruptChan)

	anonymizedMessages, _, rules, err := anonymizer.AnonymizeMessages(ctx, "", messages, nil, interruptChan)
	if err != nil {
		t.Fatalf("Anonymization failed: %v", err)
	}

	if len(rules) == 0 {
		t.Error("Expected some anonymization rules, got none")
	}

	if len(anonymizedMessages) != len(messages) {
		t.Errorf("Expected %d messages, got %d", len(messages), len(anonymizedMessages))
	}

	content := "Hello John, this is a test message from Alice at OpenAI."
	anonymized := "Hello PERSON_001, this is a test message from PERSON_003 at COMPANY_001."
	restored := anonymizer.DeAnonymize(anonymized, rules)
	if restored != content {
		t.Errorf("De-anonymization failed. Expected: %q, got: %q", content, restored)
	}

	t.Logf("Messages processed: %d", len(anonymizedMessages))
	t.Logf("Rules generated: %v", rules)
	t.Logf("Restored: %s", restored)
}

func TestMockAnonymizerDelay(t *testing.T) {
	logger := log.New(nil)

	// Reset singleton to ensure we get the correct delay configuration
	ResetMockAnonymizerForTesting()
	anonymizer := InitMockAnonymizer(10*time.Millisecond, true, logger)

	ctx := context.Background()
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Test message with John and Alice"),
	}

	interruptChan := make(chan struct{})
	defer close(interruptChan)

	start := time.Now()
	_, _, _, err := anonymizer.AnonymizeMessages(ctx, "", messages, nil, interruptChan)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Anonymization failed: %v", err)
	}

	if elapsed < 10*time.Millisecond {
		t.Errorf("Expected at least 10ms delay, but took %v", elapsed)
	}

	t.Logf("Anonymization took %v (expected >=10ms)", elapsed)
}

func TestPrivateCompletionsServiceMessageInterruption(t *testing.T) {
	logger := log.New(nil)

	// Reset singleton to ensure we get the correct delay configuration
	ResetMockAnonymizerForTesting()
	anonymizer := InitMockAnonymizer(100*time.Millisecond, true, logger)

	mockService := &mockCompletionsService{
		response: openai.ChatCompletionMessage{
			Content: "This response shouldn't be reached",
		},
	}

	privateService, err := NewPrivateCompletionsService(PrivateCompletionsConfig{
		CompletionsService: mockService,
		Anonymizer:         anonymizer,
		ExecutorWorkers:    1,
		Logger:             logger,
	})
	if err != nil {
		t.Fatalf("Failed to create private service: %v", err)
	}

	ctx := context.Background()
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Test message that should be interrupted"),
	}

	start := time.Now()
	_, err = privateService.Completions(ctx, messages, nil, "test-model", Background)
	elapsed := time.Since(start)

	t.Logf("Private completion took %v", elapsed)

	if err != nil && strings.Contains(err.Error(), "interrupted") {
		t.Logf("Successfully interrupted after %v", elapsed)
		if elapsed >= 100*time.Millisecond {
			t.Errorf("Expected interruption before full delay, but took %v", elapsed)
		}
	} else if err == nil {
		if elapsed < 100*time.Millisecond {
			t.Errorf("Expected at least 100ms delay, but took %v", elapsed)
		}
		t.Logf("Completed without interruption after %v", elapsed)
	} else {
		t.Fatalf("Unexpected error: %v", err)
	}
}

type mockCompletionsService struct {
	response openai.ChatCompletionMessage
	err      error
}

func (m *mockCompletionsService) Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string, priority Priority) (PrivateCompletionResult, error) {
	if m.err != nil {
		return PrivateCompletionResult{}, m.err
	}
	return PrivateCompletionResult{
		Message:          m.response,
		ReplacementRules: make(map[string]string),
	}, nil
}

// Enhanced mock that captures what was sent to the LLM.
type capturingMockCompletionsService struct {
	response         openai.ChatCompletionMessage
	err              error
	capturedMessages []openai.ChatCompletionMessageParamUnion
}

func (m *capturingMockCompletionsService) Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string, priority Priority) (PrivateCompletionResult, error) {
	// Capture what was sent to the LLM for verification
	m.capturedMessages = make([]openai.ChatCompletionMessageParamUnion, len(messages))
	copy(m.capturedMessages, messages)

	if m.err != nil {
		return PrivateCompletionResult{}, m.err
	}
	return PrivateCompletionResult{
		Message:          m.response,
		ReplacementRules: make(map[string]string),
	}, nil
}

func TestNewPrivateCompletionsServiceValidation(t *testing.T) {
	logger := log.New(nil)

	// Test with nil CompletionsService
	_, err := NewPrivateCompletionsService(PrivateCompletionsConfig{
		CompletionsService: nil,
		Anonymizer:         InitMockAnonymizer(0, true, logger),
		Logger:             logger,
	})
	if err == nil || !strings.Contains(err.Error(), "completionsService is required") {
		t.Errorf("Expected CompletionsService validation error, got: %v", err)
	}

	// Test with nil Anonymizer
	_, err = NewPrivateCompletionsService(PrivateCompletionsConfig{
		CompletionsService: &mockCompletionsService{},
		Anonymizer:         nil,
		Logger:             logger,
	})
	if err == nil || !strings.Contains(err.Error(), "anonymizer is required") {
		t.Errorf("Expected Anonymizer validation error, got: %v", err)
	}

	// Test with nil Logger
	_, err = NewPrivateCompletionsService(PrivateCompletionsConfig{
		CompletionsService: &mockCompletionsService{},
		Anonymizer:         InitMockAnonymizer(0, true, logger),
		Logger:             nil,
	})
	if err == nil || !strings.Contains(err.Error(), "logger is required") {
		t.Errorf("Expected Logger validation error, got: %v", err)
	}

	// Test with valid config
	service, err := NewPrivateCompletionsService(PrivateCompletionsConfig{
		CompletionsService: &mockCompletionsService{},
		Anonymizer:         InitMockAnonymizer(0, true, logger),
		ExecutorWorkers:    1,
		Logger:             logger,
	})
	if err != nil {
		t.Errorf("Expected valid config to succeed, got error: %v", err)
	}
	if service == nil {
		t.Error("Expected service to be created")
	}
	if service != nil {
		service.Shutdown()
	}
}

func TestPrivateCompletionsE2EAnonymizationFlow(t *testing.T) {
	logger := log.New(nil)

	// Reset singleton to ensure clean state
	ResetMockAnonymizerForTesting()
	anonymizer := InitMockAnonymizer(0, true, logger) // No delay for faster test

	// Create capturing mock that will record what the LLM sees
	mockLLM := &capturingMockCompletionsService{
		response: openai.ChatCompletionMessage{
			Content: "Hello PERSON_001! I understand you work at COMPANY_001 in LOCATION_006. That's great!",
		},
	}

	// Create private completions service with our mocks
	privateService, err := NewPrivateCompletionsService(PrivateCompletionsConfig{
		CompletionsService: mockLLM,
		Anonymizer:         anonymizer,
		ExecutorWorkers:    1,
		Logger:             logger,
	})
	if err != nil {
		t.Fatalf("Failed to create private service: %v", err)
	}
	defer privateService.Shutdown()

	ctx := context.Background()

	// Original user message containing sensitive information
	originalMessage := "Hello John! I heard you work at OpenAI in San Francisco. Can you tell me about your projects?"

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage(originalMessage),
	}

	// Execute the full e2e flow
	result, err := privateService.Completions(ctx, messages, nil, "test-model", Background)
	if err != nil {
		t.Fatalf("Private completions failed: %v", err)
	}

	// === VERIFICATION 1: Original message was anonymized before sending to LLM ===
	if len(mockLLM.capturedMessages) != 1 {
		t.Fatalf("Expected 1 message sent to LLM, got %d", len(mockLLM.capturedMessages))
	}

	// Extract the content that was sent to the LLM
	sentToLLMBytes, err := json.Marshal(mockLLM.capturedMessages[0])
	if err != nil {
		t.Fatalf("Failed to marshal captured message: %v", err)
	}

	var sentToLLMMap map[string]interface{}
	if err := json.Unmarshal(sentToLLMBytes, &sentToLLMMap); err != nil {
		t.Fatalf("Failed to unmarshal captured message: %v", err)
	}

	anonymizedContent, ok := sentToLLMMap["content"].(string)
	if !ok {
		t.Fatalf("Expected string content in captured message")
	}

	// Verify that sensitive data was anonymized in what was sent to LLM
	if strings.Contains(anonymizedContent, "John") {
		t.Errorf("LLM received un-anonymized name 'John': %s", anonymizedContent)
	}
	if strings.Contains(anonymizedContent, "OpenAI") {
		t.Errorf("LLM received un-anonymized company 'OpenAI': %s", anonymizedContent)
	}
	if strings.Contains(anonymizedContent, "San Francisco") {
		t.Errorf("LLM received un-anonymized location 'San Francisco': %s", anonymizedContent)
	}

	// Verify that anonymized tokens were used instead
	if !strings.Contains(anonymizedContent, "PERSON_001") {
		t.Errorf("LLM did not receive anonymized name token 'PERSON_001': %s", anonymizedContent)
	}
	if !strings.Contains(anonymizedContent, "COMPANY_001") {
		t.Errorf("LLM did not receive anonymized company token 'COMPANY_001': %s", anonymizedContent)
	}
	if !strings.Contains(anonymizedContent, "LOCATION_006") {
		t.Errorf("LLM did not receive anonymized location token 'LOCATION_006': %s", anonymizedContent)
	}

	t.Logf("Original message: %s", originalMessage)
	t.Logf("Anonymized sent to LLM: %s", anonymizedContent)

	// === VERIFICATION 2: LLM response was de-anonymized before returning to user ===
	finalResponse := result.Message.Content

	// Verify that the response was de-anonymized
	if strings.Contains(finalResponse, "PERSON_001") {
		t.Errorf("Final response still contains anonymized token 'PERSON_001': %s", finalResponse)
	}
	if strings.Contains(finalResponse, "COMPANY_001") {
		t.Errorf("Final response still contains anonymized token 'COMPANY_001': %s", finalResponse)
	}
	if strings.Contains(finalResponse, "LOCATION_006") {
		t.Errorf("Final response still contains anonymized token 'LOCATION_006': %s", finalResponse)
	}

	// Verify that original terms were restored
	if !strings.Contains(finalResponse, "John") {
		t.Errorf("Final response missing de-anonymized name 'John': %s", finalResponse)
	}
	if !strings.Contains(finalResponse, "OpenAI") {
		t.Errorf("Final response missing de-anonymized company 'OpenAI': %s", finalResponse)
	}
	if !strings.Contains(finalResponse, "San Francisco") {
		t.Errorf("Final response missing de-anonymized location 'San Francisco': %s", finalResponse)
	}

	t.Logf("Final de-anonymized response: %s", finalResponse)

	// === VERIFICATION 3: Replacement rules were captured correctly ===
	if len(result.ReplacementRules) == 0 {
		t.Error("Expected replacement rules to be returned, got none")
	}

	expectedRules := map[string]string{
		"PERSON_001":   "John",
		"COMPANY_001":  "OpenAI",
		"LOCATION_006": "San Francisco",
	}

	for token, expectedOriginal := range expectedRules {
		if actual, exists := result.ReplacementRules[token]; !exists {
			t.Errorf("Missing replacement rule for token '%s'", token)
		} else if actual != expectedOriginal {
			t.Errorf("Wrong replacement rule for '%s': expected '%s', got '%s'", token, expectedOriginal, actual)
		}
	}

	t.Logf("Replacement rules: %v", result.ReplacementRules)

	// === VERIFICATION 4: End-to-end privacy guarantee ===
	t.Log("E2E Privacy Verification PASSED:")
	t.Log("  1. Sensitive data was anonymized before LLM")
	t.Log("  2. LLM only saw anonymized tokens")
	t.Log("  3. Response was de-anonymized before user")
	t.Log("  4. Replacement rules preserved for full round-trip")
	t.Log("  5. Full system integration through microscheduler worked")
}

func TestPrivateCompletionsE2EWithToolCalls(t *testing.T) {
	logger := log.New(nil)

	// Reset singleton to ensure clean state
	ResetMockAnonymizerForTesting()
	anonymizer := InitMockAnonymizer(0, true, logger) // No delay for faster test

	// Create capturing mock that will record what the LLM sees and responds with tool calls
	mockLLM := &capturingMockCompletionsService{
		response: openai.ChatCompletionMessage{
			Content: "I'll help you find information about PERSON_001 at COMPANY_001.",
			ToolCalls: []openai.ChatCompletionMessageToolCall{
				{
					ID:   "call_001",
					Type: "function",
					Function: openai.ChatCompletionMessageToolCallFunction{
						Name:      "search_employee",
						Arguments: `{"name": "PERSON_001", "company": "COMPANY_001", "location": "LOCATION_006"}`,
					},
				},
			},
		},
	}

	// Create private completions service with our mocks
	privateService, err := NewPrivateCompletionsService(PrivateCompletionsConfig{
		CompletionsService: mockLLM,
		Anonymizer:         anonymizer,
		ExecutorWorkers:    1,
		Logger:             logger,
	})
	if err != nil {
		t.Fatalf("Failed to create private service: %v", err)
	}
	defer privateService.Shutdown()

	ctx := context.Background()

	// Original user message containing sensitive information
	originalMessage := "Please search for John Smith who works at OpenAI in San Francisco"

	// Define tool that LLM can use - with sensitive info in the tool definition
	tools := []openai.ChatCompletionToolParam{
		{
			Type: "function",
			Function: openai.FunctionDefinitionParam{
				Name:        "search_employee",
				Description: param.Opt[string]{Value: "Search for employee information"},
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type":        "string",
							"description": "Employee name (e.g., John Smith)",
						},
						"company": map[string]interface{}{
							"type":        "string",
							"description": "Company name (e.g., OpenAI)",
						},
						"location": map[string]interface{}{
							"type":        "string",
							"description": "Location (e.g., San Francisco)",
						},
					},
					"required": []string{"name", "company"},
				},
			},
		},
	}

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage(originalMessage),
	}

	// Execute the full e2e flow with tool calls
	result, err := privateService.Completions(ctx, messages, tools, "test-model", Background)
	if err != nil {
		t.Fatalf("Private completions failed: %v", err)
	}

	// === VERIFICATION 1: Original message was anonymized before sending to LLM ===
	if len(mockLLM.capturedMessages) != 1 {
		t.Fatalf("Expected 1 message sent to LLM, got %d", len(mockLLM.capturedMessages))
	}

	// Extract the content that was sent to the LLM
	sentToLLMBytes, err := json.Marshal(mockLLM.capturedMessages[0])
	if err != nil {
		t.Fatalf("Failed to marshal captured message: %v", err)
	}

	var sentToLLMMap map[string]interface{}
	if err := json.Unmarshal(sentToLLMBytes, &sentToLLMMap); err != nil {
		t.Fatalf("Failed to unmarshal captured message: %v", err)
	}

	anonymizedContent, ok := sentToLLMMap["content"].(string)
	if !ok {
		t.Fatalf("Expected string content in captured message")
	}

	// Verify that sensitive data was anonymized in what was sent to LLM
	if strings.Contains(anonymizedContent, "John") {
		t.Errorf("LLM received un-anonymized name 'John': %s", anonymizedContent)
	}
	if strings.Contains(anonymizedContent, "OpenAI") {
		t.Errorf("LLM received un-anonymized company 'OpenAI': %s", anonymizedContent)
	}
	if strings.Contains(anonymizedContent, "San Francisco") {
		t.Errorf("LLM received un-anonymized location 'San Francisco': %s", anonymizedContent)
	}

	// Verify that anonymized tokens were used instead
	if !strings.Contains(anonymizedContent, "PERSON_001") {
		t.Errorf("LLM did not receive anonymized name token 'PERSON_001': %s", anonymizedContent)
	}
	if !strings.Contains(anonymizedContent, "COMPANY_001") {
		t.Errorf("LLM did not receive anonymized company token 'COMPANY_001': %s", anonymizedContent)
	}
	if !strings.Contains(anonymizedContent, "LOCATION_006") {
		t.Errorf("LLM did not receive anonymized location token 'LOCATION_006': %s", anonymizedContent)
	}

	t.Logf("Original message: %s", originalMessage)
	t.Logf("Anonymized sent to LLM: %s", anonymizedContent)

	// === VERIFICATION 2: LLM response content was de-anonymized ===
	finalResponse := result.Message.Content

	// Verify that the response content was de-anonymized
	if strings.Contains(finalResponse, "PERSON_001") {
		t.Errorf("Final response still contains anonymized token 'PERSON_001': %s", finalResponse)
	}
	if strings.Contains(finalResponse, "COMPANY_001") {
		t.Errorf("Final response still contains anonymized token 'COMPANY_001': %s", finalResponse)
	}

	// Verify that original terms were restored in response content
	if !strings.Contains(finalResponse, "John") {
		t.Errorf("Final response missing de-anonymized name 'John': %s", finalResponse)
	}
	if !strings.Contains(finalResponse, "OpenAI") {
		t.Errorf("Final response missing de-anonymized company 'OpenAI': %s", finalResponse)
	}

	t.Logf("Final de-anonymized response: %s", finalResponse)

	// === VERIFICATION 3: Tool calls were present and arguments were de-anonymized ===
	if len(result.Message.ToolCalls) == 0 {
		t.Fatalf("Expected tool calls in response, got none")
	}

	toolCall := result.Message.ToolCalls[0]
	if toolCall.Function.Name != "search_employee" {
		t.Errorf("Expected tool call name 'search_employee', got '%s'", toolCall.Function.Name)
	}

	// Parse the tool call arguments
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		t.Fatalf("Failed to unmarshal tool call arguments: %v", err)
	}

	// Verify that tool call arguments were de-anonymized
	if name, ok := args["name"].(string); ok {
		if name != "John" {
			t.Errorf("Expected de-anonymized name 'John' in tool call, got '%s'", name)
		}
		if strings.Contains(name, "PERSON_001") {
			t.Errorf("Tool call arguments still contain anonymized token 'PERSON_001': %s", name)
		}
	} else {
		t.Error("Expected 'name' field in tool call arguments")
	}

	if company, ok := args["company"].(string); ok {
		if company != "OpenAI" {
			t.Errorf("Expected de-anonymized company 'OpenAI' in tool call, got '%s'", company)
		}
		if strings.Contains(company, "COMPANY_001") {
			t.Errorf("Tool call arguments still contain anonymized token 'COMPANY_001': %s", company)
		}
	} else {
		t.Error("Expected 'company' field in tool call arguments")
	}

	if location, ok := args["location"].(string); ok {
		if location != "San Francisco" {
			t.Errorf("Expected de-anonymized location 'San Francisco' in tool call, got '%s'", location)
		}
		if strings.Contains(location, "LOCATION_006") {
			t.Errorf("Tool call arguments still contain anonymized token 'LOCATION_006': %s", location)
		}
	} else {
		t.Error("Expected 'location' field in tool call arguments")
	}

	t.Logf("Tool call arguments: %s", toolCall.Function.Arguments)

	// === VERIFICATION 4: Replacement rules were captured correctly ===
	if len(result.ReplacementRules) == 0 {
		t.Error("Expected replacement rules to be returned, got none")
	}

	expectedRules := map[string]string{
		"PERSON_001":   "John",
		"COMPANY_001":  "OpenAI",
		"LOCATION_006": "San Francisco",
	}

	for token, expectedOriginal := range expectedRules {
		if actual, exists := result.ReplacementRules[token]; !exists {
			t.Errorf("Missing replacement rule for token '%s'", token)
		} else if actual != expectedOriginal {
			t.Errorf("Wrong replacement rule for '%s': expected '%s', got '%s'", token, expectedOriginal, actual)
		}
	}

	t.Logf("Replacement rules: %v", result.ReplacementRules)

	// === VERIFICATION 5: End-to-end privacy guarantee with tool calls ===
	t.Log("E2E Privacy Verification with Tool Calls PASSED:")
	t.Log("  1. Sensitive data was anonymized before LLM")
	t.Log("  2. LLM only saw anonymized tokens")
	t.Log("  3. Response content was de-anonymized before user")
	t.Log("  4. Tool call arguments were de-anonymized before user")
	t.Log("  5. Replacement rules preserved for full round-trip")
	t.Log("  6. Tool calls work correctly with anonymization/deanonymization")
}

func TestMockAnonymizerLongerReplacementFirst(t *testing.T) {
	logger := log.New(nil)

	// Reset singleton to ensure clean state
	ResetMockAnonymizerForTesting()

	// Create custom replacements where shorter string is subset of longer string
	customReplacements := map[string]string{
		"Ivan":        "ANON_2",
		"Ivan Ivanov": "ANON_1",
		"John":        "PERSON_001",
		"John Smith":  "PERSON_002",
	}

	// Create mock anonymizer with custom replacements
	mockAnonymizer := &MockAnonymizer{
		Delay:                  0,
		PredefinedReplacements: customReplacements,
		requestChan:            make(chan anonymizationRequest, 10),
		done:                   make(chan struct{}),
		logger:                 logger,
	}

	// Start processor
	go mockAnonymizer.processRequests()
	defer mockAnonymizer.Shutdown()

	ctx := context.Background()
	interruptChan := make(chan struct{})
	defer close(interruptChan)

	// Test anonymization with longer string first
	testCases := []struct {
		input    string
		expected string
	}{
		{
			input:    "Hello Ivan Ivanov, this is from Ivan",
			expected: "Hello ANON_1, this is from ANON_2",
		},
		{
			input:    "Meet John Smith and John",
			expected: "Meet PERSON_002 and PERSON_001",
		},
		{
			input:    "Ivan Ivanov and Ivan work together",
			expected: "ANON_1 and ANON_2 work together",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			messages := []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage(tc.input),
			}

			anonymizedMessages, _, rules, err := mockAnonymizer.AnonymizeMessages(ctx, "", messages, nil, interruptChan)
			if err != nil {
				t.Fatalf("Anonymization failed: %v", err)
			}

			// Extract anonymized content
			messageBytes, err := json.Marshal(anonymizedMessages[0])
			if err != nil {
				t.Fatalf("Failed to marshal anonymized message: %v", err)
			}

			var messageMap map[string]interface{}
			if err := json.Unmarshal(messageBytes, &messageMap); err != nil {
				t.Fatalf("Failed to unmarshal anonymized message: %v", err)
			}

			anonymizedContent, ok := messageMap["content"].(string)
			if !ok {
				t.Fatalf("Expected string content in message")
			}
			if anonymizedContent != tc.expected {
				t.Errorf("Expected: %q, got: %q", tc.expected, anonymizedContent)
			}

			// Test de-anonymization
			deAnonymized := mockAnonymizer.DeAnonymize(anonymizedContent, rules)
			if deAnonymized != tc.input {
				t.Errorf("De-anonymization failed. Expected: %q, got: %q", tc.input, deAnonymized)
			}

			t.Logf("Input: %s", tc.input)
			t.Logf("Anonymized: %s", anonymizedContent)
			t.Logf("De-anonymized: %s", deAnonymized)
			t.Logf("Rules: %v", rules)
		})
	}
}

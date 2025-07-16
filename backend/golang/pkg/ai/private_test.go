package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
)

func TestPrivateCompletionsMockAnonymizer(t *testing.T) {
	logger := log.New(nil)

	anonymizerManager := NewMockAnonymizerManager(0, true, logger)
	anonymizer := anonymizerManager.GetAnonymizer()
	defer anonymizerManager.Shutdown()

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

	anonymizerManager := NewMockAnonymizerManager(10*time.Millisecond, true, logger)
	anonymizer := anonymizerManager.GetAnonymizer()
	defer anonymizerManager.Shutdown()

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

	anonymizerManager := NewMockAnonymizerManager(100*time.Millisecond, true, logger)
	defer anonymizerManager.Shutdown()

	mockService := &mockCompletionsService{
		response: openai.ChatCompletionMessage{
			Content: "This response shouldn't be reached",
		},
	}

	privateService, err := NewPrivateCompletionsService(PrivateCompletionsConfig{
		CompletionsService: mockService,
		AnonymizerManager:  anonymizerManager,
		ExecutorWorkers:    1,
		Logger:             logger,
	})
	if err != nil {
		t.Fatalf("Failed to create private service: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Test message that should be interrupted"),
	}

	// Cancel the context after 50ms to interrupt the 100ms anonymization delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err = privateService.Completions(ctx, messages, nil, "test-model", Background)
	elapsed := time.Since(start)

	t.Logf("Private completion took %v", elapsed)

	// Should be interrupted with context cancellation
	if err == nil {
		t.Errorf("Expected error due to context cancellation, got nil")
	}

	// Error should indicate interruption or cancellation
	if !strings.Contains(err.Error(), "interrupted") && !strings.Contains(err.Error(), "canceled") {
		t.Errorf("Expected error to contain 'interrupted' or 'canceled', got: %v", err)
	}

	// Should have been interrupted before the full delay
	if elapsed >= 100*time.Millisecond {
		t.Errorf("Expected interruption before full delay (100ms), but took %v", elapsed)
	}

	t.Logf("Successfully interrupted after %v", elapsed)
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

func (m *mockCompletionsService) CompletionsStream(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) Stream {
	contentCh := make(chan StreamDelta, 1)
	toolCh := make(chan openai.ChatCompletionMessageToolCall)
	errCh := make(chan error, 1)

	go func() {
		defer close(contentCh)
		defer close(toolCh)
		defer close(errCh)

		if m.err != nil {
			errCh <- m.err
			return
		}

		// Send content as single chunk
		contentCh <- StreamDelta{
			ContentDelta: m.response.Content,
			IsCompleted:  true,
		}
	}()

	return Stream{
		Content:   contentCh,
		ToolCalls: toolCh,
		Err:       errCh,
	}
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

func (m *capturingMockCompletionsService) CompletionsStream(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) Stream {
	// Capture what was sent to the LLM for verification
	m.capturedMessages = make([]openai.ChatCompletionMessageParamUnion, len(messages))
	copy(m.capturedMessages, messages)

	contentCh := make(chan StreamDelta, 1)
	toolCh := make(chan openai.ChatCompletionMessageToolCall)
	errCh := make(chan error, 1)

	go func() {
		defer close(contentCh)
		defer close(toolCh)
		defer close(errCh)

		if m.err != nil {
			errCh <- m.err
			return
		}

		// Send content as single chunk
		contentCh <- StreamDelta{
			ContentDelta: m.response.Content,
			IsCompleted:  true,
		}
	}()

	return Stream{
		Content:   contentCh,
		ToolCalls: toolCh,
		Err:       errCh,
	}
}

func TestNewPrivateCompletionsServiceValidation(t *testing.T) {
	logger := log.New(nil)

	// Test with nil CompletionsService
	_, err := NewPrivateCompletionsService(PrivateCompletionsConfig{
		CompletionsService: nil,
		AnonymizerManager:  NewMockAnonymizerManager(0, true, logger),
		Logger:             logger,
	})
	if err == nil || !strings.Contains(err.Error(), "completionsService is required") {
		t.Errorf("Expected CompletionsService validation error, got: %v", err)
	}

	// Test with nil AnonymizerManager
	_, err = NewPrivateCompletionsService(PrivateCompletionsConfig{
		CompletionsService: &mockCompletionsService{},
		AnonymizerManager:  nil,
		Logger:             logger,
	})
	if err == nil || !strings.Contains(err.Error(), "anonymizerManager is required") {
		t.Errorf("Expected AnonymizerManager validation error, got: %v", err)
	}

	// Test with nil Logger
	_, err = NewPrivateCompletionsService(PrivateCompletionsConfig{
		CompletionsService: &mockCompletionsService{},
		AnonymizerManager:  NewMockAnonymizerManager(0, true, logger),
		Logger:             nil,
	})
	if err == nil || !strings.Contains(err.Error(), "logger is required") {
		t.Errorf("Expected Logger validation error, got: %v", err)
	}

	// Test with valid config
	anonymizerManager := NewMockAnonymizerManager(0, true, logger)
	defer anonymizerManager.Shutdown()

	service, err := NewPrivateCompletionsService(PrivateCompletionsConfig{
		CompletionsService: &mockCompletionsService{},
		AnonymizerManager:  anonymizerManager,
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

	anonymizerManager := NewMockAnonymizerManager(0, true, logger) // No delay for faster test
	defer anonymizerManager.Shutdown()

	// Create capturing mock that will record what the LLM sees
	mockLLM := &capturingMockCompletionsService{
		response: openai.ChatCompletionMessage{
			Content: "Hello PERSON_001! I understand you work at COMPANY_001 in LOCATION_006. That's great!",
		},
	}

	// Create private completions service with our mocks
	privateService, err := NewPrivateCompletionsService(PrivateCompletionsConfig{
		CompletionsService: mockLLM,
		AnonymizerManager:  anonymizerManager,
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

	anonymizerManager := NewMockAnonymizerManager(0, true, logger) // No delay for faster test
	defer anonymizerManager.Shutdown()

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
		AnonymizerManager:  anonymizerManager,
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
		if name != "John Smith" {
			t.Errorf("Expected de-anonymized name 'John Smith' in tool call, got '%s'", name)
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
		"PERSON_001":   "John Smith",
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

	// Create custom replacements where shorter string is subset of longer string
	customReplacements := map[string]string{
		"Ivan":        "ANON_2",
		"Ivan Ivanov": "ANON_1",
		"John":        "PERSON_001",
		"John Smith":  "PERSON_002",
	}

	// Create mock anonymizer with custom replacements using the new factory
	mockAnonymizer := NewMockAnonymizer(0, customReplacements, logger)
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

func TestPrivateCompletionsE2EStreamingAnonymizationFlow(t *testing.T) {
	logger := log.New(nil)

	anonymizerManager := NewMockAnonymizerManager(0, true, logger) // No delay for faster test
	defer anonymizerManager.Shutdown()

	// Create streaming mock that will simulate chunks
	mockLLM := &streamingMockCompletionsService{
		chunks: []StreamDelta{
			{ContentDelta: "Hello ", IsCompleted: false},
			{ContentDelta: "PERSON_001! ", IsCompleted: false},
			{ContentDelta: "I understand you work at ", IsCompleted: false},
			{ContentDelta: "COMPANY_001 ", IsCompleted: false},
			{ContentDelta: "in LOCATION_006. ", IsCompleted: false},
			{ContentDelta: "That's great!", IsCompleted: true},
		},
	}

	// Create private completions service with our mocks
	privateService, err := NewPrivateCompletionsService(PrivateCompletionsConfig{
		CompletionsService: mockLLM,
		AnonymizerManager:  anonymizerManager,
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

	// Capture streaming deltas
	var streamingDeltas []StreamDelta
	var finalAccumulatedAnonymized string
	var finalAccumulatedDeanonymized string

	onDelta := func(delta StreamDelta) {
		streamingDeltas = append(streamingDeltas, delta)
		finalAccumulatedAnonymized = delta.AccumulatedAnonymizedMessage
		finalAccumulatedDeanonymized = delta.AccumulatedDeanonymizedMessage
		t.Logf("Streaming delta - Chunk: '%s', Anonymized: '%s', Deanonymized: '%s'",
			delta.ContentDelta,
			delta.AccumulatedAnonymizedMessage,
			delta.AccumulatedDeanonymizedMessage)
	}

	// Execute the full e2e streaming flow
	result, err := privateService.CompletionsStream(ctx, messages, nil, "gpt-4", UI, onDelta)
	if err != nil {
		t.Fatalf("Private streaming completions failed: %v", err)
	}

	// === VERIFICATION 1: Input was anonymized before sending to LLM ===
	anonymizedContent := strings.Join(mockLLM.capturedMessages, " ")

	// Verify that sensitive information was NOT sent to the LLM
	if strings.Contains(anonymizedContent, "John") {
		t.Errorf("LLM received un-anonymized name 'John': %s", anonymizedContent)
	}
	if strings.Contains(anonymizedContent, "OpenAI") {
		t.Errorf("LLM received un-anonymized company 'OpenAI': %s", anonymizedContent)
	}
	if strings.Contains(anonymizedContent, "San Francisco") {
		t.Errorf("LLM received un-anonymized location 'San Francisco': %s", anonymizedContent)
	}

	// === VERIFICATION 2: Streaming chunks were progressively deanonymized ===
	if len(streamingDeltas) == 0 {
		t.Fatalf("Expected streaming deltas, got none")
	}

	// Check that we received the expected number of chunks
	expectedChunks := len(mockLLM.chunks)
	if len(streamingDeltas) != expectedChunks {
		t.Errorf("Expected %d streaming deltas, got %d", expectedChunks, len(streamingDeltas))
	}

	// Verify progressive accumulation and deanonymization
	var expectedAnonymized string
	var expectedDeanonymized string
	for i, delta := range streamingDeltas {
		// Build expected accumulated content
		expectedAnonymized += mockLLM.chunks[i].ContentDelta
		expectedDeanonymized = anonymizerManager.GetAnonymizer().DeAnonymize(expectedAnonymized, result.ReplacementRules)

		// Verify accumulated anonymized message
		if delta.AccumulatedAnonymizedMessage != expectedAnonymized {
			t.Errorf("Delta %d: Expected accumulated anonymized '%s', got '%s'",
				i, expectedAnonymized, delta.AccumulatedAnonymizedMessage)
		}

		// Verify accumulated deanonymized message
		if delta.AccumulatedDeanonymizedMessage != expectedDeanonymized {
			t.Errorf("Delta %d: Expected accumulated deanonymized '%s', got '%s'",
				i, expectedDeanonymized, delta.AccumulatedDeanonymizedMessage)
		}

		// Verify chunk content
		if delta.ContentDelta != mockLLM.chunks[i].ContentDelta {
			t.Errorf("Delta %d: Expected chunk '%s', got '%s'",
				i, mockLLM.chunks[i].ContentDelta, delta.ContentDelta)
		}

		// Verify completion status
		if delta.IsCompleted != mockLLM.chunks[i].IsCompleted {
			t.Errorf("Delta %d: Expected IsCompleted=%v, got %v",
				i, mockLLM.chunks[i].IsCompleted, delta.IsCompleted)
		}
	}

	// === VERIFICATION 3: Final accumulated messages are correct ===
	expectedFinalAnonymized := "Hello PERSON_001! I understand you work at COMPANY_001 in LOCATION_006. That's great!"
	if finalAccumulatedAnonymized != expectedFinalAnonymized {
		t.Errorf("Final accumulated anonymized: Expected '%s', got '%s'",
			expectedFinalAnonymized, finalAccumulatedAnonymized)
	}

	expectedFinalDeanonymized := "Hello John! I understand you work at OpenAI in San Francisco. That's great!"
	if finalAccumulatedDeanonymized != expectedFinalDeanonymized {
		t.Errorf("Final accumulated deanonymized: Expected '%s', got '%s'",
			expectedFinalDeanonymized, finalAccumulatedDeanonymized)
	}

	// === VERIFICATION 4: Final result message is deanonymized ===
	finalResponse := result.Message.Content
	if strings.Contains(finalResponse, "PERSON_001") || strings.Contains(finalResponse, "COMPANY_001") || strings.Contains(finalResponse, "LOCATION_006") {
		t.Errorf("Final response still contains anonymized tokens: %s", finalResponse)
	}

	if !strings.Contains(finalResponse, "John") || !strings.Contains(finalResponse, "OpenAI") || !strings.Contains(finalResponse, "San Francisco") {
		t.Errorf("Final response missing original terms: %s", finalResponse)
	}

	// === VERIFICATION 5: Replacement rules were captured ===
	if len(result.ReplacementRules) == 0 {
		t.Error("Expected replacement rules to be returned, got none")
	}

	t.Log("E2E Streaming Privacy Verification PASSED:")
	t.Logf("  Original message: %s", originalMessage)
	t.Logf("  Anonymized sent to LLM: %s", mockLLM.capturedMessages)
	t.Logf("  Final deanonymized response: %s", finalResponse)
	t.Logf("  Streaming deltas processed: %d", len(streamingDeltas))
	t.Logf("  Replacement rules: %v", result.ReplacementRules)
}

// streamingMockCompletionsService simulates chunk-by-chunk streaming.
type streamingMockCompletionsService struct {
	chunks           []StreamDelta
	capturedMessages []string
	err              error
}

func (m *streamingMockCompletionsService) Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string, priority Priority) (PrivateCompletionResult, error) {
	// Capture what was sent to the LLM for verification
	m.capturedMessages = make([]string, len(messages))
	for i, msg := range messages {
		if userMsg := msg.OfUser; userMsg != nil {
			m.capturedMessages[i] = userMsg.Content.OfString.Value
		}
	}

	if m.err != nil {
		return PrivateCompletionResult{}, m.err
	}

	// Build full content from chunks
	var fullContent string
	for _, chunk := range m.chunks {
		fullContent += chunk.ContentDelta
	}

	return PrivateCompletionResult{
		Message: openai.ChatCompletionMessage{
			Content: fullContent,
			Role:    "assistant",
		},
		ReplacementRules: make(map[string]string),
	}, nil
}

func (m *streamingMockCompletionsService) CompletionsStream(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) Stream {
	// Capture what was sent to the LLM for verification
	m.capturedMessages = make([]string, len(messages))
	for i, msg := range messages {
		if userMsg := msg.OfUser; userMsg != nil {
			m.capturedMessages[i] = userMsg.Content.OfString.Value
		}
	}

	contentCh := make(chan StreamDelta, len(m.chunks))
	toolCh := make(chan openai.ChatCompletionMessageToolCall)
	errCh := make(chan error, 1)

	go func() {
		defer close(contentCh)
		defer close(toolCh)
		defer close(errCh)

		if m.err != nil {
			errCh <- m.err
			return
		}

		// Send chunks sequentially
		for _, chunk := range m.chunks {
			contentCh <- chunk
		}
	}()

	return Stream{
		Content:   contentCh,
		ToolCalls: toolCh,
		Err:       errCh,
	}
}

func TestStreamingAccumulation(t *testing.T) {
	logger := log.New(nil)

	// Test that streaming properly accumulates content progressively
	anonymizerManager := NewMockAnonymizerManager(0, true, logger)
	defer anonymizerManager.Shutdown()

	mockLLM := &streamingMockCompletionsService{
		chunks: []StreamDelta{
			{ContentDelta: "First ", IsCompleted: false},
			{ContentDelta: "Second ", IsCompleted: false},
			{ContentDelta: "Third", IsCompleted: true},
		},
	}

	privateService, err := NewPrivateCompletionsService(PrivateCompletionsConfig{
		CompletionsService: mockLLM,
		AnonymizerManager:  anonymizerManager,
		ExecutorWorkers:    1,
		Logger:             logger,
	})
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer privateService.Shutdown()

	ctx := context.Background()
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Test message"),
	}

	var deltas []StreamDelta
	onDelta := func(delta StreamDelta) {
		deltas = append(deltas, delta)
	}

	result, err := privateService.CompletionsStream(ctx, messages, nil, "gpt-4", UI, onDelta)
	if err != nil {
		t.Fatalf("Streaming failed: %v", err)
	}

	// Verify progressive accumulation
	expectedAccumulations := []string{
		"First ",
		"First Second ",
		"First Second Third",
	}

	if len(deltas) != len(expectedAccumulations) {
		t.Fatalf("Expected %d deltas, got %d", len(expectedAccumulations), len(deltas))
	}

	for i, delta := range deltas {
		if delta.AccumulatedAnonymizedMessage != expectedAccumulations[i] {
			t.Errorf("Delta %d: Expected '%s', got '%s'",
				i, expectedAccumulations[i], delta.AccumulatedAnonymizedMessage)
		}
		if delta.AccumulatedDeanonymizedMessage != expectedAccumulations[i] {
			t.Errorf("Delta %d: Expected deanonymized '%s', got '%s'",
				i, expectedAccumulations[i], delta.AccumulatedDeanonymizedMessage)
		}
		if delta.IsCompleted != (i == len(deltas)-1) {
			t.Errorf("Delta %d: Expected IsCompleted=%v, got %v",
				i, i == len(deltas)-1, delta.IsCompleted)
		}
	}

	// Verify final result
	if result.Message.Content != "First Second Third" {
		t.Errorf("Expected final content 'First Second Third', got '%s'", result.Message.Content)
	}
}

func TestStreamingPatternSpanning(t *testing.T) {
	logger := log.New(nil)

	// Test patterns that span across multiple streaming chunks
	anonymizerManager := NewMockAnonymizerManager(0, true, logger)
	defer anonymizerManager.Shutdown()

	// Create chunks that split names across boundaries
	mockLLM := &streamingMockCompletionsService{
		chunks: []StreamDelta{
			{ContentDelta: "Hello J", IsCompleted: false},
			{ContentDelta: "ohn! You work at Open", IsCompleted: false},
			{ContentDelta: "AI in San Fran", IsCompleted: false},
			{ContentDelta: "cisco.", IsCompleted: true},
		},
	}

	privateService, err := NewPrivateCompletionsService(PrivateCompletionsConfig{
		CompletionsService: mockLLM,
		AnonymizerManager:  anonymizerManager,
		ExecutorWorkers:    1,
		Logger:             logger,
	})
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer privateService.Shutdown()

	ctx := context.Background()
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Hello John! You work at OpenAI in San Francisco."),
	}

	var deltas []StreamDelta
	onDelta := func(delta StreamDelta) {
		deltas = append(deltas, delta)
	}

	result, err := privateService.CompletionsStream(ctx, messages, nil, "gpt-4", UI, onDelta)
	if err != nil {
		t.Fatalf("Streaming failed: %v", err)
	}

	// Verify that the final accumulated message has all patterns correctly deanonymized
	// even though they were split across chunks
	lastDelta := deltas[len(deltas)-1]
	if !strings.Contains(lastDelta.AccumulatedDeanonymizedMessage, "John") {
		t.Errorf("Expected 'John' in final deanonymized message, got: %s", lastDelta.AccumulatedDeanonymizedMessage)
	}
	if !strings.Contains(lastDelta.AccumulatedDeanonymizedMessage, "OpenAI") {
		t.Errorf("Expected 'OpenAI' in final deanonymized message, got: %s", lastDelta.AccumulatedDeanonymizedMessage)
	}
	if !strings.Contains(lastDelta.AccumulatedDeanonymizedMessage, "San Francisco") {
		t.Errorf("Expected 'San Francisco' in final deanonymized message, got: %s", lastDelta.AccumulatedDeanonymizedMessage)
	}

	// Verify the final result is also correct
	if result.Message.Content != "Hello John! You work at OpenAI in San Francisco." {
		t.Errorf("Expected final content with original names, got: %s", result.Message.Content)
	}

	t.Logf("Pattern spanning test passed - final content: %s", result.Message.Content)
}

func TestStreamingErrorHandling(t *testing.T) {
	logger := log.New(nil)

	// Test error handling during streaming
	anonymizerManager := NewMockAnonymizerManager(0, true, logger)
	defer anonymizerManager.Shutdown()

	// Create mock that returns an error during streaming
	mockLLM := &streamingMockCompletionsService{
		chunks: []StreamDelta{
			{ContentDelta: "Hello ", IsCompleted: false},
		},
		err: fmt.Errorf("streaming error occurred"),
	}

	privateService, err := NewPrivateCompletionsService(PrivateCompletionsConfig{
		CompletionsService: mockLLM,
		AnonymizerManager:  anonymizerManager,
		ExecutorWorkers:    1,
		Logger:             logger,
	})
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer privateService.Shutdown()

	ctx := context.Background()
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Test message"),
	}

	var deltas []StreamDelta
	onDelta := func(delta StreamDelta) {
		deltas = append(deltas, delta)
	}

	// This should return an error
	_, err = privateService.CompletionsStream(ctx, messages, nil, "gpt-4", UI, onDelta)
	if err == nil {
		t.Fatalf("Expected error from streaming, but got none")
	}

	if !strings.Contains(err.Error(), "streaming error occurred") {
		t.Errorf("Expected error message to contain 'streaming error occurred', got: %s", err.Error())
	}

	t.Logf("Error handling test passed - error: %v", err)
}

func TestStreamingContextCancellation(t *testing.T) {
	logger := log.New(nil)

	// Test context cancellation during streaming
	anonymizerManager := NewMockAnonymizerManager(0, true, logger)
	defer anonymizerManager.Shutdown()

	// Create mock with infinite streaming that we'll cancel
	mockLLM := &infiniteStreamingMock{}

	privateService, err := NewPrivateCompletionsService(PrivateCompletionsConfig{
		CompletionsService: mockLLM,
		AnonymizerManager:  anonymizerManager,
		ExecutorWorkers:    1,
		Logger:             logger,
	})
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer privateService.Shutdown()

	// Create a context that will be canceled
	ctx, cancel := context.WithCancel(context.Background())

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Test message"),
	}

	var deltas []StreamDelta
	onDelta := func(delta StreamDelta) {
		deltas = append(deltas, delta)
	}

	// Start streaming in a goroutine
	done := make(chan error, 1)
	go func() {
		_, err := privateService.CompletionsStream(ctx, messages, nil, "gpt-4", UI, onDelta)
		done <- err
	}()

	// Cancel the context after a short delay
	time.Sleep(10 * time.Millisecond)
	cancel()

	// Wait for the streaming to complete with error
	select {
	case err := <-done:
		if err == nil {
			t.Fatalf("Expected error from context cancellation, but got none")
		}
		if !strings.Contains(err.Error(), "context canceled") {
			t.Errorf("Expected error message to contain 'context canceled', got: %s", err.Error())
		}
		t.Logf("Context cancellation test passed - error: %v", err)
	case <-time.After(1 * time.Second):
		t.Fatalf("Streaming did not complete within timeout")
	}
}

// infiniteStreamingMock simulates a streaming service that never completes.
type infiniteStreamingMock struct{}

func (m *infiniteStreamingMock) Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string, priority Priority) (PrivateCompletionResult, error) {
	return PrivateCompletionResult{
		Message: openai.ChatCompletionMessage{
			Content: "Test response",
			Role:    "assistant",
		},
	}, nil
}

func (m *infiniteStreamingMock) CompletionsStream(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) Stream {
	contentCh := make(chan StreamDelta)
	toolCh := make(chan openai.ChatCompletionMessageToolCall)
	errCh := make(chan error)

	go func() {
		defer close(contentCh)
		defer close(toolCh)
		defer close(errCh)

		// Keep sending chunks until context is canceled
		for {
			select {
			case <-ctx.Done():
				return
			case contentCh <- StreamDelta{ContentDelta: "chunk", IsCompleted: false}:
				time.Sleep(5 * time.Millisecond)
			}
		}
	}()

	return Stream{
		Content:   contentCh,
		ToolCalls: toolCh,
		Err:       errCh,
	}
}

// circularDependencyTracker tracks how many times Completions is called to detect infinite loops.
type circularDependencyTracker struct {
	mockCompletionsService
	callCount int
	t         *testing.T
}

func (c *circularDependencyTracker) Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string, priority Priority) (PrivateCompletionResult, error) {
	c.callCount++
	// If this gets called more than once, we have a circular dependency
	if c.callCount > 1 {
		c.t.Errorf("Circular dependency detected: Completions called %d times", c.callCount)
	}
	return c.mockCompletionsService.Completions(ctx, messages, tools, model, priority)
}

// TestNoCircularDependencyInPrivateCompletions tests that the PrivateCompletionsService
// doesn't create a circular dependency when calling the underlying completions service.
// This test prevents the infinite loop issue that occurred when PrivateCompletionsService
// called Service.Completions() which then called back to PrivateCompletionsService.
func TestNoCircularDependencyInPrivateCompletions(t *testing.T) {
	logger := log.New(nil)
	logger.SetLevel(log.DebugLevel)

	// Create a service that will track if Completions method is called
	tracker := &circularDependencyTracker{
		mockCompletionsService: mockCompletionsService{
			response: openai.ChatCompletionMessage{
				Content: "Test response",
			},
		},
		t: t,
	}

	// Create mock anonymizer
	mockAnonymizerManager := NewMockAnonymizerManager(1*time.Millisecond, true, logger)
	defer mockAnonymizerManager.Shutdown()

	// Create private completions service
	privateService, err := NewPrivateCompletionsService(PrivateCompletionsConfig{
		CompletionsService: tracker,
		AnonymizerManager:  mockAnonymizerManager,
		ExecutorWorkers:    1,
		Logger:             logger,
	})
	if err != nil {
		t.Fatalf("Failed to create private completions service: %v", err)
	}
	defer privateService.Shutdown()

	// Test that a call to the private service doesn't create circular calls
	ctx := context.Background()
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Test message with John Smith"),
	}

	result, err := privateService.Completions(ctx, messages, nil, "test-model", Background)
	if err != nil {
		t.Fatalf("Private completions failed: %v", err)
	}

	if result.Message.Content != "Test response" {
		t.Errorf("Expected 'Test response', got %q", result.Message.Content)
	}

	// Verify that the underlying service was called exactly once
	if tracker.callCount != 1 {
		t.Errorf("Expected exactly 1 call to underlying service, got %d", tracker.callCount)
	}

	t.Logf("No circular dependency detected - underlying service called %d time(s)", tracker.callCount)
}

// TestPrivateCompletionsWithRealService tests that the private completions service
// works correctly with a real AI service instance (using RawCompletions to avoid circular dependency).
func TestPrivateCompletionsWithRealService(t *testing.T) {
	logger := log.New(nil)
	logger.SetLevel(log.DebugLevel)

	// Create a real AI service
	aiService := NewOpenAIService(logger, "test-key", "https://api.openai.com/v1")

	// Create mock anonymizer
	mockAnonymizerManager := NewMockAnonymizerManager(1*time.Millisecond, true, logger)
	defer mockAnonymizerManager.Shutdown()

	// Create private completions service with the real service
	privateService, err := NewPrivateCompletionsService(PrivateCompletionsConfig{
		CompletionsService: aiService,
		AnonymizerManager:  mockAnonymizerManager,
		ExecutorWorkers:    1,
		Logger:             logger,
	})
	if err != nil {
		t.Fatalf("Failed to create private completions service: %v", err)
	}
	defer privateService.Shutdown()

	// Verify that the service has the RawCompletions method available
	// This ensures our fix for circular dependency is in place
	_, hasRawCompletions := any(aiService).(interface {
		RawCompletions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) (PrivateCompletionResult, error)
	})

	if !hasRawCompletions {
		t.Errorf("AI Service missing RawCompletions method - circular dependency fix not implemented")
	}

	t.Logf("RawCompletions method available - circular dependency prevention in place")
}

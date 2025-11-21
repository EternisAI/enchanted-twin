package ai

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/packages/param"
)

// TestPrivateCompletionsWithMCPToolCalling tests that MCP-style tool calling
// works correctly with the PrivateCompletions service, mimicking the scenario
// where a user connects Google MCP server and requests functionality.
func TestPrivateCompletionsWithMCPToolCalling(t *testing.T) {
	logger := log.New(nil)

	anonymizerManager := NewMockAnonymizerManager(0, true, logger)
	defer anonymizerManager.Shutdown()

	// Create mock that simulates MCP tool calling response
	mockLLM := &capturingMockCompletionsService{
		response: openai.ChatCompletionMessage{
			Content: "I'll help you with that task using the available tools.",
			ToolCalls: []openai.ChatCompletionMessageToolCallUnion{
				{
					ID:   "call_mcp_001",
					Type: "function",
					Function: openai.ChatCompletionMessageFunctionToolCallFunction{
						Name:      "google_search",
						Arguments: `{"query": "PERSON_001 at COMPANY_001", "max_results": 5}`,
					},
				},
				{
					ID:   "call_mcp_002",
					Type: "function",
					Function: openai.ChatCompletionMessageFunctionToolCallFunction{
						Name:      "gmail_send",
						Arguments: `{"to": "PERSON_001@COMPANY_001.com", "subject": "Meeting with PERSON_003", "body": "Hi PERSON_001, can we schedule a meeting about the project at COMPANY_001?"}`,
					},
				},
			},
		},
	}

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

	// User message that would trigger MCP tool usage
	originalMessage := "Search for information about John Smith at OpenAI and send him an email about meeting with Alice Johnson"

	// Define MCP-style tools (Google search and Gmail)
	tools := []openai.ChatCompletionToolUnionParam{
		openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        "google_search",
			Description: param.Opt[string]{Value: "Search Google for information"},
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Search query",
					},
					"max_results": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of results",
					},
				},
				"required": []string{"query"},
			},
		}),
		openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        "gmail_send",
			Description: param.Opt[string]{Value: "Send an email via Gmail"},
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"to": map[string]interface{}{
						"type":        "string",
						"description": "Recipient email address",
					},
					"subject": map[string]interface{}{
						"type":        "string",
						"description": "Email subject",
					},
					"body": map[string]interface{}{
						"type":        "string",
						"description": "Email body",
					},
				},
				"required": []string{"to", "subject", "body"},
			},
		}),
	}

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage(originalMessage),
	}

	// Execute the completion with tools
	result, err := privateService.Completions(ctx, messages, tools, "gpt-4o-mini", Background)
	if err != nil {
		t.Fatalf("Private completions with MCP tools failed: %v", err)
	}

	// === VERIFICATION 1: Tool calls are present ===
	if len(result.Message.ToolCalls) == 0 {
		t.Fatalf("Expected tool calls in response, got none")
	}

	if len(result.Message.ToolCalls) != 2 {
		t.Fatalf("Expected 2 tool calls, got %d", len(result.Message.ToolCalls))
	}

	// === VERIFICATION 2: Tool call names are correct ===
	expectedToolNames := []string{"google_search", "gmail_send"}
	for i, toolCall := range result.Message.ToolCalls {
		if toolCall.Function.Name != expectedToolNames[i] {
			t.Errorf("Tool call %d: Expected name '%s', got '%s'",
				i, expectedToolNames[i], toolCall.Function.Name)
		}
	}

	// === VERIFICATION 3: Tool call arguments are de-anonymized ===

	// Check Google search tool call
	searchTool := result.Message.ToolCalls[0]
	var searchArgs map[string]interface{}
	if err := json.Unmarshal([]byte(searchTool.Function.Arguments), &searchArgs); err != nil {
		t.Fatalf("Failed to unmarshal search tool arguments: %v", err)
	}

	searchQuery, ok := searchArgs["query"].(string)
	if !ok {
		t.Fatalf("Expected string query in search tool arguments")
	}

	// Verify search query was de-anonymized
	if !strings.Contains(searchQuery, "John Smith") {
		t.Errorf("Search query missing de-anonymized name 'John Smith': %s", searchQuery)
	}
	if !strings.Contains(searchQuery, "OpenAI") {
		t.Errorf("Search query missing de-anonymized company 'OpenAI': %s", searchQuery)
	}
	if strings.Contains(searchQuery, "PERSON_001") || strings.Contains(searchQuery, "COMPANY_001") {
		t.Errorf("Search query still contains anonymized tokens: %s", searchQuery)
	}

	// Check Gmail tool call
	gmailTool := result.Message.ToolCalls[1]
	var gmailArgs map[string]interface{}
	if err := json.Unmarshal([]byte(gmailTool.Function.Arguments), &gmailArgs); err != nil {
		t.Fatalf("Failed to unmarshal gmail tool arguments: %v", err)
	}

	// Verify email recipient was de-anonymized
	if to, toOk := gmailArgs["to"].(string); toOk {
		if !strings.Contains(to, "John Smith") && !strings.Contains(to, "OpenAI") {
			t.Errorf("Email 'to' field missing de-anonymized values: %s", to)
		}
		if strings.Contains(to, "PERSON_001") || strings.Contains(to, "COMPANY_001") {
			t.Errorf("Email 'to' field still contains anonymized tokens: %s", to)
		}
	}

	// Verify email subject was de-anonymized
	if subject, subjectOk := gmailArgs["subject"].(string); subjectOk {
		if !strings.Contains(subject, "Alice Johnson") {
			t.Errorf("Email subject missing de-anonymized name 'Alice Johnson': %s", subject)
		}
		if strings.Contains(subject, "PERSON_003") {
			t.Errorf("Email subject still contains anonymized tokens: %s", subject)
		}
	}

	// Verify email body was de-anonymized
	if body, bodyOk := gmailArgs["body"].(string); bodyOk {
		if !strings.Contains(body, "John Smith") {
			t.Errorf("Email body missing de-anonymized name 'John Smith': %s", body)
		}
		if !strings.Contains(body, "OpenAI") {
			t.Errorf("Email body missing de-anonymized company 'OpenAI': %s", body)
		}
		if strings.Contains(body, "PERSON_001") || strings.Contains(body, "COMPANY_001") {
			t.Errorf("Email body still contains anonymized tokens: %s", body)
		}
	}

	// === VERIFICATION 4: Input was anonymized before sending to LLM ===
	if len(mockLLM.capturedMessages) != 1 {
		t.Fatalf("Expected 1 message sent to LLM, got %d", len(mockLLM.capturedMessages))
	}

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
	if strings.Contains(anonymizedContent, "John Smith") || strings.Contains(anonymizedContent, "OpenAI") || strings.Contains(anonymizedContent, "Alice Johnson") {
		t.Errorf("LLM received un-anonymized sensitive data: %s", anonymizedContent)
	}

	// === VERIFICATION 5: Response content was de-anonymized ===
	finalResponse := result.Message.Content
	if strings.Contains(finalResponse, "PERSON_001") || strings.Contains(finalResponse, "COMPANY_001") {
		t.Errorf("Final response still contains anonymized tokens: %s", finalResponse)
	}

	t.Logf("MCP Tool Calling Test PASSED:")
	t.Logf("  Original message: %s", originalMessage)
	t.Logf("  Anonymized sent to LLM: %s", anonymizedContent)
	t.Logf("  Final response: %s", finalResponse)
	t.Logf("  Tool calls: %d", len(result.Message.ToolCalls))
	t.Logf("  Search tool args: %s", searchTool.Function.Arguments)
	t.Logf("  Gmail tool args: %s", gmailTool.Function.Arguments)
	t.Logf("  Replacement rules: %v", result.ReplacementRules)
}

// TestPrivateCompletionsWithComplexMCPScenario tests a more complex scenario
// similar to connecting multiple MCP servers (Google, Slack, etc.)
func TestPrivateCompletionsWithComplexMCPScenario(t *testing.T) {
	logger := log.New(nil)

	anonymizerManager := NewMockAnonymizerManager(0, true, logger)
	defer anonymizerManager.Shutdown()

	// Create mock that simulates multiple MCP tool calls
	mockLLM := &capturingMockCompletionsService{
		response: openai.ChatCompletionMessage{
			Content: "I'll help you manage your tasks across multiple platforms.",
			ToolCalls: []openai.ChatCompletionMessageToolCallUnion{
				{
					ID:   "call_slack_001",
					Type: "function",
					Function: openai.ChatCompletionMessageFunctionToolCallFunction{
						Name:      "slack_send_message",
						Arguments: `{"channel": "#general", "text": "Meeting with PERSON_001 from COMPANY_001 scheduled for tomorrow at LOCATION_006"}`,
					},
				},
				{
					ID:   "call_calendar_001",
					Type: "function",
					Function: openai.ChatCompletionMessageFunctionToolCallFunction{
						Name:      "calendar_create_event",
						Arguments: `{"title": "Meeting with PERSON_001", "location": "LOCATION_006", "attendees": ["PERSON_001@COMPANY_001.com", "PERSON_003@COMPANY_002.com"]}`,
					},
				},
				{
					ID:   "call_twitter_001",
					Type: "function",
					Function: openai.ChatCompletionMessageFunctionToolCallFunction{
						Name:      "twitter_post",
						Arguments: `{"text": "Excited to meet with PERSON_001 from COMPANY_001 tomorrow! #networking"}`,
					},
				},
			},
		},
	}

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

	// Complex user message involving multiple people, companies, and locations
	originalMessage := "Schedule a meeting with John Smith from OpenAI tomorrow at San Francisco office. Also send a Slack message to the team and post about it on Twitter. Include Alice Johnson from Microsoft in the calendar invite."

	// Define multiple MCP-style tools
	tools := []openai.ChatCompletionToolUnionParam{
		openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        "slack_send_message",
			Description: param.Opt[string]{Value: "Send a message to Slack channel"},
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"channel": map[string]interface{}{
						"type":        "string",
						"description": "Slack channel name",
					},
					"text": map[string]interface{}{
						"type":        "string",
						"description": "Message text",
					},
				},
				"required": []string{"channel", "text"},
			},
		}),
		openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        "calendar_create_event",
			Description: param.Opt[string]{Value: "Create a calendar event"},
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"title": map[string]interface{}{
						"type":        "string",
						"description": "Event title",
					},
					"location": map[string]interface{}{
						"type":        "string",
						"description": "Event location",
					},
					"attendees": map[string]interface{}{
						"type":        "array",
						"description": "List of attendee emails",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
				},
				"required": []string{"title", "location"},
			},
		}),
		openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        "twitter_post",
			Description: param.Opt[string]{Value: "Post a tweet on Twitter"},
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"text": map[string]interface{}{
						"type":        "string",
						"description": "Tweet text",
					},
				},
				"required": []string{"text"},
			},
		}),
	}

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage(originalMessage),
	}

	// Execute the completion with multiple tools
	result, err := privateService.Completions(ctx, messages, tools, "gpt-4o-mini", Background)
	if err != nil {
		t.Fatalf("Private completions with complex MCP tools failed: %v", err)
	}

	// === VERIFICATION 1: All tool calls are present ===
	if len(result.Message.ToolCalls) != 3 {
		t.Fatalf("Expected 3 tool calls, got %d", len(result.Message.ToolCalls))
	}

	// === VERIFICATION 2: All tool call arguments are properly de-anonymized ===

	// Check Slack tool call
	slackTool := result.Message.ToolCalls[0]
	var slackArgs map[string]interface{}
	if err := json.Unmarshal([]byte(slackTool.Function.Arguments), &slackArgs); err != nil {
		t.Fatalf("Failed to unmarshal slack tool arguments: %v", err)
	}

	slackText, ok := slackArgs["text"].(string)
	if !ok {
		t.Fatalf("Expected string text in slack tool arguments")
	}

	// Verify Slack message was de-anonymized
	if !strings.Contains(slackText, "John Smith") {
		t.Errorf("Slack message missing de-anonymized name 'John Smith': %s", slackText)
	}
	if !strings.Contains(slackText, "OpenAI") {
		t.Errorf("Slack message missing de-anonymized company 'OpenAI': %s", slackText)
	}
	if !strings.Contains(slackText, "San Francisco") {
		t.Errorf("Slack message missing de-anonymized location 'San Francisco': %s", slackText)
	}
	if strings.Contains(slackText, "PERSON_001") || strings.Contains(slackText, "COMPANY_001") || strings.Contains(slackText, "LOCATION_006") {
		t.Errorf("Slack message still contains anonymized tokens: %s", slackText)
	}

	// Check Calendar tool call
	calendarTool := result.Message.ToolCalls[1]
	var calendarArgs map[string]interface{}
	if err := json.Unmarshal([]byte(calendarTool.Function.Arguments), &calendarArgs); err != nil {
		t.Fatalf("Failed to unmarshal calendar tool arguments: %v", err)
	}

	calendarTitle, ok := calendarArgs["title"].(string)
	if !ok {
		t.Fatalf("Expected string title in calendar tool arguments")
	}

	// Verify calendar event was de-anonymized
	if !strings.Contains(calendarTitle, "John Smith") {
		t.Errorf("Calendar title missing de-anonymized name 'John Smith': %s", calendarTitle)
	}
	if strings.Contains(calendarTitle, "PERSON_001") {
		t.Errorf("Calendar title still contains anonymized tokens: %s", calendarTitle)
	}

	// Check attendees array
	if attendees, attendeesOk := calendarArgs["attendees"].([]interface{}); attendeesOk {
		attendeeStr := ""
		for _, attendee := range attendees {
			if str, strOk := attendee.(string); strOk {
				attendeeStr += str + " "
			}
		}
		if !strings.Contains(attendeeStr, "John Smith") && !strings.Contains(attendeeStr, "OpenAI") {
			t.Errorf("Calendar attendees missing de-anonymized values: %s", attendeeStr)
		}
		if !strings.Contains(attendeeStr, "Alice Johnson") && !strings.Contains(attendeeStr, "Microsoft") {
			t.Errorf("Calendar attendees missing de-anonymized values: %s", attendeeStr)
		}
	}

	// Check Twitter tool call
	twitterTool := result.Message.ToolCalls[2]
	var twitterArgs map[string]interface{}
	if err := json.Unmarshal([]byte(twitterTool.Function.Arguments), &twitterArgs); err != nil {
		t.Fatalf("Failed to unmarshal twitter tool arguments: %v", err)
	}

	twitterText, ok := twitterArgs["text"].(string)
	if !ok {
		t.Fatalf("Expected string text in twitter tool arguments")
	}

	// Verify Twitter post was de-anonymized
	if !strings.Contains(twitterText, "John Smith") {
		t.Errorf("Twitter text missing de-anonymized name 'John Smith': %s", twitterText)
	}
	if !strings.Contains(twitterText, "OpenAI") {
		t.Errorf("Twitter text missing de-anonymized company 'OpenAI': %s", twitterText)
	}
	if strings.Contains(twitterText, "PERSON_001") || strings.Contains(twitterText, "COMPANY_001") {
		t.Errorf("Twitter text still contains anonymized tokens: %s", twitterText)
	}

	// === VERIFICATION 3: Input was properly anonymized ===
	if len(mockLLM.capturedMessages) != 1 {
		t.Fatalf("Expected 1 message sent to LLM, got %d", len(mockLLM.capturedMessages))
	}

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

	// Verify that ALL sensitive data was anonymized in what was sent to LLM
	sensitiveTerms := []string{"John Smith", "OpenAI", "San Francisco", "Alice Johnson", "Microsoft"}
	for _, term := range sensitiveTerms {
		if strings.Contains(anonymizedContent, term) {
			t.Errorf("LLM received un-anonymized sensitive data '%s': %s", term, anonymizedContent)
		}
	}

	t.Logf("Complex MCP Tool Calling Test PASSED:")
	t.Logf("  Original message: %s", originalMessage)
	t.Logf("  Anonymized sent to LLM: %s", anonymizedContent)
	t.Logf("  Final response: %s", result.Message.Content)
	t.Logf("  Tool calls: %d", len(result.Message.ToolCalls))
	t.Logf("  Replacement rules: %v", result.ReplacementRules)
}

// TestPrivateCompletionsToolCallingFailure tests what happens when tool calling fails.
func TestPrivateCompletionsToolCallingFailure(t *testing.T) {
	logger := log.New(nil)

	anonymizerManager := NewMockAnonymizerManager(0, true, logger)
	defer anonymizerManager.Shutdown()

	// Create mock that returns tool calls with malformed JSON
	mockLLM := &capturingMockCompletionsService{
		response: openai.ChatCompletionMessage{
			Content: "I'll help you with that.",
			ToolCalls: []openai.ChatCompletionMessageToolCallUnion{
				{
					ID:   "call_broken_001",
					Type: "function",
					Function: openai.ChatCompletionMessageFunctionToolCallFunction{
						Name:      "test_tool",
						Arguments: `{"name": "PERSON_001", "invalid_json": }`, // Malformed JSON
					},
				},
			},
		},
	}

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

	tools := []openai.ChatCompletionToolUnionParam{
		openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        "test_tool",
			Description: param.Opt[string]{Value: "Test tool"},
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type": "string",
					},
				},
				"required": []string{"name"},
			},
		}),
	}

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Test message with John Smith"),
	}

	// This should not fail even with malformed tool call JSON
	result, err := privateService.Completions(ctx, messages, tools, "gpt-4o-mini", Background)
	if err != nil {
		t.Fatalf("Private completions failed: %v", err)
	}

	// Should still return the tool call, even if arguments are malformed
	if len(result.Message.ToolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(result.Message.ToolCalls))
	}

	// The malformed JSON should be returned as-is (anonymizer should handle gracefully)
	toolCall := result.Message.ToolCalls[0]
	if toolCall.Function.Name != "test_tool" {
		t.Errorf("Expected tool name 'test_tool', got '%s'", toolCall.Function.Name)
	}

	// The arguments should still be present (even if malformed)
	if toolCall.Function.Arguments == "" {
		t.Error("Expected tool arguments to be present")
	}

	t.Logf("Tool calling failure test PASSED - service handled malformed JSON gracefully")
	t.Logf("  Tool call arguments: %s", toolCall.Function.Arguments)
}

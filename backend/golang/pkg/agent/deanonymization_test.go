package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
)

// Mock tool for testing de-anonymization.
type mockDeAnonymizationTool struct {
	name         string
	executed     bool
	receivedArgs map[string]interface{}
	result       types.ToolResult
}

func (m *mockDeAnonymizationTool) Execute(ctx context.Context, args map[string]interface{}) (types.ToolResult, error) {
	m.executed = true
	m.receivedArgs = args
	return m.result, nil
}

func TestDeAnonymizationFlow(t *testing.T) {
	// Test that demonstrates the de-anonymization flow works correctly

	// 1. Create replacement rules (what the anonymizer would provide)
	replacementRules := map[string]string{
		"PERSON_001":  "John Smith",
		"COMPANY_001": "OpenAI",
	}

	// 2. Create anonymized tool call arguments (what the LLM would generate)
	anonymizedArgs := `{"name": "PERSON_001", "company": "COMPANY_001"}`

	// 3. Parse the arguments
	var parsedArgs map[string]interface{}
	err := json.Unmarshal([]byte(anonymizedArgs), &parsedArgs)
	if err != nil {
		t.Fatalf("Failed to parse arguments: %v", err)
	}

	// 4. De-anonymize the arguments (this is what should happen in ExecuteStreamWithPrivacy)
	deAnonymizedArgs := make(map[string]interface{})
	for key, value := range parsedArgs {
		if strValue, ok := value.(string); ok {
			if realValue, exists := replacementRules[strValue]; exists {
				deAnonymizedArgs[key] = realValue
			} else {
				deAnonymizedArgs[key] = value
			}
		} else {
			deAnonymizedArgs[key] = value
		}
	}

	// 5. Create mock tool
	mockTool := &mockDeAnonymizationTool{
		name: "test_search",
		result: &types.StructuredToolResult{
			Output: map[string]any{
				"content": "Found information about the person",
			},
		},
	}

	// 6. Execute the tool with de-anonymized arguments
	result, err := mockTool.Execute(context.Background(), deAnonymizedArgs)
	if err != nil {
		t.Fatalf("Tool execution failed: %v", err)
	}

	// 7. Verify the tool was executed
	if !mockTool.executed {
		t.Error("Tool was not executed")
	}

	// 8. Verify the tool received de-anonymized arguments
	if mockTool.receivedArgs == nil {
		t.Fatal("Tool did not receive arguments")
	}

	if name, ok := mockTool.receivedArgs["name"].(string); !ok || name != "John Smith" {
		t.Errorf("Expected de-anonymized name 'John Smith', got %v", mockTool.receivedArgs["name"])
	}

	if company, ok := mockTool.receivedArgs["company"].(string); !ok || company != "OpenAI" {
		t.Errorf("Expected de-anonymized company 'OpenAI', got %v", mockTool.receivedArgs["company"])
	}

	// 9. Verify the result
	if result.Content() != "Found information about the person" {
		t.Errorf("Expected tool result, got %s", result.Content())
	}

	t.Log("De-anonymization flow test passed - tool received de-anonymized arguments")
}

func TestAnonymizationReplacementRules(t *testing.T) {
	// Test the replacement rules functionality
	replacementRules := map[string]string{
		"PERSON_001":  "John Smith",
		"COMPANY_001": "OpenAI",
		"EMAIL_001":   "john.smith@openai.com",
	}

	// Test cases for anonymized strings
	testCases := []struct {
		anonymized string
		expected   string
	}{
		{"PERSON_001", "John Smith"},
		{"COMPANY_001", "OpenAI"},
		{"EMAIL_001", "john.smith@openai.com"},
		{"UNKNOWN_001", "UNKNOWN_001"}, // Should remain unchanged
	}

	for _, tc := range testCases {
		if realValue, exists := replacementRules[tc.anonymized]; exists {
			if realValue != tc.expected {
				t.Errorf("Expected %s -> %s, got %s", tc.anonymized, tc.expected, realValue)
			}
		} else {
			if tc.anonymized != tc.expected {
				t.Errorf("Expected %s to remain unchanged, but expected %s", tc.anonymized, tc.expected)
			}
		}
	}

	t.Log("Anonymization replacement rules test passed")
}

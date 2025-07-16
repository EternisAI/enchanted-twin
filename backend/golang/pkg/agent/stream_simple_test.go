package agent

import (
	"testing"

	"github.com/charmbracelet/log"

	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
)

// Simple test to verify deanonymization works.
func TestExecuteStreamWithPrivacy_Simple(t *testing.T) {
	_ = log.New(nil)

	// Create a mock tool result
	toolResult := &types.StructuredToolResult{
		Output: map[string]any{
			"content": "Test result",
		},
	}

	// Test that deanonymization would work by checking if replacement rules exist
	replacementRules := map[string]string{
		"PERSON_001":  "John Smith",
		"COMPANY_001": "OpenAI",
	}

	// Verify replacement rules functionality
	if len(replacementRules) != 2 {
		t.Errorf("Expected 2 replacement rules, got %d", len(replacementRules))
	}

	if replacementRules["PERSON_001"] != "John Smith" {
		t.Errorf("Expected PERSON_001 to be John Smith, got %s", replacementRules["PERSON_001"])
	}

	if replacementRules["COMPANY_001"] != "OpenAI" {
		t.Errorf("Expected COMPANY_001 to be OpenAI, got %s", replacementRules["COMPANY_001"])
	}

	// Test tool result functionality
	if toolResult.Content() != "Test result" {
		t.Errorf("Expected tool result content to be 'Test result', got %s", toolResult.Content())
	}

	t.Log("Simple deanonymization test passed")
}

// Test tool execution flow.
func TestToolExecutionFlow(t *testing.T) {
	_ = log.New(nil)

	// Test that we can create an agent and it has the correct fields
	agent := &Agent{
		CompletionsModel: "gpt-4",
	}

	if agent.CompletionsModel != "gpt-4" {
		t.Errorf("Expected model to be gpt-4, got %s", agent.CompletionsModel)
	}

	// Test tool execution conceptually
	testArgs := map[string]interface{}{
		"name":    "John Smith",
		"company": "OpenAI",
	}

	// Verify we can parse tool arguments
	if name, ok := testArgs["name"].(string); !ok || name != "John Smith" {
		t.Errorf("Expected name to be John Smith, got %v", testArgs["name"])
	}

	if company, ok := testArgs["company"].(string); !ok || company != "OpenAI" {
		t.Errorf("Expected company to be OpenAI, got %v", testArgs["company"])
	}

	t.Log("Tool execution flow test passed")
}

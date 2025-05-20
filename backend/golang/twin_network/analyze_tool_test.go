package twin_network

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/charmbracelet/log"
)

func TestReadNetworkTool(t *testing.T) {
	apiKey := "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"
	if apiKey == "" {
		fmt.Println("Please set COMPLETIONS_API_KEY environment variable")
		return
	}

	baseURL := "https://openrouter.ai/api/v1"

	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
		Level:           log.DebugLevel,
	})

	aiService := ai.NewOpenAIService(logger, apiKey, baseURL)

	tool := NewReadNetworkTool(logger, aiService, "gpt-4.1-mini")

	messages := []map[string]string{
		{
			"role":    "user",
			"content": "Hello from the twin network! How are you doing today?",
		},
		{
			"role":    "assistant",
			"content": "I'm doing well, thank you for asking! How can I assist you today?",
		},
		{
			"role":    "user",
			"content": "Can you help me analyze this data pattern?",
		},
		{
			"role":    "assistant",
			"content": "I've analyzed the pattern and found some interesting correlations.",
		},
	}

	inputs := map[string]any{
		"network_message": messages,
	}

	result, err := tool.Execute(context.Background(), inputs)
	if err != nil {
		t.Errorf("Tool execution failed: %v", err)
		return
	}

	// Print the structured result fields
	if structuredResult, ok := result.(*types.StructuredToolResult); ok {
		fmt.Printf("Tool Name: %s\n", structuredResult.ToolName)
		fmt.Printf("Tool Params: %+v\n", structuredResult.ToolParams)
		fmt.Printf("Output: %+v\n", structuredResult.Output)
	} else {
		t.Errorf("Unexpected result type: %T", result)
	}
}

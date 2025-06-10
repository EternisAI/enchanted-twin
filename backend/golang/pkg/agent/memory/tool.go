package memory

import (
	"context"
	"errors"
	"fmt"

	"github.com/charmbracelet/log"
	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"

	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
)

// MemorySearchTool implements a tool for searching agent memory.
type MemorySearchTool struct {
	Logger *log.Logger
	Memory Storage
}

// NewMemorySearchTool creates a new memory search tool.
func NewMemorySearchTool(logger *log.Logger, memory Storage) *MemorySearchTool {
	return &MemorySearchTool{Logger: logger, Memory: memory}
}

// Execute runs the memory search.
func (t *MemorySearchTool) Execute(ctx context.Context, input map[string]any) (types.ToolResult, error) {
	queryVal, exists := input["query"]
	if !exists {
		return nil, errors.New("query is required")
	}

	query, ok := queryVal.(string)
	if !ok {
		return nil, errors.New("query must be a string")
	}

	// For now, search across all speakers
	result, err := t.Memory.Query(ctx, query, nil)
	if err != nil {
		t.Logger.Error("Memory query failed", "error", err, "query", query)
		return nil, err
	}

	t.Logger.Info("Memory query completed",
		"query", query,
		"facts_found", len(result.Facts))

	// Log first few results for debugging
	for i, fact := range result.Facts {
		if i >= 3 { // Only log first 3 to avoid spam
			break
		}
		t.Logger.Info("Memory result sample",
			"index", i+1,
			"content", fact.Content,
			"speaker", fact.Speaker,
			"source", fact.Source,
			"timestamp", fact.Timestamp.Format("2006-01-02 15:04:05"))
	}

	if len(result.Facts) == 0 {
		t.Logger.Warn("No facts found for query - this suggests a semantic search issue", "query", query)

		// Try some test queries to debug
		testQueries := []string{"WhatsApp", "contact", "Ornella", "name"}
		for _, testQuery := range testQueries {
			testResult, testErr := t.Memory.Query(ctx, testQuery, nil)
			if testErr == nil {
				t.Logger.Info("Test query result",
					"test_query", testQuery,
					"facts_found", len(testResult.Facts))
			}
		}
	}

	resultText := ""
	for i, fact := range result.Facts {
		resultText += fmt.Sprintf(
			"Memory %d: %s (Source: %s, Time: %s)\n",
			i+1,
			fact.Content,
			fact.Source,
			fact.Timestamp.Format("2006-01-02 15:04:05"),
		)
	}

	t.Logger.Debug(
		"Memory tool result",
		"facts_count",
		len(result.Facts),
		"response",
		resultText,
	)

	t.Logger.Info("=== MEMORY TOOL QUERY END ===")

	return types.SimpleToolResult(resultText), nil
}

// Definition returns the OpenAI tool definition.
func (t *MemorySearchTool) Definition() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name: "memory_tool",
			Description: param.NewOpt(
				"Information that should be retrieved from the user's memory.",
			),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]string{
						"type": "string",
					},
				},
				"required": []string{"query"},
			},
		},
	}
}

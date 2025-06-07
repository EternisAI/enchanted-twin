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
		return nil, err
	}

	resultText := ""
	for i, fact := range result.Facts {
		timeStr := "N/A"
		if !fact.Timestamp.IsZero() {
			timeStr = fact.Timestamp.Format("2006-01-02 15:04:05")
		}
		resultText += fmt.Sprintf(
			"Memory %d: %s (Speaker: %s, Source: %s, Time: %s)\n",
			i+1,
			fact.Content,
			fact.Speaker,
			fact.Source,
			timeStr,
		)
	}

	t.Logger.Debug(
		"Memory tool result",
		"facts_count",
		len(result.Facts),
		"response",
		resultText,
	)

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

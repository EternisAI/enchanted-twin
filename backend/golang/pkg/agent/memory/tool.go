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

	result, err := t.Memory.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	resultText := ""
	for i, text := range result.Text {
		resultText += fmt.Sprintf("Answer %d: %s\n", i, text)
	}
	for _, doc := range result.Documents {
		resultText += fmt.Sprintf(
			"Memory Document %s: %s. At %s\n",
			doc.ID,
			doc.Content,
			doc.Timestamp,
		)
	}

	t.Logger.Debug(
		"Memory tool result",
		"documents_count",
		len(result.Documents),
		"text_count",
		len(result.Text),
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

// GetMemoryTools returns all memory-related tools.
func GetMemoryTools(logger *log.Logger, storage Storage) []types.Tool {
	if storage == nil {
		return []types.Tool{}
	}

	return []types.Tool{
		NewMemorySearchTool(logger, storage),
	}
}

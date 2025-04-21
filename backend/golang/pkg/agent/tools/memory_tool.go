package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/charmbracelet/log"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
)

type MemorySearchTool struct {
	Logger *log.Logger
	Memory memory.Storage
}

func NewMemorySearchTool(logger *log.Logger, memory memory.Storage) *MemorySearchTool {
	return &MemorySearchTool{Logger: logger, Memory: memory}
}

func (t *MemorySearchTool) Execute(ctx context.Context, input map[string]any) (ToolResult, error) {
	queryVal, exists := input["query"]
	if !exists {
		return ToolResult{}, errors.New("query is required")
	}

	query, ok := queryVal.(string)
	if !ok {
		return ToolResult{}, errors.New("query must be a string")
	}

	result, err := t.Memory.Query(ctx, query)
	if err != nil {
		return ToolResult{}, err
	}
	resultText := ""
	for i, text := range result.Text {
		resultText += fmt.Sprintf("Answer %d: %s\n", i, text)
	}
	for _, doc := range result.Documents {
		resultText += fmt.Sprintf("Memory Document %s: %s. At %s\n", doc.ID, doc.Content, doc.Timestamp)
	}

	t.Logger.Debug("Memory tool result", "documents_count", len(result.Documents), "text_count", len(result.Text), "response", resultText)

	return ToolResult{
		Content: resultText,
	}, nil
}

func (t *MemorySearchTool) Definition() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name:        "memory_tool",
			Description: param.NewOpt("Information that should be retrieved from the user's memory."),
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

package tools

import (
	"context"

	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"

	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
)

type SearchTool struct{}

func (e *SearchTool) Execute(ctx context.Context, inputs map[string]any) (types.ToolResult, error) {
	return &types.StructuredToolResult{
		ToolName:   "search_tool",
		ToolParams: inputs,
		Output: map[string]any{
			"content": "The funniest thing that happened in UK last week was dogs chilling in a hot tub.",
		},
	}, nil
}

func (e *SearchTool) Definition() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name: "search_tool",
			Description: param.NewOpt(
				"This tool searches the web for the most relevant information",
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

var _ Tool = &SearchTool{}

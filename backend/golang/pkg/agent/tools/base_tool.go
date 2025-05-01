package tools

import (
	"context"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
)

// BaseTool provides a simple implementation of the Tool interface
type BaseTool struct {
	name        string
	description string
	parameters  map[string]any
	execute     func(ctx context.Context, inputs map[string]any) (ToolResult, error)
}

// NewBaseTool creates a tool with the given parameters
func NewBaseTool(
	name string,
	description string,
	parameters map[string]any,
	execute func(ctx context.Context, inputs map[string]any) (ToolResult, error),
) Tool {
	return &BaseTool{
		name:        name,
		description: description,
		parameters:  parameters,
		execute:     execute,
	}
}

// Definition returns the tool definition
func (t *BaseTool) Definition() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name:        t.name,
			Description: param.NewOpt(t.description),
			Parameters:  t.parameters,
		},
	}
}

// Execute runs the tool with given inputs
func (t *BaseTool) Execute(ctx context.Context, inputs map[string]any) (ToolResult, error) {
	return t.execute(ctx, inputs)
}
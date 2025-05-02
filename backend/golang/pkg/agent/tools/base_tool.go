package tools

import (
	"context"

	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
)

// BaseTool provides a simple implementation of the Tool interface
type BaseTool struct {
	name        string
	description string
	parameters  map[string]any
	execute     func(ctx context.Context, inputs map[string]any) (types.ToolResult, error)
}

// NewBaseTool creates a tool with the given parameters
func NewBaseTool(
	name string,
	description string,
	parameters map[string]any,
	execute func(ctx context.Context, inputs map[string]any) (types.ToolResult, error),
) *BaseTool {
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
func (t *BaseTool) Execute(ctx context.Context, inputs map[string]any) (types.ToolResult, error) {
	result, err := t.execute(ctx, inputs)

	// Check if the tool name is set for structured result
	// TODO: this should be supported another way, either by supporting a `.WithToolName()` method or
	// by ensuring that the tool name is set in the tool definition
	if result.Tool() == "" {
		if structResult, ok := result.(*types.StructuredToolResult); ok {
			// If tool name is not set, set it automatically
			structResult.ToolName = t.name
			return structResult, nil
		}
	}

	return result, err
}

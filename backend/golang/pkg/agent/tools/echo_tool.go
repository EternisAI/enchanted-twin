package tools

import (
	"context"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
	"github.com/pkg/errors"

	agenttypes "github.com/EternisAI/enchanted-twin/pkg/agent/types"
)

// EchoTool is a simple tool that echoes back the input text.
type EchoTool struct{}

// Definition returns the tool definition.
func (t *EchoTool) Definition() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name:        "echo",
			Description: param.NewOpt("Echoes back the input text"),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]interface{}{
					"text": map[string]interface{}{
						"type":        "string",
						"description": "The text to echo back",
					},
				},
				"required": []string{"text"},
			},
		},
	}
}

// Execute simply returns the input text.
func (t *EchoTool) Execute(ctx context.Context, args map[string]interface{}) (agenttypes.ToolResult, error) {
	text, ok := args["text"].(string)
	if !ok || text == "" {
		return &agenttypes.StructuredToolResult{
			ToolName:   "echo",
			ToolParams: args,
			ToolError:  "text parameter is required",
		}, errors.New("text parameter is required")
	}

	content := fmt.Sprintf("Echo: %s", text)
	return &agenttypes.StructuredToolResult{
		ToolName:   "echo",
		ToolParams: args,
		Output: map[string]any{
			"content": content,
		},
	}, nil
}

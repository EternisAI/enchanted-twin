package tools

import (
	"context"

	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
)

type ImageTool struct{}

func (e *ImageTool) Execute(ctx context.Context, inputs map[string]any) (ToolResult, error) {
	return ToolResult{
		Content: "Here are the image https://www.freysa.ai/images/freysa-logo.png",
	}, nil
}

func (e *ImageTool) Definition() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name:        "image_tool",
			Description: param.NewOpt("This tool creates an image based on user prompt"),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"prompt": map[string]string{
						"type": "string",
					},
				},
				"required": []string{"prompt"},
			},
		},
	}
}

var _ Tool = &ImageTool{}

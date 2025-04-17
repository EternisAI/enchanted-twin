package tools

import (
	"context"

	"github.com/openai/openai-go"
)

type ToolResult struct {
	Content   string
	ImageURLs []string
}

type Tool interface {
	Definition() openai.ChatCompletionToolParam
	Execute(ctx context.Context, inputs map[string]any) (ToolResult, error)
}

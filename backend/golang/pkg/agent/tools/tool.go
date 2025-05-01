package tools

import (
	"context"

	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
	"github.com/openai/openai-go"
)

// Tool is a local alias for the Tool interface to avoid import cycles
// We're maintaining compatibility with the existing codebase while we transition
type Tool interface {
	Definition() openai.ChatCompletionToolParam
	Execute(ctx context.Context, inputs map[string]any) (*types.StandardToolResult, error)
}

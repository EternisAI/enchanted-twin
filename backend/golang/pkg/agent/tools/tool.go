package tools

import (
	"context"

	"github.com/openai/openai-go"

	agenttypes "github.com/EternisAI/enchanted-twin/pkg/agent/types"
)

// We're maintaining compatibility with the existing codebase while we transition.
type Tool interface {
	Definition() openai.ChatCompletionToolParam
	Execute(ctx context.Context, inputs map[string]any) (agenttypes.ToolResult, error)
}

package tools

import (
	"context"

	"github.com/openai/openai-go/v3"

	agenttypes "github.com/EternisAI/enchanted-twin/pkg/agent/types"
)

// We're maintaining compatibility with the existing codebase while we transition.
type Tool interface {
	Definition() openai.ChatCompletionToolUnionParam
	Execute(ctx context.Context, inputs map[string]any) (agenttypes.ToolResult, error)
}

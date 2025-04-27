package plannedv2

import (
	"context"
	"fmt"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/openai/openai-go"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"
)

const DefaultToolTimeout = 1 * time.Minute

type AgentActivities struct {
	aiService *ai.Service
}

func NewAgentActivities(aiService *ai.Service) *AgentActivities {
	return &AgentActivities{
		aiService: aiService,
	}
}

func (a *AgentActivities) RegisterActivities(w worker.Worker) {
	w.RegisterActivity(a.LLMCompletionActivity)
	w.RegisterActivity(a.ExecuteToolActivity)
}

func (a *AgentActivities) LLMCompletionActivity(
	ctx context.Context,
	model string,
	messages []Message,
	tools []openai.ChatCompletionToolParam,
) (openai.ChatCompletionMessage, error) {

	params := openai.ChatCompletionNewParams{
		Model:    model,
		Messages: ToOpenAIMessages(messages),
		Tools:    tools,
	}

	return a.aiService.ParamsCompletions(ctx, params)
}

func (a *AgentActivities) ExecuteToolActivity(
	ctx context.Context,
	toolName string,
	args map[string]interface{},
) (*types.ToolResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Executing tool", "tool", toolName, "args", args)

	// Special built-in workflow tools
	if toolName == "sleep" || toolName == "sleep_until" {
		// These are handled directly in tools.go and not in the registry
		return nil, fmt.Errorf("tool '%s' should be handled by the workflow directly", toolName)
	}

	// Use the global registry
	registry := tools.GetGlobal(nil) // We don't pass logger as it's already initialized
	tool, exists := registry.Get(toolName)
	if !exists {
		return nil, fmt.Errorf("tool not found in registry: %s", toolName)
	}

	// Execute the tool
	result, err := tool.Execute(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to execute tool '%s': %w", toolName, err)
	}

	// Convert to our ToolResult format
	return &types.ToolResult{
		Tool:      toolName,
		Params:    args,
		Content:   result.Content,
		Data:      result.Content, // Using content as data for compatibility
		ImageURLs: result.ImageURLs,
	}, nil
}

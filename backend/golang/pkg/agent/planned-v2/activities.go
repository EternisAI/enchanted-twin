package planned

import (
	"context"
	"fmt"
	"time"

	"github.com/openai/openai-go"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"

	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

const DefaultToolTimeout = 1 * time.Minute

type AgentActivities struct {
	aiService *ai.Service
	registry  tools.ToolRegistry
	executor  *ToolExecutor
}

func NewAgentActivities(ctx context.Context, aiService *ai.Service, registry tools.ToolRegistry) *AgentActivities {
	activities := &AgentActivities{
		aiService: aiService,
		registry:  registry,
		executor:  NewToolExecutor(registry, nil),
	}

	return activities
}

func (a *AgentActivities) RegisterActivities(w worker.Worker) {
	w.RegisterActivity(a.LLMCompletionActivity)
	w.RegisterActivity(a.ExecuteToolActivity)
}

func (a *AgentActivities) LLMCompletionActivity(
	ctx context.Context,
	model string,
	messages []ai.Message,
	selectedTools []string,
) (openai.ChatCompletionMessage, error) {
	toolz := a.registry
	if len(selectedTools) > 0 {
		toolz = toolz.Selecting(selectedTools...)
	}
	toolz = toolz.Excluding("execute_plan", "schedule_task")

	availableTools := toolz.Definitions()
	for _, tool := range tools.WorkflowImmediateTools() {
		toolDef := tool.Definition()
		if toolDef.Function.Name == "" {
			continue
		}
		availableTools = append(availableTools, toolDef)
	}

	params := openai.ChatCompletionNewParams{
		Model:    model,
		Messages: ai.ToOpenAIMessages(messages),
		Tools:    availableTools,
	}

	return a.aiService.ParamsCompletions(ctx, params)
}

func (a *AgentActivities) ExecuteToolActivity(
	ctx context.Context,
	toolName string,
	args map[string]interface{},
) (types.ToolResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Executing tool", "tool", toolName, "args", args)

	// Special built-in workflow tools
	if toolName == "sleep" || toolName == "sleep_until" || toolName == "final_response" {
		// These are handled directly in the workflow and not in the registry
		return nil, fmt.Errorf("tool '%s' should be handled by the workflow directly", toolName)
	}

	// Use the injected registry if available, otherwise fall back to global
	registry := a.registry

	tool, exists := registry.Get(toolName)
	if !exists {
		return nil, fmt.Errorf("tool not found in registry: %s", toolName)
	}

	// Execute the tool
	result, err := tool.Execute(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to execute tool '%s': %w", toolName, err)
	}

	// Return the result directly since it already implements the ToolResult interface
	return result, nil
}

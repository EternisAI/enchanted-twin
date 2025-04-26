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

// DefaultToolTimeout is the default timeout for tool execution.
const DefaultToolTimeout = 1 * time.Minute

// RegisterActivities registers all activities with the Temporal worker.
func RegisterActivities(w worker.Worker) {
	// Register LLM activities
	w.RegisterActivity(LLMCompletionActivity)

	// Register tool activities
	w.RegisterActivity(ExecuteToolActivity)
}

// LLMCompletionActivity executes a completion request against the LLM API.
func LLMCompletionActivity(
	ctx context.Context,
	model string,
	messages []Message,
	tools []openai.ChatCompletionToolParam,
) (openai.ChatCompletionMessage, error) {
	// We don't need the logger for now
	// logger := activity.GetLogger(ctx)

	params := openai.ChatCompletionNewParams{
		Model:    model,
		Messages: ToOpenAIMessages(messages),
		Tools:    tools,
	}

	// Get AI service singleton
	aiService := ai.GetInstance()
	if aiService == nil {
		return openai.ChatCompletionMessage{}, fmt.Errorf("AI service singleton not initialized")
	}

	// Execute completions
	return aiService.ParamsCompletions(ctx, params)
}

// ExecuteToolActivity is a generic activity for executing tools.
func ExecuteToolActivity(
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

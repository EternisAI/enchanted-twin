package plannedv2

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

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
	w.RegisterActivity(EchoActivity)
	w.RegisterActivity(MathActivity)
}

// LLMCompletionActivity executes a completion request against the LLM API.
func LLMCompletionActivity(
	ctx context.Context,
	model string,
	messages []Message,
	tools []openai.ChatCompletionToolParam,
) (openai.ChatCompletionMessage, error) {
	logger := activity.GetLogger(ctx)

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
) (*ToolResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Executing tool", "tool", toolName, "args", args)

	// Execute the tool based on its type
	switch toolName {
	case "echo":
		return executeEchoTool(ctx, args)
	case "math":
		return executeMathTool(ctx, args)
	default:
		return nil, fmt.Errorf("tool execution not implemented: %s", toolName)
	}
}

// EchoActivity is a simple activity that echoes back the input text.
func EchoActivity(ctx context.Context, text string) (string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Executing EchoActivity", "text", text)

	// Simply return the input text
	return fmt.Sprintf("Echo: %s", text), nil
}

// MathActivity performs basic math operations.
func MathActivity(ctx context.Context, operation string, a, b float64) (float64, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Executing MathActivity", "operation", operation, "a", a, "b", b)

	// Perform the requested operation
	switch operation {
	case "add":
		return a + b, nil
	case "subtract":
		return a - b, nil
	case "multiply":
		return a * b, nil
	case "divide":
		if b == 0 {
			return 0, fmt.Errorf("division by zero")
		}
		return a / b, nil
	default:
		return 0, fmt.Errorf("unknown operation: %s", operation)
	}
}

// executeEchoTool executes the echo tool.
func executeEchoTool(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	// Extract text parameter
	text, ok := params["text"].(string)
	if !ok {
		return nil, fmt.Errorf("echo tool requires text parameter")
	}

	// Execute the activity
	result, err := EchoActivity(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("failed to execute echo activity: %w", err)
	}

	// Return the result
	return &ToolResult{
		Tool:    "echo",
		Params:  params,
		Content: result,
		Data:    result,
	}, nil
}

// executeMathTool executes the math tool.
func executeMathTool(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	// Extract parameters
	operation, ok1 := params["operation"].(string)

	// Handle different types for a and b (json.Number, float64, etc.)
	var a, b float64
	var ok2, ok3 bool

	// Try to extract a and b as float64
	a, ok2 = params["a"].(float64)
	b, ok3 = params["b"].(float64)

	// If not directly float64, try to parse from json.Number or other types
	if !ok2 {
		if aNum, ok := params["a"].(json.Number); ok {
			if aFloat, err := aNum.Float64(); err == nil {
				a = aFloat
				ok2 = true
			}
		} else if aInt, ok := params["a"].(int); ok {
			a = float64(aInt)
			ok2 = true
		}
	}

	if !ok3 {
		if bNum, ok := params["b"].(json.Number); ok {
			if bFloat, err := bNum.Float64(); err == nil {
				b = bFloat
				ok3 = true
			}
		} else if bInt, ok := params["b"].(int); ok {
			b = float64(bInt)
			ok3 = true
		}
	}

	// Check if all parameters are valid
	if !ok1 || !ok2 || !ok3 {
		return nil, fmt.Errorf("math tool requires operation, a, and b parameters")
	}

	// Execute the activity
	result, err := MathActivity(ctx, operation, a, b)
	if err != nil {
		return nil, fmt.Errorf("failed to execute math activity: %w", err)
	}

	// Create a human-readable observation
	observation := fmt.Sprintf("Math result: %v %s %v = %v", a, operation, b, result)

	// Return the result
	return &ToolResult{
		Tool:    "math",
		Params:  params,
		Content: observation,
		Data:    result,
	}, nil
}

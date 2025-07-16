package agent

import (
	"context"
	"encoding/json"

	"github.com/openai/openai-go"

	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

type StreamDelta = ai.StreamDelta

// ExecuteStreamWithPrivacy executes agent with privacy-enabled streaming and tool support.
func (a *Agent) ExecuteStreamWithPrivacy(
	ctx context.Context,
	messages []openai.ChatCompletionMessageParamUnion,
	currentTools []tools.Tool,
	onDelta func(StreamDelta),
	reasoning bool,
) (AgentResponse, error) {
	// Build lookup + OpenAI tool defs once.
	toolDefs := make([]openai.ChatCompletionToolParam, 0, len(currentTools))
	toolMap := map[string]tools.Tool{}
	for _, t := range currentTools {
		d := t.Definition()
		toolDefs = append(toolDefs, d)
		toolMap[d.Function.Name] = t
	}

	languageModel := a.CompletionsModel
	if reasoning {
		languageModel = a.ReasoningModel
	}

	var allToolCalls []openai.ChatCompletionMessageToolCall
	var allToolResults []types.ToolResult
	var finalContent string
	var finalReplacementRules map[string]string

	for currentStep := 0; currentStep < MAX_STEPS; currentStep++ {
		result, err := a.aiService.CompletionsStreamWithPrivacy(ctx, messages, toolDefs, languageModel, onDelta)
		if err != nil {
			return AgentResponse{}, err
		}

		finalContent = result.Message.Content
		finalReplacementRules = result.ReplacementRules
		messages = append(messages, result.Message.ToParam())

		if len(result.Message.ToolCalls) == 0 {
			break
		}

		for _, toolCall := range result.Message.ToolCalls {
			if a.PreToolCallback != nil {
				a.PreToolCallback(toolCall)
			}

			tool, exists := toolMap[toolCall.Function.Name]
			if !exists {
				a.logger.Error("Tool not found", "tool_name", toolCall.Function.Name)
				continue
			}

			var args map[string]interface{}
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
				a.logger.Error("Failed to parse tool arguments", "error", err)
				continue
			}

			deAnonymizedArgs := make(map[string]interface{})
			for key, value := range args {
				if strValue, ok := value.(string); ok {
					if realValue, exists := result.ReplacementRules[strValue]; exists {
						deAnonymizedArgs[key] = realValue
					} else {
						deAnonymizedArgs[key] = value
					}
				} else {
					deAnonymizedArgs[key] = value
				}
			}

			toolResult, err := tool.Execute(ctx, deAnonymizedArgs)
			if err != nil {
				a.logger.Error("Tool execution failed", "tool_name", toolCall.Function.Name, "error", err)
				continue
			}

			if a.PostToolCallback != nil {
				a.PostToolCallback(toolCall, toolResult)
			}

			allToolResults = append(allToolResults, toolResult)
			allToolCalls = append(allToolCalls, toolCall)
			messages = append(messages, openai.ToolMessage(toolResult.Content(), toolCall.ID))
		}
	}

	return AgentResponse{
		Content:          finalContent,
		ToolCalls:        allToolCalls,
		ToolResults:      allToolResults,
		ImageURLs:        []string{},
		ReplacementRules: finalReplacementRules,
	}, nil
}

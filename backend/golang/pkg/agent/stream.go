package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/openai/openai-go"

	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

type StreamDelta = ai.StreamDelta

func (a *Agent) ExecuteStreamWithPrivacy(
	ctx context.Context,
	conversationID string,
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
	var allImageURLs []string
	var finalContent string
	var finalReplacementRules map[string]string
	var toolErrors []string

	for currentStep := 0; currentStep < MAX_STEPS; currentStep++ {
		result, err := a.aiService.CompletionsStreamWithPrivacy(ctx, conversationID, messages, toolDefs, languageModel, onDelta)
		if err != nil {
			return AgentResponse{}, err
		}

		finalContent = result.Message.Content
		finalReplacementRules = result.ReplacementRules

		// Create anonymized version of the agent response for conversation context
		// We need to reverse the replacement rules to convert from de-anonymized back to anonymized
		reverseRules := make(map[string]string)
		for anonymized, original := range result.ReplacementRules {
			reverseRules[original] = anonymized
		}

		// Apply reverse anonymization to the agent response
		anonymizedAgentResponse := result.Message
		if anonymizedAgentResponse.Content != "" {
			// Use replacement trie for case-preserving anonymization
			trie := ai.NewReplacementTrieFromRules(reverseRules)
			anonymizedContent, _ := trie.ReplaceAll(anonymizedAgentResponse.Content)
			anonymizedAgentResponse.Content = anonymizedContent
		}

		// Handle tool calls anonymization if present
		if len(anonymizedAgentResponse.ToolCalls) > 0 {
			trie := ai.NewReplacementTrieFromRules(reverseRules)
			for i, toolCall := range anonymizedAgentResponse.ToolCalls {
				if toolCall.Function.Arguments != "" {
					anonymizedArgs, _ := trie.ReplaceAll(toolCall.Function.Arguments)
					anonymizedAgentResponse.ToolCalls[i].Function.Arguments = anonymizedArgs
				}
			}
		}

		messages = append(messages, anonymizedAgentResponse.ToParam())

		if len(result.Message.ToolCalls) == 0 {
			break
		}

		for _, toolCall := range result.Message.ToolCalls {
			allToolCalls = append(allToolCalls, toolCall)

			if a.PreToolCallback != nil {
				a.PreToolCallback(toolCall)
			}

			tool, exists := toolMap[toolCall.Function.Name]
			if !exists {
				err := fmt.Sprintf("Tool not found: %s", toolCall.Function.Name)
				a.logger.Error("Tool not found", "tool_name", toolCall.Function.Name)
				toolErrors = append(toolErrors, err)
				continue
			}

			var args map[string]interface{}
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
				errorMsg := fmt.Sprintf("Failed to parse arguments for tool %s: %v", toolCall.Function.Name, err)
				a.logger.Error("Failed to parse tool arguments", "error", err)
				toolErrors = append(toolErrors, errorMsg)
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
				errorMsg := fmt.Sprintf("Tool execution failed for %s: %v", toolCall.Function.Name, err)
				a.logger.Error("Tool execution failed", "tool_name", toolCall.Function.Name, "error", err, "args", toolCall.Function.Arguments)
				toolErrors = append(toolErrors, errorMsg)
				messages = append(messages, openai.ToolMessage(errorMsg, toolCall.ID))
				continue
			}

			if a.PostToolCallback != nil {
				a.PostToolCallback(toolCall, toolResult)
			}

			allToolResults = append(allToolResults, toolResult)
			messages = append(messages, openai.ToolMessage(toolResult.Content(), toolCall.ID))
			allImageURLs = append(allImageURLs, toolResult.ImageURLs()...)
		}
	}

	return AgentResponse{
		Content:          finalContent,
		ToolCalls:        allToolCalls,
		ToolResults:      allToolResults,
		ImageURLs:        allImageURLs,
		ReplacementRules: finalReplacementRules,
		Errors:           toolErrors,
	}, nil
}

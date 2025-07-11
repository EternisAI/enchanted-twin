package agent

import (
	"context"
	"strings"

	"github.com/openai/openai-go"

	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

type StreamDelta = ai.StreamDelta

// ExecuteStreamWithPrivacy uses privacy-enabled streaming with anonymization.
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
	} else {
		if strings.Contains(languageModel, "qwen3") {
			// HACK: qwen3 supports non-reasoning mode for faster responses with special tag `/no_think`
			messages[len(messages)-1] = openai.UserMessage(messages[len(messages)-1].OfUser.Content.OfString.Value + "/no_think")
		}
	}

	// Use privacy-enabled streaming
	result, err := a.aiService.CompletionsStreamWithPrivacy(ctx, messages, toolDefs, languageModel, onDelta)
	if err != nil {
		return AgentResponse{}, err
	}

	// For now, return basic response - tool calls will be handled in future iterations
	return AgentResponse{
		Content:   result.Message.Content,
		ToolCalls: result.Message.ToolCalls,
		ImageURLs: []string{}, // TODO: Handle image URLs from tools
	}, nil
}

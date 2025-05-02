package twinchat

import (
	"fmt"
	"strings"

	"github.com/openai/openai-go"

	"github.com/EternisAI/enchanted-twin/graph/model"
)

func ToOpenAIMessage(message model.Message) (openai.ChatCompletionMessageParamUnion, error) {
	switch strings.ToLower(message.Role.String()) {
	case "system":
		return openai.SystemMessage(*message.Text), nil
	case "user":
		return openai.UserMessage(*message.Text), nil
	case "assistant":
		return openai.AssistantMessage(*message.Text), nil
	case "developer":
		return openai.DeveloperMessage(*message.Text), nil
	case "tool":
		return openai.ToolMessage(*message.Text, ""), nil
	default:
		return openai.ChatCompletionMessageParamUnion{}, fmt.Errorf("unknown message role: %s", message.Role)
	}
}

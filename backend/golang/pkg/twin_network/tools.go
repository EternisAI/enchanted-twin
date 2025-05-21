package twin_network

import (
	"context"
	"errors"

	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
)

type TwinNetworkAPI interface {
	PostMessage(ctx context.Context, networkID string, threadID string, content string, authorPubKey string, signature string) error
}

type SendNetworkMessageTool struct {
	networkAPI TwinNetworkAPI
	agentKey   *AgentKey
}

func NewSendNetworkMessageTool(networkAPI TwinNetworkAPI, agentKey *AgentKey) *SendNetworkMessageTool {
	return &SendNetworkMessageTool{
		networkAPI: networkAPI,
		agentKey:   agentKey,
	}
}

func (e *SendNetworkMessageTool) Execute(ctx context.Context, inputs map[string]any) (types.ToolResult, error) {
	message, ok := inputs["message"].(string)
	if !ok {
		return nil, errors.New("message is not a string")
	}
	// threadID, ok := inputs["thread_id"].(string)
	// if !ok {
	// 	return nil, errors.New("thread_id is not a string")
	// }

	signature, err := e.agentKey.SignMessage(message)
	if err != nil {
		return nil, err
	}

	networkID := "default"

	err = e.networkAPI.PostMessage(ctx, networkID, "", message, e.agentKey.PubKeyHex(), signature)
	if err != nil {
		return nil, err
	}

	return types.SimpleToolResult("Message sent to twin network."), nil
}

func (e *SendNetworkMessageTool) Definition() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name:        "send_to_twin_network",
			Description: param.NewOpt("This tool sends a message to the public twin network"),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"message": map[string]string{
						"content":     "string",
						"description": "The message to be sent to the twin network on behalf of the human",
					},
					// "thread_id": map[string]string{
					// 	"content":     "string",
					// 	"description": "The thread ID to send the message to. Empty thread ID corresponds to a new thread.",
					// },
				},
				"required": []string{"message"},
			},
		},
	}
}

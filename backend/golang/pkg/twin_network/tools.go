package twin_network

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
	threadID, ok := inputs["thread_id"].(string)
	if !ok {
		threadID = ""
	}

	// Strip the # character if present in the thread ID
	originalThreadID := threadID
	threadID = strings.TrimPrefix(threadID, "#")

	// Add debug logging for thread ID processing
	fmt.Printf("Twin Network: Processing thread ID from '%s' to '%s'\n", originalThreadID, threadID)

	signature, err := e.agentKey.SignMessage(message)
	if err != nil {
		return nil, err
	}

	networkID := "default"

	err = e.networkAPI.PostMessage(ctx, networkID, threadID, message, e.agentKey.PubKeyHex(), signature)
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
					"thread_id": map[string]string{
						"content":     "string",
						"description": "The thread ID to send the message to. CRITICAL: When replying to a message from the twin network, you MUST copy the EXACT thread ID string from the original message. Look for 'thread id: #XXXX' in the conversation and use that exact value. DO NOT generate new thread IDs. Example: If you see 'thread id: #1234', you must use '#1234' as the thread_id parameter value. The # character will be automatically stripped internally when sending the message. Empty thread ID should only be used for completely new threads, never for replies.",
					},
				},
				"required": []string{"message", "thread_id"},
			},
		},
	}
}

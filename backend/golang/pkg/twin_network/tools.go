// Owner: slimane@eternis.ai
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

	originalThreadID := threadID
	threadID = strings.TrimPrefix(threadID, "#")

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
				"required": []string{"message"},
			},
		},
	}
}

type UpdateThreadStateTool struct {
	threadStore *ThreadStore
}

func NewUpdateThreadStateTool(threadStore *ThreadStore) *UpdateThreadStateTool {
	return &UpdateThreadStateTool{
		threadStore: threadStore,
	}
}

func (t *UpdateThreadStateTool) Execute(ctx context.Context, inputs map[string]any) (types.ToolResult, error) {
	threadID, ok := inputs["thread_id"].(string)
	if !ok || threadID == "" {
		return nil, errors.New("thread_id is required and must be a string")
	}

	stateStr, ok := inputs["state"].(string)
	if !ok || stateStr == "" {
		return nil, errors.New("state is required and must be a string")
	}

	var state ThreadState
	switch stateStr {
	case "ignored":
		state = ThreadStateIgnored
	case "completed":
		state = ThreadStateCompleted
	default:
		return nil, fmt.Errorf("invalid state: %s (must be 'ignored' or 'completed')", stateStr)
	}

	if err := t.threadStore.SetThreadState(ctx, threadID, state); err != nil {
		return nil, fmt.Errorf("failed to set thread state: %w", err)
	}

	return types.SimpleToolResult(fmt.Sprintf("Thread %s state updated to %s", threadID, state)), nil
}

func (t *UpdateThreadStateTool) Definition() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name:        "twin_network_update_thread",
			Description: param.NewOpt("This tool updates the state of a thread in the twin network"),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"thread_id": map[string]string{
						"content":     "string",
						"description": "The thread ID to update the state for. CRITICAL: You must provide the exact thread ID from the message, strip the # if present.",
					},
					"state": map[string]string{
						"content":     "string",
						"description": "The state to set for the thread. Must be one of: 'ignored' or 'completed'. Use 'ignored' to hide a thread from future processing, or 'completed' to mark it as done. Only use 'completed' after scheduling a task.",
					},
					"reason": map[string]string{
						"content":     "string",
						"description": "The reason for the state change. This will be displayed to the user in the UI.",
					},
				},
				"required": []string{"thread_id", "state", "reason"},
			},
		},
	}
}

package twin_network

import (
	"context"
	"fmt"

	"github.com/EternisAI/enchanted-twin/pkg/twin_network/graph/model"
	openai "github.com/openai/openai-go"
)

type MonitorNetworkActivityInput struct {
	NetworkID     string
	LastMessageID string
	Messages      []NetworkMessage
}

type QueryNetworkActivityInput struct {
	NetworkID string
	FromID    string
	Limit     int
}

func (a *TwinNetworkWorkflow) QueryNetworkActivity(ctx context.Context, input QueryNetworkActivityInput) ([]model.NetworkMessage, error) {
	a.logger.Debug("Querying network activity",
		"networkID", input.NetworkID,
		"fromID", input.FromID,
		"limit", input.Limit)

	allNewMessages, err := a.twinNetworkAPI.GetNewMessages(ctx, input.NetworkID, input.FromID, input.Limit)
	if err != nil {
		a.logger.Error("Failed to fetch new messages", "error", err)
		return nil, fmt.Errorf("failed to fetch new messages: %w", err)
	}

	return allNewMessages, nil
}

func (a *TwinNetworkWorkflow) EvaluateMessage(ctx context.Context, message NetworkMessage) (string, error) {
	personality, err := a.identityService.GetPersonality(ctx)
	if err != nil {
		a.logger.Error("Failed to get identity context for batch processing", "error", err, "networkID", message.NetworkID)
		return "", fmt.Errorf("failed to get identity context for batch: %w", err)
	}
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(fmt.Sprintf(`Analyze the conversation and provide your analysis in two parts:\n
		1. Reasoning: Your analysis of the conversation flow, message patterns, and the roles of each participant\n
		2. Response: A suggested next response that would be appropriate in this context\n

		If you think this message is useful to your human, send it to the chat by calling "send_to_chat" tool and specifying "chat_id" to be empty string.
		If the bulletin board message is interesting to your human and requires response and if you know the correct response call "send_bulletin_board_message" tool. 
		If you're missing some information nescessary to respond, only send message to your human chat.

		Here is some context about your personality and identity:

		Thread ID: %s
		%s`, personality, message.ThreadID)),
		openai.UserMessage(message.String()),
	}

	tools := a.toolRegistry.GetAll()

	response, err := a.agent.Execute(ctx, nil, messages, tools)
	if err != nil {
		a.logger.Error("Failed to execute agent", "error", err)
		return "", fmt.Errorf("failed to execute agent: %w", err)
	}

	return response.Content, nil
}

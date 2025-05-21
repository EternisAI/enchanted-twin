package twin_network

import (
	"context"
	"fmt"
	"strconv"

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

func (a *TwinNetworkWorkflow) QueryNetworkActivity(ctx context.Context, input QueryNetworkActivityInput) ([]NetworkMessage, error) {
	a.logger.Debug("Querying network activity",
		"networkID", input.NetworkID,
		"fromID", input.FromID,
		"limit", input.Limit)

	allNewMessages, err := a.twinNetworkAPI.GetNewMessages(ctx, input.NetworkID, input.FromID, input.Limit)
	if err != nil {
		a.logger.Error("Failed to fetch new messages", "error", err)
		return nil, fmt.Errorf("failed to fetch new messages: %w", err)
	}

	networkMessages := make([]NetworkMessage, len(allNewMessages))
	for i, msg := range allNewMessages {
		id, err := strconv.ParseInt(msg.ID, 10, 64)
		if err != nil {
			a.logger.Error("Failed to parse message ID", "error", err, "id", msg.ID)
			continue
		}

		myPubKey := a.agentKey.PubKeyHex()

		networkMessages[i] = NetworkMessage{
			ID:           id,
			AuthorPubKey: msg.AuthorPubKey,
			NetworkID:    msg.NetworkID,
			Content:      msg.Content,
			IsMine:       msg.AuthorPubKey == myPubKey,
			Signature:    msg.Signature,
		}
	}

	return networkMessages, nil
}

func (a *TwinNetworkWorkflow) EvaluateMessage(ctx context.Context, messages []NetworkMessage) (string, error) {
	if len(messages) == 0 {
		return "", nil
	}

	personality, err := a.identityService.GetPersonality(ctx)
	if err != nil {
		a.logger.Error("Failed to get identity context for batch processing",
			"error", err,
			"networkID", messages[0].NetworkID)
		return "", fmt.Errorf("failed to get identity context for batch: %w", err)
	}

	// Start with the system message
	chatMessages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(fmt.Sprintf(`Analyze the conversation and provide your analysis in two parts:\n
		1. Reasoning: Your analysis of the conversation flow, message patterns, and the roles of each participant\n
		2. Response: A suggested next response that would be appropriate in this context\n

		Always call "send_to_twin_network" tool to respond to the message.
		If you think this message is useful to your human, send it to the chat by calling "send_to_chat" tool and specifying "chat_id" to be empty string.

		If you're missing some information necessary to respond, only send message to your human chat.

		Here is some context about your personality and identity:

		Thread ID: %s
		%s`, messages[0].ThreadID, personality)),
	}

	// Convert each message into the appropriate format
	// Messages from the agent are assistant messages, others are user messages
	agentPubKey := a.agentKey.PubKeyHex()

	for _, msg := range messages {
		if msg.AuthorPubKey == agentPubKey {
			chatMessages = append(chatMessages, openai.AssistantMessage(msg.Content))
		} else {
			prefixedContent := fmt.Sprintf("[%s] %s", msg.AuthorPubKey, msg.Content)
			chatMessages = append(chatMessages, openai.UserMessage(prefixedContent))
		}
	}

	tools := a.toolRegistry.GetAll()

	response, err := a.agent.Execute(ctx, nil, chatMessages, tools)
	if err != nil {
		a.logger.Error("Failed to execute agent", "error", err)
		return "", fmt.Errorf("failed to execute agent: %w", err)
	}

	return response.Content, nil
}

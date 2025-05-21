package twin_network

import (
	"context"
	"fmt"
	"strconv"

	"github.com/EternisAI/enchanted-twin/graph/model"
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

	chatMessages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(fmt.Sprintf(`
		You are talking on behalf of your human with other twin agents talking on behalf of their humans.
		Your role is to interact with them and if anything is of value to your human, respond to it.

		Analyze the conversation and provide your analysis in two parts:\n
		1. Reasoning: Your analysis of the conversation flow, message patterns, and the roles of each participant\n
		2. Response: A suggested next response that would be appropriate in this context\n

		IMPORTANT: There are two separate communication channels:
		1. The twin network: Use "send_to_twin_network" ONLY when you want to respond to other agents
		2. Your human's chat: Use "send_to_chat" ONLY when you want to relay important information to your human

		DO NOT MIRROR OR REPEAT messages from the network back to the network.
		If you think a message is useful to your human, use ONLY the "send_to_chat" tool to forward it directly to your human.
		Only use "send_to_twin_network" when you have a NEW response to contribute to the conversation.

		If you're missing some information necessary from the human to respond, use the send_to_chat tool to ask your human.

		For example if someone ask if anyone is interested in an event, ask your human with the send_to_chat tool before booking the tickets.
		Then after confirmation book the ticket and send the message to the twin network with the "send_to_twin_network" tool.

		If for example someone invite you for a game of poker, ask the network participants who else is interested in playing.
		You need more than 2 participants to play poker obviously. You might also need a set of cards and chips.

		If you are not sure if your human would be interested in the message, ask your human with the send_to_chat tool.
		When forwarding the message to your human, specify that the messsages are coming from the twin network so that the human can understand the context.

		Call any tool necessary to move forward with the conversation into a productive conclusion: book calendar, send emails, etc.
		Do not linger undefinitely and be proactive.
		If the conversation isn't moving forward just stop answering.
		Be practical and remember to check your human calendar and also to check if the time/date make sense for your human.
		Call your human by his name.

		The other twin participants are identified by their public keys.

		Here is the latest information about your human's personality and identity:
		
		Thread ID: %s
		%s`, messages[0].ThreadID, personality)),
	}

	agentPubKey := a.agentKey.PubKeyHex()

	chatMessages = append(chatMessages, openai.UserMessage(fmt.Sprintf("Thread ID: %s", messages[0].ThreadID)))

	conversation := ""
	for _, msg := range messages {
		if msg.AuthorPubKey == agentPubKey {
			conversation += fmt.Sprintf("[you] %s\n", msg.Content)
		} else {
			conversation += fmt.Sprintf("[%s] %s\n", msg.AuthorPubKey, msg.Content)
		}
	}

	chatMessages = append(chatMessages, openai.UserMessage(conversation))

	tools := a.toolRegistry.GetAll()

	response, err := a.agent.Execute(ctx, nil, chatMessages, tools)
	if err != nil {
		a.logger.Error("Failed to execute agent", "error", err)
		return "", fmt.Errorf("failed to execute agent: %w", err)
	}

	return response.Content, nil
}

func (a *TwinNetworkWorkflow) GetChatMessages(ctx context.Context, chatID string) ([]*model.Message, error) {
	a.logger.Debug("Getting messages from chat", "chatID", chatID)

	messages, err := a.twinChatService.GetMessagesByChatId(ctx, chatID)
	if err != nil {
		a.logger.Error("Failed to get chat messages", "error", err, "chatID", chatID)
		return nil, fmt.Errorf("failed to get chat messages: %w", err)
	}

	return messages, nil
}

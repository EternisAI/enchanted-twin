package twin_network

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/EternisAI/enchanted-twin/graph/model"
	openai "github.com/openai/openai-go"
)

type MonitorNetworkActivityInput struct {
	NetworkID     string
	LastTimestamp time.Time
	Messages      []NetworkMessage
}

type QueryNetworkActivityInput struct {
	NetworkID string
	FromTime  time.Time
	Limit     int
}

type ThreadInfo struct {
	AuthorPubKey string
	ThreadID     string
	ID           string
	Messages     []NetworkMessage
}

func (a *TwinNetworkWorkflow) QueryNetworkActivity(ctx context.Context, input QueryNetworkActivityInput) ([]ThreadInfo, error) {
	a.logger.Debug("Querying network activity",
		"networkID", input.NetworkID,
		"fromTime", input.FromTime,
		"limit", input.Limit)

	threads, err := a.twinNetworkAPI.GetNewMessages(ctx, input.NetworkID, input.FromTime, input.Limit)
	if err != nil {
		a.logger.Error("Failed to fetch new threads", "error", err)
		return nil, fmt.Errorf("failed to fetch new threads: %w", err)
	}

	threadsList := make([]ThreadInfo, 0, len(threads))

	for _, thread := range threads {
		threadID := thread.ID
		threadMessages := make([]NetworkMessage, 0, len(thread.Messages))
		authorPubKey := ""

		for _, msg := range thread.Messages {
			id, err := strconv.ParseInt(msg.ID, 10, 64)
			if err != nil {
				a.logger.Error("Failed to parse message ID", "error", err, "id", msg.ID)
				continue
			}

			createdAt, err := time.Parse(time.RFC3339, msg.CreatedAt)
			if err != nil {
				a.logger.Error("Failed to parse message timestamp", "error", err, "createdAt", msg.CreatedAt)
				continue
			}

			if authorPubKey == "" {
				authorPubKey = msg.AuthorPubKey
			}

			threadMessages = append(threadMessages, NetworkMessage{
				ID:           id,
				AuthorPubKey: msg.AuthorPubKey,
				NetworkID:    msg.NetworkID,
				Content:      msg.Content,
				IsMine:       msg.IsMine,
				Signature:    msg.Signature,
				CreatedAt:    createdAt,
				ThreadID:     threadID,
			})
		}

		if len(threadMessages) > 0 {
			sort.Slice(threadMessages, func(i, j int) bool {
				return threadMessages[i].CreatedAt.Before(threadMessages[j].CreatedAt)
			})

			threadsList = append(threadsList, ThreadInfo{
				AuthorPubKey: authorPubKey,
				ThreadID:     threadID,
				ID:           threadID,
				Messages:     threadMessages,
			})
		}
	}

	return threadsList, nil
}

func (a *TwinNetworkWorkflow) EvaluateMessage(ctx context.Context, messages []NetworkMessage, threadAuthor string) (string, error) {
	if len(messages) == 0 {
		return "", nil
	}

	userProfile, err := a.userStorage.GetUserProfile(ctx)
	if err != nil {
		return "", err
	}

	personality, err := a.identityService.GetPersonality(ctx)
	if err != nil {
		a.logger.Error("Failed to get identity context for batch processing",
			"error", err,
			"networkID", messages[0].NetworkID)
		return "", fmt.Errorf("failed to get identity context for batch: %w", err)
	}
	systemPrompt := fmt.Sprintf(`
	You are the digital twin of one human.
	
	Your job for every incoming Twin-Network message is to decide whether to:
	  • forward it to your human
	  • silently ignore it  
	  • if the author of the thread concludes the thread, then use the tool *schedule_task* to create a task for your human
	
	━━━━━━━━━━  DECISION RULE  ━━━━━━━━━━
	1. Check the proposal against your human's stated interests/dislikes.
	2. If it clearly conflicts with a dislike (e.g. "coffee" when bio says "I hate coffee"):
		 → IGNORE the message entirely.
		 → Do **NOT** call *send_to_chat*.
		 → Do **NOT** call *send_to_twin_network*.
	3. If it obviously matches an interest, or you are genuinely unsure:
		 → Forward the proposal with *send_to_chat* and wait for guidance.
	4. Never ask for confirmation on something that contradicts a known dislike.
	5. If the proposal is'nt realistically feasible (for example going to the moon or meeting in different countries in a short amount of time), ignore it.
	6. Do not forward messages unless there isn't a decision to be made. If the user already said yes, then no need to ask again. Be productive and dont linger.
	
	━━━━━━━━━━  TOOL USAGE  ━━━━━━━━━━
	• *send_to_chat*  – only for aligned or uncertain proposals, or to report completed actions.
	• *send_to_twin_network* – use **only** after your human explicitly approves participation or when wrapping up a completed proposal.
	• Do **NOT** echo network messages back to the network.
	• Once the author marks a proposal completed, stop sending network messages except for essential wrap-up actions (calendar booking, email, etc.).
	• *schedule_task* – use this tool to create a task for your human, all threads must be concluded before using this tool
	━━━━━━━━━━  EXAMPLES  ━━━━━━━━━━
	✘ Incoming: "Coffee 2 pm at 381 Castro Street."
	   —> Ignore (no tools used).
	
	✔ Incoming: "Poker night Friday 8 pm."
	   —> Use *send_to_chat* asking whether to join; if yes, reply on the network and schedule the event.

	   then organizer confirms by sending a new message: "Perfect, I'll see you there!"
	   —> Use tool to create a task for your human
	
	Be concise, proactive, and drop the thread if it stalls.
	
	Thread ID: %s  
	Author public key: %s
	
	Human profile (top decision factor):  
	%s
	`, messages[0].ThreadID, threadAuthor, personality)

	if userProfile.Name != nil {
		systemPrompt += fmt.Sprintf("Human's name: %s\n", *userProfile.Name)
	}
	if userProfile.Bio != nil {
		systemPrompt += fmt.Sprintf("Human's interests/bio: %s\n", *userProfile.Bio)
	}

	agentPubKey := a.agentKey.PubKeyHex()

	userMessage := fmt.Sprintf("Thread ID: %s. ", messages[0].ThreadID)

	for _, msg := range messages {
		if msg.AuthorPubKey == agentPubKey {
			userMessage += fmt.Sprintf("[%s](You) %s.\n", msg.AuthorPubKey, msg.Content)
		} else {
			userMessage += fmt.Sprintf("[%s] %s.\n", msg.AuthorPubKey, msg.Content)
		}
	}

	chatMessages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
		openai.UserMessage(userMessage),
	}

	a.logger.Debug("Evaluating message", "system_prompt", systemPrompt, "user_message", userMessage)
	tools := a.toolRegistry.GetAll()
	for _, tool := range tools {
		a.logger.Debug("Tool", "tool", tool.Definition().Function.Name)
	}

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

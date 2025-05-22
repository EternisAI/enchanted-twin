// Owner: slimane@eternis.ai
package twin_network

import (
	"context"
	"fmt"
	"sort"
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
			createdAt, err := time.Parse(time.RFC3339, msg.CreatedAt)
			if err != nil {
				a.logger.Error("Failed to parse message timestamp", "error", err, "createdAt", msg.CreatedAt)
				continue
			}

			if authorPubKey == "" {
				authorPubKey = msg.AuthorPubKey
			}

			threadMessages = append(threadMessages, NetworkMessage{
				ID:           msg.ID,
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

func (a *TwinNetworkWorkflow) EvaluateMessage(ctx context.Context, messages []NetworkMessage, threadAuthor string, isOrganizer bool) (string, error) {
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
	var systemPrompt string
	if isOrganizer {
		systemPrompt = fmt.Sprintf(`
	You are the digital twin of one human.

	You are communicating with other twins through a network.
	Messages related to a proposal live in a thread.

	Your job as organizer TWIN is:
	If you are the twin of the organizer/author of the thread, then you must communicate a lot about what's going on with your human using send_to_chat tool
	until the author of the thread confirms that everything is set or that the proposal is cancelled.
	We mustn't leave the other twins in the dark.

 
	━━━━━━━━━━  TOOL USAGE  ━━━━━━━━━━
	• *send_to_chat*  – keep human systematically informed about whats going on in the thread.
	• *send_to_twin_network* – use **only** after your human explicitly approves participation or when wrapping up a completed proposal.
	• Do **NOT** echo network messages back to the network.
	• *schedule_task* – use this tool to create a task for your human.
	• *update_thread* – DO NOT USE THIS TOOL to ignore the thread. It is for the participants twins only.
	• Once the author marks a proposal completed, stop sending network messages except for essential wrap-up actions (calendar booking, email, etc.).
	
	━━━━━━━━━━  EXAMPLES  ━━━━━━━━━━
	 "Inviting Coffee 2 pm at 381 Castro Street."

	 Other twin replies: "I'm interested in joining"

	 You notify your human: "Someone is interested in joining" using send_to_chat tool

	 You ask regulalry your human to confirm the event if everything is set.
	 Once event is confirmed, you use schedule_task tool to create a task for your human and stop feeding that thread.


	Be concise, proactive, and drop the thread if it stalls.
	
	Thread ID: %s  
	Author public key: %s
	
	Human profile (top decision factor):  
	%s
	`, messages[0].ThreadID, threadAuthor, personality)
	} else {
		systemPrompt = fmt.Sprintf(`
You are the digital twin of one human.

	You are communicating with other twins through a network.
	You might receive or send proposal through messages.
	Messages related to a proposal live in a thread.
	
	Your job as participant TWIN (not organizer) is to:
	  • forward it to your human to collect necessary information
	  • silently ignore the thread if you are not interested
	  • mark the thread as complete if you made a decision about to act on the proposal or not
	  • do nothing and wait for the author of the thread to conclude the thread (note: different from ignoring the thread)
	   
	  additionally:
	  • if the author of the thread concludes the thread, then use the tool *schedule_task* to create a task for your human
	  • if you decided to act on the proposal, then also use the tool *schedule_task* 
	  • when you make committing decision like joining an event do not forget to notify your human using *send_to_chat* tool 

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
	7. Be very practical if you get a time make sure that the timezone is correct before scheduling the task.
	
	━━━━━━━━━━  TOOL USAGE  ━━━━━━━━━━
	• *send_to_chat*  – only for aligned or uncertain proposals, or to report completed actions.
	• *send_to_twin_network* – use **only** after your human explicitly approves participation or when wrapping up a completed proposal.
	• Do **NOT** echo network messages back to the network.
	• Once the author marks a proposal completed, stop sending network messages except for essential wrap-up actions (calendar booking, email, etc.).
	• *schedule_task* – use this tool to create a task for your human, all threads must be concluded before using this tool
	• *update_thread* – use this tool to update the state of a thread, use this tool to mark a thread as completed or ignored. Only use this tool after the task is scheduled.
	
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
	}
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

// GetThreadState gets the state of a thread
func (a *TwinNetworkWorkflow) GetThreadState(ctx context.Context, threadID string) (ThreadState, error) {
	state, err := a.threadStore.GetThreadState(ctx, threadID)
	if err != nil && err.Error() == "failed to get thread state: sql: no rows in result set" {
		a.logger.Debug("Thread not found in store, initializing", "threadID", threadID)
		if err := a.threadStore.SetThreadState(ctx, threadID, ThreadStateNone); err != nil {
			return ThreadStateNone, err
		}
		return ThreadStateNone, nil
	}
	return state, err
}

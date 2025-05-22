// Owner: slimane@eternis.ai
package twin_network

import (
	"encoding/json"
	"fmt"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/EternisAI/enchanted-twin/graph/model"
)

const (
	NetworkMonitorWorkflowID = "network-monitor-workflow"
)

type NetworkMonitorInput struct {
	NetworkID     string
	LastTimestamp time.Time
	ChatID        string
}

type NetworkMonitorOutput struct {
	ProcessedMessages int
	LastTimestamp     time.Time
	ChatID            string
}

func (w *TwinNetworkWorkflow) NetworkMonitorWorkflow(ctx workflow.Context, input NetworkMonitorInput) (*NetworkMonitorOutput, error) {
	options := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
	})

	var resolvedLastTimestamp time.Time
	var chatID string

	var lastCompletionOutput NetworkMonitorOutput
	if workflow.HasLastCompletionResult(ctx) {
		if err := workflow.GetLastCompletionResult(ctx, &lastCompletionOutput); err == nil {
			resolvedLastTimestamp = lastCompletionOutput.LastTimestamp
			chatID = lastCompletionOutput.ChatID
		} else {
			workflow.GetLogger(ctx).Error("NetworkMonitorWorkflow: Failed to get last completion result. Using current input's LastTimestamp.", "error", err)
			resolvedLastTimestamp = input.LastTimestamp
			chatID = input.ChatID
		}
	} else {
		resolvedLastTimestamp = input.LastTimestamp
		chatID = input.ChatID
	}

	activityInput := NetworkMonitorInput{
		NetworkID:     input.NetworkID,
		LastTimestamp: resolvedLastTimestamp,
		ChatID:        chatID,
	}

	lookbackTime := resolvedLastTimestamp.Add(-30 * time.Minute)
	queryInput := QueryNetworkActivityInput{
		NetworkID: input.NetworkID,
		FromTime:  lookbackTime,
		Limit:     30,
	}

	var threads []ThreadInfo
	err := workflow.ExecuteActivity(options, w.QueryNetworkActivity, queryInput).Get(ctx, &threads)
	if err != nil {
		return nil, fmt.Errorf("failed to query network activity: %w", err)
	}

	totalProcessedMessages := 0
	latestTimestamp := resolvedLastTimestamp

	workflow.GetLogger(ctx).Info("Processing threads", "threads", threads)
	for _, thread := range threads {
		threadID := thread.ThreadID
		messages := thread.Messages

		var threadState ThreadState
		err := workflow.ExecuteActivity(options, w.GetThreadState, threadID).Get(ctx, &threadState)
		if err != nil {
			workflow.GetLogger(ctx).Error("Failed to get thread state", "error", err, "threadID", threadID)
		} else if threadState == ThreadStateIgnored || threadState == ThreadStateCompleted {
			workflow.GetLogger(ctx).Info("Skipping thread with state", "threadID", threadID, "state", threadState)
			continue
		}

		workflow.GetLogger(ctx).Info("Processing thread", "threadID", threadID, "messageCount", len(messages), "networkID", input.NetworkID)

		if len(messages) == 0 {
			continue
		}

		threadLatestTimestamp := messages[0].CreatedAt
		for _, msg := range messages {
			if msg.CreatedAt.After(threadLatestTimestamp) {
				threadLatestTimestamp = msg.CreatedAt
			}
		}

		if threadLatestTimestamp.After(latestTimestamp) {
			latestTimestamp = threadLatestTimestamp
		}

		if !threadLatestTimestamp.After(activityInput.LastTimestamp) {
			workflow.GetLogger(ctx).Info("No new messages in thread", "threadID", threadID)
			continue
		}

		authorPubKey := thread.AuthorPubKey

		if !messages[0].IsMine && chatID != "" {
			var chatMessages []*model.Message
			err := workflow.ExecuteActivity(options, w.GetChatMessages, chatID).Get(ctx, &chatMessages)
			if err != nil {
				workflow.GetLogger(ctx).Error("Failed to get chat messages", "error", err, "chatID", chatID)
			} else {
				workflow.GetLogger(ctx).Info("Retrieved chat messages", "count", len(chatMessages), "chatID", chatID)

				if len(chatMessages) > 0 {
					var lastUserMessage *model.Message
					for i := len(chatMessages) - 1; i >= 0; i-- {
						if chatMessages[i].Role == model.RoleUser {
							lastUserMessage = chatMessages[i]
							break
						}
					}

					if lastUserMessage != nil {
						workflow.GetLogger(ctx).Info("Found user response in chat", "message", *lastUserMessage.Text)
						messages = append([]NetworkMessage{
							{
								Content:   fmt.Sprintf("User response from chat: %s", *lastUserMessage.Text),
								IsMine:    true,
								CreatedAt: time.Now(),
								ThreadID:  threadID,
							},
						}, messages...)
					}
				}
			}
		}

		isOrganizer := authorPubKey == input.NetworkID
		if !messages[0].IsMine {
			var response string
			err = workflow.ExecuteActivity(options, w.EvaluateMessage, messages, authorPubKey, isOrganizer).Get(ctx, &response)
			if err != nil {
				workflow.GetLogger(ctx).Error("Failed to evaluate messages", "error", err)
			} else if response != "" {
				workflow.GetLogger(ctx).Info("Successfully evaluated messages", "response", response)

				if newChatID := w.extractChatIDFromResponse(response); newChatID != "" {
					chatID = newChatID
					workflow.GetLogger(ctx).Info("Updated chat ID from response", "chatID", chatID)
				}
			}
		} else {
			workflow.GetLogger(ctx).Info("Skipping evaluation as the last message is from the user", "messageID", messages[0].ID)
		}

		totalProcessedMessages += len(messages)
	}

	if totalProcessedMessages == 0 {
		workflow.GetLogger(ctx).Info("No new messages found in any thread", "networkID", input.NetworkID)
	} else {
		workflow.GetLogger(ctx).Info("Processed messages across all threads",
			"totalMessages", totalProcessedMessages,
			"threadCount", len(threads),
			"networkID", input.NetworkID)
	}

	output := NetworkMonitorOutput{
		ProcessedMessages: totalProcessedMessages,
		LastTimestamp:     latestTimestamp,
		ChatID:            chatID,
	}

	return &output, nil
}

func (w *TwinNetworkWorkflow) extractChatIDFromResponse(response string) string {
	if len(response) > 0 && response[0] == '{' {
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(response), &result); err == nil {
			if toolParams, ok := result["ToolParams"].(map[string]interface{}); ok {
				if chatID, ok := toolParams["chat_id"].(string); ok {
					return chatID
				}
			}
		}
	}
	return ""
}

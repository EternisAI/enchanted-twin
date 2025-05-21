package twin_network

import (
	"fmt"
	"strconv"
	"time"

	"go.temporal.io/sdk/workflow"
)

const (
	NetworkMonitorWorkflowID = "network-monitor-workflow"
)

type NetworkMonitorInput struct {
	NetworkID     string
	LastMessageID int64
}

type NetworkMonitorOutput struct {
	ProcessedMessages int
	LastMessageID     int64
}

func (w *TwinNetworkWorkflow) NetworkMonitorWorkflow(ctx workflow.Context, input NetworkMonitorInput) (*NetworkMonitorOutput, error) {
	options := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
	})

	var resolvedLastMessageID int64

	var lastCompletionOutput NetworkMonitorOutput
	if workflow.HasLastCompletionResult(ctx) {
		if err := workflow.GetLastCompletionResult(ctx, &lastCompletionOutput); err == nil {
			resolvedLastMessageID = lastCompletionOutput.LastMessageID
		} else {
			workflow.GetLogger(ctx).Error("NetworkMonitorWorkflow: Failed to get last completion result. Using current input's LastMessageID.", "error", err)
			resolvedLastMessageID = input.LastMessageID
		}
	} else {
		resolvedLastMessageID = input.LastMessageID
	}

	activityInput := NetworkMonitorInput{
		NetworkID:     input.NetworkID,
		LastMessageID: resolvedLastMessageID,
	}

	queryInput := QueryNetworkActivityInput{
		NetworkID: input.NetworkID,
		FromID:    strconv.FormatInt(resolvedLastMessageID, 10),
		Limit:     30,
	}
	var allNewMessages []NetworkMessage
	err := workflow.ExecuteActivity(options, w.QueryNetworkActivity, queryInput).Get(ctx, &allNewMessages)
	if err != nil {
		return nil, fmt.Errorf("failed to query network activity: %w", err)
	}
	workflow.GetLogger(ctx).Info("Retrieved new messages", "count", len(allNewMessages), "networkID", input.NetworkID)

	if len(allNewMessages) > 0 {
		activityInput.LastMessageID = allNewMessages[0].ID

		var response string
		err = workflow.ExecuteActivity(options, w.EvaluateMessage, allNewMessages).Get(ctx, &response)
		if err != nil {
			workflow.GetLogger(ctx).Error("Failed to evaluate messages", "error", err)
		} else if response != "" {
			workflow.GetLogger(ctx).Info("Successfully evaluated messages", "response", response)
		}
	} else {
		workflow.GetLogger(ctx).Info("No new messages found", "networkID", input.NetworkID)
		return &NetworkMonitorOutput{
			ProcessedMessages: 0,
			LastMessageID:     resolvedLastMessageID,
		}, nil
	}

	output := NetworkMonitorOutput{
		ProcessedMessages: len(allNewMessages),
		LastMessageID:     activityInput.LastMessageID,
	}

	return &output, nil
}

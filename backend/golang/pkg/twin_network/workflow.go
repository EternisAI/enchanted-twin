package twin_network

import (
	"encoding/json"
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
	LastMessageID interface{}
}

type NetworkMonitorOutput struct {
	ProcessedMessages int
	LastMessageID     string
}

func (w *TwinNetworkWorkflow) NetworkMonitorWorkflow(ctx workflow.Context, input NetworkMonitorInput) (*NetworkMonitorOutput, error) {
	options := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
	})

	var resolvedLastMessageID string

	// Process LastMessageID from the current workflow input (which might be from initial start)
	// This part handles the initial unmarshalling issue if a number is passed.
	currentInputLastMessageIDStr := ""
	if input.LastMessageID != nil {
		switch v := input.LastMessageID.(type) {
		case string:
			currentInputLastMessageIDStr = v
		case json.Number:
			currentInputLastMessageIDStr = v.String()
		case float64:
			currentInputLastMessageIDStr = strconv.FormatFloat(v, 'f', -1, 64)
		case int:
			currentInputLastMessageIDStr = strconv.Itoa(v)
		case int64:
			currentInputLastMessageIDStr = strconv.FormatInt(v, 10)
		default:
			workflow.GetLogger(ctx).Warn(fmt.Sprintf("NetworkMonitorWorkflow: Unexpected type for input.LastMessageID. Defaulting to empty. Type: %T, Value: %v", v, v))
			currentInputLastMessageIDStr = "" // Default to empty or handle error
		}
	}

	// Check for last completion result (for continue-as-new)
	var lastCompletionOutput NetworkMonitorOutput
	if workflow.HasLastCompletionResult(ctx) {
		if err := workflow.GetLastCompletionResult(ctx, &lastCompletionOutput); err == nil {
			// LastMessageID from previous run's output (NetworkMonitorOutput.LastMessageID is string)
			resolvedLastMessageID = lastCompletionOutput.LastMessageID
		} else {
			workflow.GetLogger(ctx).Error("NetworkMonitorWorkflow: Failed to get last completion result. Using current input's LastMessageID.", "error", err)
			resolvedLastMessageID = currentInputLastMessageIDStr
		}
	} else {
		// No last completion result, use the processed current input's LastMessageID
		resolvedLastMessageID = currentInputLastMessageIDStr
	}

	// Prepare the input for the activity.
	// Since NetworkMonitorInput.LastMessageID is now interface{}, resolvedLastMessageID (a string)
	// will be passed as an interface{} containing a string.
	activityInput := NetworkMonitorInput{
		NetworkID:     input.NetworkID,
		LastMessageID: resolvedLastMessageID,
	}

	var output NetworkMonitorOutput
	// The activity w.MonitorNetworkActivity will receive activityInput.
	// Inside the activity, input.LastMessageID will be an interface{} containing the string.
	err := workflow.ExecuteActivity(options, w.MonitorNetworkActivity, activityInput).Get(ctx, &output)
	if err != nil {
		return nil, err
	}

	return &output, nil
}

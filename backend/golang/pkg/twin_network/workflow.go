package twin_network

import (
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

	// Get the last completion result to track the last message ID
	var lastOutput *NetworkMonitorOutput
	if workflow.HasLastCompletionResult(ctx) {
		err := workflow.GetLastCompletionResult(ctx, &lastOutput)
		if err != nil {
			return nil, err
		}
	}

	// If we have a last output, use its last message ID
	if lastOutput != nil {
		input.LastMessageID = lastOutput.LastMessageID
	}

	var output NetworkMonitorOutput
	err := workflow.ExecuteActivity(options, w.MonitorNetworkActivity, input).Get(ctx, &output)
	if err != nil {
		return nil, err
	}

	return &output, nil
}

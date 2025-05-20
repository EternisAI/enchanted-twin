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

	var lastOutput *NetworkMonitorOutput
	if workflow.HasLastCompletionResult(ctx) {
		err := workflow.GetLastCompletionResult(ctx, &lastOutput)
		if err != nil {
			return nil, err
		}
	}

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

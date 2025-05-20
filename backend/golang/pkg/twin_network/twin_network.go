package twin_network

import (
	"context"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/identity"
	"github.com/charmbracelet/log"
	"go.temporal.io/sdk/client"
)

type TwinNetworkWorkflow struct {
	ai               *ai.Service
	logger           *log.Logger
	networkServerURL string
	readNetworkTool  *ReadNetworkTool
	agentKey         AgentKey
	identityService  *identity.IdentityService
}

func NewTwinNetworkWorkflow(ai *ai.Service, logger *log.Logger, networkServerURL string, agentKey AgentKey) *TwinNetworkWorkflow {
	return &TwinNetworkWorkflow{
		ai:               ai,
		logger:           logger,
		networkServerURL: networkServerURL,
		readNetworkTool:  NewReadNetworkTool(logger, ai, "gpt-4.1-mini"),
		agentKey:         agentKey,
	}
}

func (w *TwinNetworkWorkflow) ScheduleNetworkMonitor(logger *log.Logger, temporalClient client.Client) error {
	ctx := context.Background()
	workflowOptions := client.StartWorkflowOptions{
		ID:        NetworkMonitorWorkflowID,
		TaskQueue: "default",
	}

	input := NetworkMonitorInput{
		NetworkID: "default",
	}

	_, err := temporalClient.ExecuteWorkflow(
		ctx,
		workflowOptions,
		w.NetworkMonitorWorkflow,
		input,
	)
	if err != nil {
		return err
	}

	// Schedule the workflow to run every minute
	scheduleOptions := client.ScheduleOptions{
		ID: NetworkMonitorWorkflowID + "-schedule",
		Spec: client.ScheduleSpec{
			Intervals: []client.ScheduleIntervalSpec{
				{
					Every: time.Minute,
				},
			},
		},
		Action: &client.ScheduleWorkflowAction{
			Workflow:                 w.NetworkMonitorWorkflow,
			WorkflowExecutionTimeout: time.Hour,
			WorkflowTaskTimeout:      time.Minute,
			TaskQueue:                "default",
			Args:                     []interface{}{input},
		},
	}

	_, err = temporalClient.ScheduleClient().Create(ctx, scheduleOptions)
	return err
}

func (a *TwinNetworkWorkflow) RegisterActivities(w interface{ RegisterActivity(interface{}) }) {
	w.RegisterActivity(a.MonitorNetworkActivity)
}

func (a *TwinNetworkWorkflow) RegisterWorkflows(w interface{ RegisterWorkflow(interface{}) }) {
	w.RegisterWorkflow(a.NetworkMonitorWorkflow)
}

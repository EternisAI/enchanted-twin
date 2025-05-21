package twin_network

import (
	"context"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/agent"
	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/identity"
	"github.com/EternisAI/enchanted-twin/pkg/twin_network/api"
	"github.com/EternisAI/enchanted-twin/pkg/twinchat"
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
	twinChatService  *twinchat.Service
	agent            *agent.Agent
	toolRegistry     tools.ToolRegistry
	twinNetworkAPI   api.TwinNetworkAPI
}

type TwinNetworkWorkflowInput struct {
	AI               *ai.Service
	Logger           *log.Logger
	NetworkServerURL string
	AgentKey         AgentKey
	IdentityService  *identity.IdentityService
	TwinChatService  *twinchat.Service
	Agent            *agent.Agent
	ToolRegistry     tools.ToolRegistry
	TwinNetworkAPI   api.TwinNetworkAPI
}

func NewTwinNetworkWorkflow(input TwinNetworkWorkflowInput) *TwinNetworkWorkflow {
	return &TwinNetworkWorkflow{
		ai:               input.AI,
		logger:           input.Logger,
		networkServerURL: input.NetworkServerURL,
		readNetworkTool:  NewReadNetworkTool(input.Logger, input.AI, "gpt-4.1-mini"),
		agentKey:         input.AgentKey,
		identityService:  input.IdentityService,
		twinChatService:  input.TwinChatService,
		agent:            input.Agent,
		toolRegistry:     input.ToolRegistry,
		twinNetworkAPI:   input.TwinNetworkAPI,
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
	w.RegisterActivity(a.QueryNetworkActivity)
}

func (a *TwinNetworkWorkflow) RegisterWorkflows(w interface{ RegisterWorkflow(interface{}) }) {
	w.RegisterWorkflow(a.NetworkMonitorWorkflow)
}

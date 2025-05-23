// Owner: slimane@eternis.ai
package twin_network

import (
	"context"
	"time"

	"github.com/charmbracelet/log"
	"go.temporal.io/sdk/client"

	"github.com/EternisAI/enchanted-twin/pkg/agent"
	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/pkg/identity"
	"github.com/EternisAI/enchanted-twin/pkg/twin_network/api"
	"github.com/EternisAI/enchanted-twin/pkg/twinchat"
)

type TwinNetworkWorkflow struct {
	ai               *ai.Service
	logger           *log.Logger
	networkServerURL string
	agentKey         AgentKey
	identityService  *identity.IdentityService
	twinChatService  *twinchat.Service
	userStorage      *db.Store
	agent            *agent.Agent
	toolRegistry     tools.ToolRegistry
	twinNetworkAPI   api.TwinNetworkAPI
	threadStore      *ThreadStore
}

type TwinNetworkWorkflowInput struct {
	AI               *ai.Service
	Logger           *log.Logger
	NetworkServerURL string
	AgentKey         AgentKey
	IdentityService  *identity.IdentityService
	TwinChatService  *twinchat.Service
	UserStorage      *db.Store
	Agent            *agent.Agent
	ToolRegistry     tools.ToolRegistry
	TwinNetworkAPI   *api.TwinNetworkAPI
}

func NewTwinNetworkWorkflow(input TwinNetworkWorkflowInput) *TwinNetworkWorkflow {
	threadStore := NewThreadStore(input.UserStorage)

	workflow := &TwinNetworkWorkflow{
		ai:               input.AI,
		logger:           input.Logger,
		networkServerURL: input.NetworkServerURL,
		agentKey:         input.AgentKey,
		identityService:  input.IdentityService,
		twinChatService:  input.TwinChatService,
		userStorage:      input.UserStorage,
		agent:            input.Agent,
		toolRegistry:     input.ToolRegistry,
		twinNetworkAPI:   *input.TwinNetworkAPI,
		threadStore:      threadStore,
	}

	// Register the UpdateThreadStateTool with the agent
	if input.Agent != nil && input.ToolRegistry != nil {
		updateThreadTool := NewUpdateThreadStateTool(threadStore)
		_ = input.ToolRegistry.Register(updateThreadTool)
	}

	return workflow
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
					Every: time.Second * 20,
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
	w.RegisterActivity(a.EvaluateMessage)
	w.RegisterActivity(a.QueryNetworkActivity)
	w.RegisterActivity(a.GetChatMessages)
	w.RegisterActivity(a.GetThreadState)
	w.RegisterActivity(a.SetThreadChatID)
	w.RegisterActivity(a.GetThreadChatID)
}

func (a *TwinNetworkWorkflow) RegisterWorkflows(w interface{ RegisterWorkflow(interface{}) }) {
	w.RegisterWorkflow(a.NetworkMonitorWorkflow)
}

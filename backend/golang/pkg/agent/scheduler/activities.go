package scheduler

import (
	"context"
	"fmt"

	"github.com/charmbracelet/log"

	"github.com/EternisAI/enchanted-twin/pkg/agent"
	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	openai "github.com/openai/openai-go"
	"go.temporal.io/sdk/worker"
)

type TaskSchedulerActivities struct {
	AIService        *ai.Service
	Agent            *agent.Agent
	ToolsRegistry    tools.ToolRegistry
	CompletionsModel string
	logger           *log.Logger
}

func NewTaskSchedulerActivities(logger *log.Logger, AIService *ai.Service, Agent *agent.Agent, Tools tools.ToolRegistry, completionsModel string) *TaskSchedulerActivities {
	return &TaskSchedulerActivities{
		AIService:        AIService,
		Agent:            Agent,
		ToolsRegistry:    Tools,
		CompletionsModel: completionsModel,
		logger:           logger,
	}
}

func (s *TaskSchedulerActivities) RegisterWorkflowsAndActivities(worker worker.Worker) {
	worker.RegisterWorkflow(TaskScheduleWorkflow)
	worker.RegisterActivity(s.executeActivity)
}

type ExecuteTaskActivityInput struct {
	Task           string
	PreviousResult *string
	ChatID         string
}

func (s *TaskSchedulerActivities) executeActivity(ctx context.Context, input ExecuteTaskActivityInput) (string, error) {
	systemPrompt := "You are a personal assistant and digital twin of a human. Your goal is to help your human in any way possible and help them to improve themselves. You are smart and wise and aim understand your human at a deep level. When you are asked to search the web, you should use the `perplexity_ask` tool if it exists. You must send a message after completing all tool calls. You must ensure that the final message includes the answer to the original task. If user's task requires communicating the result back to them, you should use the `send_to_chat` tool unless user specified different output channel like email, telegram or other."

	if input.PreviousResult != nil {
		systemPrompt += fmt.Sprintf("\n\nPrevious result: %s.", *input.PreviousResult)
	}

	systemPrompt += fmt.Sprintf("\n\nChat ID: %s.", input.ChatID)

	fmt.Println("executeActivity systemPrompt", systemPrompt)

	tools := s.ToolsRegistry.Excluding("schedule_task").GetAll()
	for _, tool := range tools {
		s.logger.Info("Tool", "name", tool.Definition().Function.Name, "description", tool.Definition().Function.Description)
	}

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
		openai.UserMessage(input.Task),
	}
	response, err := s.Agent.Execute(ctx, map[string]any{}, messages, tools)
	if err != nil {
		return "", err
	}
	return response.String(), nil
}

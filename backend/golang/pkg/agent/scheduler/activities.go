package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	openai "github.com/openai/openai-go"
	"go.temporal.io/sdk/worker"

	"github.com/EternisAI/enchanted-twin/graph/model"
	"github.com/EternisAI/enchanted-twin/pkg/agent"
	"github.com/EternisAI/enchanted-twin/pkg/agent/notifications"
	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/helpers"
)

type userProfile interface {
	GetUserProfile(ctx context.Context) (*model.UserProfile, error)
}

type TaskSchedulerActivities struct {
	AIService        *ai.Service
	Agent            *agent.Agent
	ToolsRegistry    tools.ToolRegistry
	CompletionsModel string
	logger           *log.Logger
	userStorage      userProfile
	notifications    *notifications.Service
}

func NewTaskSchedulerActivities(logger *log.Logger, AIService *ai.Service, Agent *agent.Agent, Tools tools.ToolRegistry, completionsModel string, userStorage userProfile, notifications *notifications.Service) *TaskSchedulerActivities {
	return &TaskSchedulerActivities{
		AIService:        AIService,
		Agent:            Agent,
		ToolsRegistry:    Tools,
		CompletionsModel: completionsModel,
		logger:           logger,
		userStorage:      userStorage,
		notifications:    notifications,
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
	Notify         bool
	Name           string
}

func (s *TaskSchedulerActivities) executeActivity(ctx context.Context, input ExecuteTaskActivityInput) (string, error) {
	systemPrompt := "You are a personal assistant and digital twin of a human. Your goal is to help your human in any way possible and help them to improve themselves. You are smart and wise and aim understand your human at a deep level. When you are asked to search the web, you should use the `perplexity_ask` tool if it exists. You must send a message after completing all tool calls. You must ensure that the final message includes the answer to the original task. You must never ask where to send the message, you MUST send it to the chat by default by specifying the chat_id. You are currently performing a periodic task, if there is previous execution result below make sure not repeat the same result when possible. "

	userProfile, err := s.userStorage.GetUserProfile(ctx)
	if err != nil {
		return "", err
	}

	if userProfile.Name != nil {
		systemPrompt += fmt.Sprintf("Name of your human: %s. ", *userProfile.Name)
	}
	if userProfile.Bio != nil {
		systemPrompt += fmt.Sprintf("Details about the user: %s. ", *userProfile.Bio)
	}
	systemPrompt += fmt.Sprintf("Current date and time: %s.", time.Now().Format(time.RFC3339))

	if input.PreviousResult != nil {
		systemPrompt += fmt.Sprintf("\n\nPrevious result: %s.", *input.PreviousResult)
	}
	systemPrompt += fmt.Sprintf("\n\nChat ID: ```%s```.", input.ChatID)

	fmt.Println("systemPrompt", systemPrompt)

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

	if input.Notify {
		notification := &model.AppNotification{
			ID:        uuid.New().String(),
			Title:     input.Name,
			Message:   response.Content,
			CreatedAt: time.Now().Format(time.RFC3339),
			Link:      helpers.Ptr("twin://chat/" + input.ChatID),
		}
		if len(response.ImageURLs) > 0 {
			notification.Image = &response.ImageURLs[0]
			s.logger.Debug("Sending notification with image", "image", notification.Image)
		}
		err = s.notifications.SendNotification(ctx, notification)
		if err != nil {
			return "", err
		}
	}

	return response.String(), nil
}

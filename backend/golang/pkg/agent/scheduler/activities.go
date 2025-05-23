package scheduler

import (
	"context"
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
	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/pkg/helpers"
	"github.com/EternisAI/enchanted-twin/pkg/prompts"
)

type userProfile interface {
	GetUserProfile(ctx context.Context) (*model.UserProfile, error)
	GetOAuthTokensArray(ctx context.Context, provider string) ([]db.OAuthTokens, error)
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
	systemPrompt, err := s.buildSystemPrompt(ctx, input.ChatID, input.PreviousResult)
	if err != nil {
		return "", err
	}

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

func (s *TaskSchedulerActivities) buildSystemPrompt(ctx context.Context, chatID string, previousResult *string) (string, error) {
	userProfile, err := s.userStorage.GetUserProfile(ctx)
	if err != nil {
		return "", err
	}

	oauthTokens, err := s.userStorage.GetOAuthTokensArray(ctx, "google")
	if err != nil {
		return "", err
	}
	var emailAccounts []string
	if len(oauthTokens) > 0 {
		for _, token := range oauthTokens {
			emailAccounts = append(emailAccounts, token.Username)
		}
	}

	systemPrompt, err := prompts.BuildScheduledTaskSystemPrompt(prompts.ScheduledTaskSystemPrompt{
		UserName:       userProfile.Name,
		Bio:            userProfile.Bio,
		ChatID:         &chatID,
		CurrentTime:    time.Now().Format(time.RFC3339),
		EmailAccounts:  emailAccounts,
		PreviousResult: previousResult,
	})
	if err != nil {
		return "", err
	}
	return systemPrompt, nil
}

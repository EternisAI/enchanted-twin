package twinchat

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/EternisAI/enchanted-twin/graph/model"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/twinchat/repository"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/openai/openai-go"
	"github.com/pkg/errors"
)

const (
	MODEL = "gpt-4o-mini"
)

type Service struct {
	aiService ai.Service
	storage   Storage
	nc        *nats.Conn
}

func NewService(aiService *ai.Service, storage Storage, nc *nats.Conn) *Service {
	return &Service{
		aiService: *aiService,
		storage:   storage,
		nc:        nc,
	}
}

func (s *Service) SendMessage(ctx context.Context, chatID string, message string) (*model.Message, error) {
	messages, err := s.storage.GetMessagesByChatId(ctx, chatID)
	if err != nil {
		return nil, err
	}

	messageHistory := make([]openai.ChatCompletionMessageParamUnion, 0)
	for _, message := range messages {
		openaiMessage, err := ToOpenAIMessage(*message)
		if err != nil {
			return nil, err
		}
		messageHistory = append(messageHistory, openaiMessage)
	}

	messageHistory = append(messageHistory, openai.UserMessage(message))

	completion, err := s.aiService.Completions(ctx, messageHistory, MODEL)
	if err != nil {
		return nil, err
	}

	subject := fmt.Sprintf("chat.%s", chatID)
	userMessageJson, err := json.Marshal(model.Message{
		ID:        uuid.New().String(),
		Text:      &completion.Content,
		CreatedAt: time.Now().Format(time.RFC3339),
		Role:      model.RoleUser,
	})
	if err != nil {
		return nil, err
	}
	err = s.nc.Publish(subject, userMessageJson)
	if err != nil {
		return nil, err
	}

	toolCalls := []string{}
	toolArgs := []string{}
	toolResults := []string{}

	// for _, toolCall := range completion.ToolCalls {
	// 	toolCalls = append(toolCalls, toolCall.Function.Name)
	// 	toolArgs = append(toolArgs, toolCall.Function.Arguments)

	// 	var toolArgs map[string]interface{}
	// 	err := json.Unmarshal([]byte(toolCall.Function.Arguments), &toolArgs)
	// 	if err != nil {
	// 		return nil, errors.Wrap(err, "failed to unmarshal tool call arguments")
	// 	}

	// 	// execute tool
	// 	toolResult := "X"
	// 	toolResults = append(toolResults, toolResult)
	// }

	toolCallsJson, err := json.Marshal(toolCalls)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal tool calls")
	}

	toolArgsJson, err := json.Marshal(toolArgs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal tool call arguments")
	}

	toolResultsJson, err := json.Marshal(toolResults)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal tool call results")
	}

	_, err = s.storage.AddMessageToChat(ctx, repository.Message{
		ID:        uuid.New().String(),
		ChatID:    chatID,
		Text:      message,
		Role:      model.RoleUser.String(),
		CreatedAt: time.Now(),
	})
	if err != nil {
		return nil, err
	}

	idAssistant, err := s.storage.AddMessageToChat(ctx, repository.Message{
		ID:          uuid.New().String(),
		ChatID:      chatID,
		Text:        completion.Content,
		ToolCalls:   repository.JSONForSQLLite(toolCallsJson),
		ToolArgs:    repository.JSONForSQLLite(toolArgsJson),
		ToolResults: repository.JSONForSQLLite(toolResultsJson),
		Role:        model.RoleUser.String(),
		CreatedAt:   time.Now(),
	})

	if err != nil {
		return nil, err
	}

	return &model.Message{
		ID:        idAssistant,
		Text:      &completion.Content,
		Role:      model.RoleUser,
		CreatedAt: time.Now().Format(time.RFC3339),
	}, nil
}

func (s *Service) GetChats(ctx context.Context) ([]*model.Chat, error) {
	return s.storage.GetChats(ctx)
}

func (s *Service) GetChat(ctx context.Context, chatID string) (model.Chat, error) {
	return s.storage.GetChat(ctx, chatID)
}

func (s *Service) CreateChat(ctx context.Context, name string) (model.Chat, error) {
	return s.storage.CreateChat(ctx, name)
}

func (s *Service) GetMessagesByChatId(ctx context.Context, chatID string) ([]*model.Message, error) {
	return s.storage.GetMessagesByChatId(ctx, chatID)
}

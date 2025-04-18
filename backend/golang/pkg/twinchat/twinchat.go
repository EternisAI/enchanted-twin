package twinchat

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/EternisAI/enchanted-twin/graph/model"
	"github.com/EternisAI/enchanted-twin/pkg/agent"
	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/helpers"
	"github.com/EternisAI/enchanted-twin/pkg/twinchat/repository"
	"github.com/pkg/errors"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/openai/openai-go"
)

type Service struct {
	aiService *ai.Service
	storage   Storage
	nc        *nats.Conn
}

func NewService(aiService *ai.Service, storage Storage, nc *nats.Conn) *Service {
	return &Service{
		aiService: aiService,
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

	agent := agent.NewAgent(s.nc, s.aiService)
	tools := []tools.Tool{
		&tools.SearchTool{},
		&tools.ImageTool{},
	}
	response, err := agent.Execute(ctx, messageHistory, tools)
	if err != nil {
		return nil, err
	}

	subject := fmt.Sprintf("chat.%s", chatID)
	assistantMessageJson, err := json.Marshal(model.Message{
		ID:        uuid.New().String(),
		Text:      &response.Content,
		ImageUrls: response.ImageURLs,
		CreatedAt: time.Now().Format(time.RFC3339),
		Role:      model.RoleAssistant,
	})
	if err != nil {
		return nil, err
	}
	err = s.nc.Publish(subject, assistantMessageJson)
	if err != nil {
		return nil, err
	}

	// user message
	_, err = s.storage.AddMessageToChat(ctx, repository.Message{
		ID:           uuid.New().String(),
		ChatID:       chatID,
		Text:         message,
		Role:         model.RoleUser.String(),
		CreatedAtStr: time.Now().Format(time.RFC3339),
	})
	if err != nil {
		return nil, err
	}

	// assistant message
	assistantMessageDb := repository.Message{
		ID:           uuid.New().String(),
		ChatID:       chatID,
		Text:         response.Content,
		Role:         model.RoleAssistant.String(),
		CreatedAtStr: time.Now().Format(time.RFC3339),
	}
	if len(response.ToolCalls) > 0 {
		toolCallsJson, err := json.Marshal(response.ToolCalls)
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal tool calls")
		}
		assistantMessageDb.ToolCallsStr = helpers.Ptr(string(toolCallsJson))
	}
	if len(response.ToolResults) > 0 {
		toolResultsJson, err := json.Marshal(response.ToolResults)
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal tool results")
		}
		assistantMessageDb.ToolResultsStr = helpers.Ptr(string(toolResultsJson))
	}
	if len(response.ImageURLs) > 0 {
		imageURLsJson, err := json.Marshal(response.ImageURLs)
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal image URLs")
		}
		assistantMessageDb.ImageURLsStr = helpers.Ptr(string(imageURLsJson))
	}

	idAssistant, err := s.storage.AddMessageToChat(ctx, assistantMessageDb)

	if err != nil {
		return nil, err
	}

	return &model.Message{
		ID:        idAssistant,
		Text:      &response.Content,
		Role:      model.RoleUser,
		ImageUrls: response.ImageURLs,
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

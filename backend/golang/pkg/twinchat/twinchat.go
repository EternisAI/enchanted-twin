package twinchat

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/log"

	"time"

	"github.com/EternisAI/enchanted-twin/graph/model"
	"github.com/EternisAI/enchanted-twin/pkg/agent"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
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
	logger    *log.Logger
	memory    memory.Storage
}

func NewService(logger *log.Logger, aiService *ai.Service, storage Storage, nc *nats.Conn, memory memory.Storage) *Service {
	return &Service{
		logger:    logger,
		aiService: aiService,
		storage:   storage,
		nc:        nc,
		memory:    memory,
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

	assistantMessageId := uuid.New().String()

	preToolCallback := func(toolCall openai.ChatCompletionMessageToolCall) {
		tcJson, err := json.Marshal(model.ToolCall{
			ID:          toolCall.ID,
			Name:        toolCall.Function.Name,
			MessageID:   assistantMessageId,
			IsCompleted: false,
		})
		if err != nil {
			s.logger.Error("failed to marshal tool call", "error", err)
			return
		}
		subject := fmt.Sprintf("chat.%s.tool_call", chatID)
		err = s.nc.Publish(subject, tcJson)
		if err != nil {
			s.logger.Error("failed to publish tool call", "error", err)
		}
	}

	toolCallResultsMap := make(map[string]model.ToolCallResult)
	postToolCallback := func(toolCall openai.ChatCompletionMessageToolCall, toolResult tools.ToolResult) {
		tcJson, err := json.Marshal(model.ToolCall{
			ID:        toolCall.ID,
			Name:      toolCall.Function.Name,
			MessageID: assistantMessageId,
			Result: &model.ToolCallResult{
				Content:   &toolResult.Content,
				ImageUrls: toolResult.ImageURLs,
			},
			IsCompleted: true,
		})
		toolCallResultsMap[toolCall.ID] = model.ToolCallResult{
			Content:   &toolResult.Content,
			ImageUrls: toolResult.ImageURLs,
		}
		if err != nil {
			s.logger.Error("failed to marshal tool call", "error", err)
			return
		}
		subject := fmt.Sprintf("chat.%s.tool_call", chatID)
		err = s.nc.Publish(subject, tcJson)
		if err != nil {
			s.logger.Error("failed to publish tool call", "error", err)
		}
	}

	agent := agent.NewAgent(s.logger, s.nc, s.aiService, preToolCallback, postToolCallback)
	tools := []tools.Tool{
		&tools.SearchTool{},
		&tools.ImageTool{},
		tools.NewMemorySearchTool(s.logger, s.memory),
	}

	response, err := agent.Execute(ctx, messageHistory, tools)
	if err != nil {
		return nil, err
	}

	subject := fmt.Sprintf("chat.%s", chatID)
	toolResults := make([]string, len(response.ToolResults))
	for i, v := range response.ToolResults {
		toolResultsJson, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		toolResults[i] = string(toolResultsJson)
	}

	toolCalls := make([]*model.ToolCall, len(response.ToolCalls))
	for i, v := range response.ToolCalls {
		toolCalls[i] = &model.ToolCall{
			ID:          v.ID,
			Name:        v.Function.Name,
			IsCompleted: true,
		}
	}

	assistantMessageJson, err := json.Marshal(model.Message{
		ID:          assistantMessageId,
		Text:        &response.Content,
		ImageUrls:   response.ImageURLs,
		CreatedAt:   time.Now().Format(time.RFC3339),
		Role:        model.RoleAssistant,
		ToolCalls:   toolCalls,
		ToolResults: toolResults,
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
		toolCalls := make([]model.ToolCall, 0)
		for _, toolCall := range response.ToolCalls {

			toolCall := model.ToolCall{
				ID:          toolCall.ID,
				Name:        toolCall.Function.Name,
				MessageID:   assistantMessageId,
				IsCompleted: true,
			}
			result, ok := toolCallResultsMap[toolCall.ID]
			if ok {
				toolCall.Result = &result
			}
			toolCalls = append(toolCalls, toolCall)
		}
		toolCallsJson, err := json.Marshal(toolCalls)
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

func (s *Service) DeleteChat(ctx context.Context, chatID string) error {
	return s.storage.DeleteChat(ctx, chatID)
}

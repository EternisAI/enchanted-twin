// Owner: august@eternis.ai
package twinchat

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
	"github.com/pkg/errors"

	"github.com/EternisAI/enchanted-twin/graph/model"
	"github.com/EternisAI/enchanted-twin/pkg/agent"
	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/pkg/helpers"
	"github.com/EternisAI/enchanted-twin/pkg/twinchat/repository"
)

type Service struct {
	aiService        *ai.Service
	storage          Storage
	nc               *nats.Conn
	logger           *log.Logger
	completionsModel string
	toolRegistry     *tools.ToolMapRegistry
	userStorage      *db.Store
}

func NewService(
	logger *log.Logger,
	aiService *ai.Service,
	storage Storage,
	nc *nats.Conn,
	registry *tools.ToolMapRegistry,
	userStorage *db.Store,
	completionsModel string,
) *Service {
	return &Service{
		logger:           logger,
		aiService:        aiService,
		storage:          storage,
		nc:               nc,
		completionsModel: completionsModel,
		toolRegistry:     registry,
		userStorage:      userStorage,
	}
}

func (s *Service) Execute(
	ctx context.Context,
	origin map[string]any,
	messageHistory []openai.ChatCompletionMessageParamUnion,
	preToolCallback func(toolCall openai.ChatCompletionMessageToolCall),
	postToolCallback func(toolCall openai.ChatCompletionMessageToolCall, toolResult types.ToolResult),
	onDelta func(agent.StreamDelta),
) (*agent.AgentResponse, error) {
	agent := agent.NewAgent(
		s.logger,
		s.nc,
		s.aiService,
		s.completionsModel,
		preToolCallback,
		postToolCallback,
	)

	// Get the tool list from the registry
	toolsList := s.toolRegistry.GetAll()

	// TODO(cosmic): pass origin to agent
	response, err := agent.ExecuteStream(ctx, messageHistory, toolsList, onDelta)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

func (s *Service) SendMessage(
	ctx context.Context,
	chatID string,
	message string,
) (*model.Message, error) {
	messages, err := s.storage.GetMessagesByChatId(ctx, chatID)
	if err != nil {
		return nil, err
	}

	systemPrompt := "You are a personal assistant and digital twin of a human. Your goal is to help your human in any way possible and help them to improve themselves. You are smart and wise and aim understand your human at a deep level. When you are asked to search the web, you should use the `perplexity_ask` tool if it exists. When user asks something to be done every minute, every hour, every day, every week, every month, every year, you should use the `schedule_task` tool and construct cron expression. "
	now := time.Now().Format(time.RFC3339)
	systemPrompt += fmt.Sprintf("\n\nCurrent system time: %s.\n", now)

	userProfile, err := s.userStorage.GetUserProfile(ctx)
	if err != nil {
		return nil, err
	}

	if userProfile.Name != nil {
		systemPrompt += fmt.Sprintf("User name: %s. ", *userProfile.Name)
	}
	if userProfile.Bio != nil {
		systemPrompt += fmt.Sprintf("Details about the user: %s. ", *userProfile.Bio)
	}
	systemPrompt += fmt.Sprintf("Current date and time: %s.", time.Now().Format(time.RFC3339))
	systemPrompt += fmt.Sprintf("Current Chat ID is %s.", chatID)

	messageHistory := make([]openai.ChatCompletionMessageParamUnion, 0)
	messageHistory = append(
		messageHistory,
		openai.SystemMessage(systemPrompt),
	)
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
	postToolCallback := func(toolCall openai.ChatCompletionMessageToolCall, toolResult types.ToolResult) {
		tcJson, err := json.Marshal(model.ToolCall{
			ID:        toolCall.ID,
			Name:      toolCall.Function.Name,
			MessageID: assistantMessageId,
			Result: &model.ToolCallResult{
				Content:   helpers.Ptr(toolResult.Content()),
				ImageUrls: toolResult.ImageURLs(),
			},
			IsCompleted: true,
		})
		toolCallResultsMap[toolCall.ID] = model.ToolCallResult{
			Content:   helpers.Ptr(toolResult.Content()),
			ImageUrls: toolResult.ImageURLs(),
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

	// user message - add to db and publish to NATS channel
	userMsgID := uuid.New().String()
	createdAt := time.Now().Format(time.RFC3339)

	onDelta := func(delta agent.StreamDelta) {
		payload := model.MessageStreamPayload{
			MessageID:  assistantMessageId,
			ImageUrls:  delta.ImageURLs,
			Chunk:      delta.ContentDelta,
			Role:       model.RoleAssistant,
			IsComplete: delta.IsCompleted,
			CreatedAt:  &createdAt,
		}
		_ = helpers.NatsPublish(s.nc, fmt.Sprintf("chat.%s.stream", chatID), payload)
	}

	origin := map[string]any{
		"chat_id":    chatID,
		"message_id": userMsgID,
	}

	response, err := s.Execute(ctx, origin, messageHistory, preToolCallback, postToolCallback, onDelta)
	if err != nil {
		return nil, err
	}
	s.logger.Debug(
		"Agent response",
		"content",
		response.Content,
		"tool_calls",
		len(response.ToolCalls),
		"tool_results",
		len(response.ToolResults),
	)

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

	// Create the message for DB
	userMsg := repository.Message{
		ID:           userMsgID,
		ChatID:       chatID,
		Text:         message,
		Role:         model.RoleUser.String(),
		CreatedAtStr: createdAt,
	}

	// Add to database
	_, err = s.storage.AddMessageToChat(ctx, userMsg)
	if err != nil {
		return nil, err
	}

	// Publish to NATS
	userNatsMsg := model.Message{
		ID:        userMsgID,
		Text:      &message,
		ImageUrls: []string{},
		CreatedAt: createdAt,
		Role:      model.RoleUser,
	}

	userNatsMsgJSON, err := json.Marshal(userNatsMsg)
	if err != nil {
		s.logger.Error("failed to marshal user NATS message", "error", err)
		// Continue anyway - db storage succeeded
	} else {
		// Publish on the chat channel
		subject := fmt.Sprintf("chat.%s", chatID)
		if err := s.nc.Publish(subject, userNatsMsgJSON); err != nil {
			s.logger.Error("failed to publish user message to NATS", "error", err)
			// Continue anyway - db storage succeeded
		}
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
			s.logger.Info("Tool call", "name", toolCall.Function.Name, "args", toolCall.Function.Arguments)

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
		ID:          idAssistant,
		Text:        &response.Content,
		Role:        model.RoleUser,
		ImageUrls:   response.ImageURLs,
		CreatedAt:   time.Now().Format(time.RFC3339),
		ToolCalls:   assistantMessageDb.ToModel().ToolCalls,
		ToolResults: assistantMessageDb.ToModel().ToolResults,
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

func (s *Service) GetMessagesByChatId(
	ctx context.Context,
	chatID string,
) ([]*model.Message, error) {
	return s.storage.GetMessagesByChatId(ctx, chatID)
}

func (s *Service) DeleteChat(ctx context.Context, chatID string) error {
	return s.storage.DeleteChat(ctx, chatID)
}

func (s *Service) GetChatSuggestions(
	ctx context.Context,
	chatID string,
) ([]*model.ChatSuggestionsCategory, error) {
	historicalMessages, err := s.storage.GetMessagesByChatId(ctx, chatID)
	if err != nil {
		return nil, err
	}

	var conversationContext string
	for _, message := range historicalMessages {
		conversationContext += fmt.Sprintf("%s: %s\n\n", message.Role, *message.Text)
	}

	instruction := fmt.Sprintf(
		"Generate 3 chat suggestions that user might ask for each of the category based on the chat history. Category names: Ask (should be questions about the content, should predict what user might wanna do next). Search (should be a plausible search based on the content). Research (should be a plausible research question based on the content).\n\n\nConversation history:\n%s",
		conversationContext,
	)

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage(instruction),
	}

	tool := openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name: "generate_suggestion",
			Description: param.NewOpt(
				"This tool generates chat suggestions for a user based on the existing context",
			),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"category": map[string]string{
						"type": "string",
					},
					"suggestions": map[string]any{
						"type": "array",
						"items": map[string]string{
							"type": "string",
						},
					},
				},
				"required": []string{"category", "suggestions"},
			},
		},
	}

	choice, err := s.aiService.Completions(
		ctx,
		messages,
		[]openai.ChatCompletionToolParam{tool},
		s.completionsModel,
	)
	if err != nil {
		return nil, err
	}

	suggestionsList := make([]*model.ChatSuggestionsCategory, 0)
	for _, choice := range choice.ToolCalls {
		var suggestions struct {
			Category    string   `json:"category"`
			Suggestions []string `json:"suggestions"`
		}
		err := json.Unmarshal([]byte(choice.Function.Arguments), &suggestions)
		if err != nil {
			return nil, err
		}
		suggestionsList = append(suggestionsList, &model.ChatSuggestionsCategory{
			Category:    suggestions.Category,
			Suggestions: suggestions.Suggestions,
		})
	}

	return suggestionsList, nil
}

func (s *Service) Tools() []tools.Tool {
	sendToChatTool := NewSendToChatTool(s.storage, s.nc)
	return []tools.Tool{sendToChatTool}
}

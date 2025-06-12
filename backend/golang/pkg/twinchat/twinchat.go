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
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/pkg/helpers"
	"github.com/EternisAI/enchanted-twin/pkg/identity"
	"github.com/EternisAI/enchanted-twin/pkg/prompts"
	"github.com/EternisAI/enchanted-twin/pkg/twinchat/repository"
)

type Storage interface {
	GetChat(ctx context.Context, id string) (model.Chat, error)
	GetChats(ctx context.Context) ([]*model.Chat, error)
	CreateChat(ctx context.Context, name string, category model.ChatCategory, holonThreadID *string) (model.Chat, error)
	DeleteChat(ctx context.Context, chatID string) error
	GetMessagesByChatId(ctx context.Context, chatId string) ([]*model.Message, error)
	AddMessageToChat(ctx context.Context, message repository.Message) (string, error)
}

type Service struct {
	aiService        *ai.Service
	storage          Storage
	nc               *nats.Conn
	memoryService    memory.Storage
	logger           *log.Logger
	completionsModel string
	reasoningModel   string
	toolRegistry     *tools.ToolMapRegistry
	userStorage      *db.Store
	identityService  *identity.IdentityService
}

func NewService(
	logger *log.Logger,
	aiService *ai.Service,
	storage Storage,
	nc *nats.Conn,
	memoryService memory.Storage,
	registry *tools.ToolMapRegistry,
	userStorage *db.Store,
	completionsModel string,
	reasoningModel string,
	identityService *identity.IdentityService,
) *Service {
	return &Service{
		logger:           logger,
		aiService:        aiService,
		storage:          storage,
		nc:               nc,
		memoryService:    memoryService,
		completionsModel: completionsModel,
		reasoningModel:   reasoningModel,
		toolRegistry:     registry,
		userStorage:      userStorage,
		identityService:  identityService,
	}
}

func (s *Service) Execute(
	ctx context.Context,
	origin map[string]any,
	messageHistory []openai.ChatCompletionMessageParamUnion,
	preToolCallback func(toolCall openai.ChatCompletionMessageToolCall),
	postToolCallback func(toolCall openai.ChatCompletionMessageToolCall, toolResult types.ToolResult),
	onDelta func(agent.StreamDelta),
	reasoning bool,
) (*agent.AgentResponse, error) {
	agent := agent.NewAgent(
		s.logger,
		s.nc,
		s.aiService,
		s.completionsModel,
		s.reasoningModel,
		preToolCallback,
		postToolCallback,
	)

	// Get the tool list from the registry
	toolsList := s.toolRegistry.Excluding("send_to_chat").GetAll()

	// TODO(cosmic): pass origin to agent
	response, err := agent.ExecuteStream(ctx, messageHistory, toolsList, onDelta, reasoning)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

func (s *Service) buildSystemPrompt(ctx context.Context, chatID string, isVoice bool, userMemoryProfile string) (string, error) {
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

	chat, err := s.storage.GetChat(ctx, chatID)
	if err != nil {
		return "", err
	}
	holonThreadId := chat.HolonThreadID

	systemPrompt, err := prompts.BuildTwinChatSystemPrompt(prompts.TwinChatSystemPrompt{
		UserName:          userProfile.Name,
		Bio:               userProfile.Bio,
		EmailAccounts:     emailAccounts,
		ChatID:            &chatID,
		CurrentTime:       time.Now().Format(time.RFC3339),
		IsVoice:           isVoice,
		UserMemoryProfile: userMemoryProfile,
		HolonThreadID:     holonThreadId,
	})
	if err != nil {
		return "", err
	}
	return systemPrompt, nil
}

func (s *Service) SendMessage(
	ctx context.Context,
	chatID string,
	message string,
	isReasoning bool,
	isVoice bool,
) (*model.Message, error) {
	now := time.Now()

	messages := make([]*model.Message, 0)
	if chatID == "" {
		// Determine category based on isVoice parameter
		category := model.ChatCategoryText
		if isVoice {
			category = model.ChatCategoryVoice
		}
		chat, err := s.storage.CreateChat(ctx, "New chat", category, nil)
		if err != nil {
			return nil, err
		}
		s.logger.Info("Created new chat", "chat_id", chat.ID)
		chatID = chat.ID
	} else {
		messages_, err := s.storage.GetMessagesByChatId(ctx, chatID)
		if err != nil {
			return nil, err
		}
		messages = messages_
	}

	userMemoryProfile, err := s.identityService.GetUserProfile(ctx)
	if err != nil {
		s.logger.Error("failed to get user memory profile", "error", err)
		userMemoryProfile = ""
	}

	systemPrompt, err := s.buildSystemPrompt(ctx, chatID, isVoice, userMemoryProfile)
	if err != nil {
		return nil, err
	}

	s.logger.Info("System prompt", "prompt", systemPrompt, "isVoice", isVoice, "isReasoning", isReasoning)
	// if userProfile.Name != nil {
	// 	systemPrompt += fmt.Sprintf("Name of your human: %s. ", *userProfile.Name)
	// }
	// if userProfile.Bio != nil {
	// 	systemPrompt += fmt.Sprintf("Details about the user: %s. ", *userProfile.Bio)
	// }

	oauthTokens, err := s.userStorage.GetOAuthTokensArray(ctx, "google")
	if err != nil {
		return nil, err
	}
	if len(oauthTokens) > 0 {
		systemPrompt += "You have following email accounts connected to your account: "
		for _, token := range oauthTokens {
			systemPrompt += fmt.Sprintf("%s, ", token.Username)
		}
	} else {
		systemPrompt += "You have no email accounts connected to your account."
	}

	oauthTokens, err = s.userStorage.GetOAuthTokensArray(ctx, "twitter")
	if err != nil {
		return nil, err
	}
	if len(oauthTokens) > 0 {
		systemPrompt += "When a request references the user's *feed* or *timeline*, the assistant " +
			"MUST first call `list_feed_tweets`, paginate as needed, and may then " +
			"client-side-filter the results. It MUST NOT call `search_tweets` in this " +
			"scenario."
	}

	systemPrompt += fmt.Sprintf("Current date and time: %s.", time.Now().Format(time.RFC3339))
	systemPrompt += fmt.Sprintf("Current Chat ID is %s.", chatID)

	s.logger.Info("System prompt", "prompt", systemPrompt)

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

	s.logger.Info("Executing agent", "reasoning", isReasoning)
	response, err := s.Execute(ctx, origin, messageHistory, preToolCallback, postToolCallback, onDelta, isReasoning)
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
		CreatedAtStr: now.Format(time.RFC3339Nano),
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
		CreatedAtStr: time.Now().Format(time.RFC3339Nano),
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

	// Index the conversation asynchronously
	go func() {
		err := s.IndexConversation(context.Background(), chatID)
		if err != nil {
			s.logger.Error("failed to index conversation", "chat_id", chatID, "error", err)
		}
	}()

	return &model.Message{
		ID:          idAssistant,
		Text:        &response.Content,
		Role:        model.RoleAssistant,
		ImageUrls:   response.ImageURLs,
		CreatedAt:   time.Now().Format(time.RFC3339),
		ToolCalls:   assistantMessageDb.ToModel().ToolCalls,
		ToolResults: assistantMessageDb.ToModel().ToolResults,
	}, nil
}

// ProcessMessageHistory processes a list of messages and saves only the new ones (diff) to the database.
func (s *Service) ProcessMessageHistory(
	ctx context.Context,
	chatID string,
	messages []*model.MessageInput,
	isReasoning bool,
	isVoice bool,
) (*model.Message, error) {
	now := time.Now()

	// Create chat if it doesn't exist
	if chatID == "" {
		category := model.ChatCategoryText
		if isVoice {
			category = model.ChatCategoryVoice
		}
		chat, err := s.storage.CreateChat(ctx, "New chat", category, nil)
		if err != nil {
			return nil, err
		}
		s.logger.Info("Created new chat for message history", "chat_id", chat.ID)
		chatID = chat.ID
	}

	// Fetch existing messages for the chat
	existingMessages, err := s.storage.GetMessagesByChatId(ctx, chatID)
	if err != nil {
		return nil, err
	}

	// Find the diff: only save messages that are not already present (by role and text, in order)
	diffStart := 0
	for i := 0; i < len(existingMessages) && i < len(messages); i++ {
		if existingMessages[i].Role != messages[i].Role ||
			existingMessages[i].Text == nil || *existingMessages[i].Text != messages[i].Text {
			break
		}
		diffStart = i + 1
	}
	newMessages := messages[diffStart:]

	userMemoryProfile, err := s.identityService.GetUserProfile(ctx)
	if err != nil {
		s.logger.Error("failed to get user memory profile", "error", err)
		userMemoryProfile = ""
	}

	systemPrompt, err := s.buildSystemPrompt(ctx, chatID, isVoice, userMemoryProfile)
	if err != nil {
		return nil, err
	}

	s.logger.Info("System prompt for history processing", "prompt", systemPrompt, "isVoice", isVoice, "isReasoning", isReasoning)

	messageHistory := make([]openai.ChatCompletionMessageParamUnion, 0)
	messageHistory = append(
		messageHistory,
		openai.SystemMessage(systemPrompt),
	)

	// Add all existing messages to the message history for the AI
	for _, msg := range existingMessages {
		if msg.Text == nil {
			continue
		}
		switch msg.Role {
		case model.RoleUser:
			messageHistory = append(messageHistory, openai.UserMessage(*msg.Text))
		case model.RoleAssistant:
			messageHistory = append(messageHistory, openai.AssistantMessage(*msg.Text))
		}
	}

	// Save only the new messages to the database and add to message history
	for _, msg := range newMessages {
		msgID := uuid.New().String()
		createdAt := now.Format(time.RFC3339Nano)
		dbMessage := repository.Message{
			ID:           msgID,
			ChatID:       chatID,
			Text:         msg.Text,
			Role:         msg.Role.String(),
			CreatedAtStr: createdAt,
		}
		_, err = s.storage.AddMessageToChat(ctx, dbMessage)
		if err != nil {
			return nil, err
		}
		natsMsg := model.Message{
			ID:        msgID,
			Text:      &msg.Text,
			ImageUrls: []string{},
			CreatedAt: time.Now().Format(time.RFC3339),
			Role:      msg.Role,
		}
		natsMsgJSON, err := json.Marshal(natsMsg)
		if err != nil {
			s.logger.Error("failed to marshal message for NATS", "error", err)
		} else {
			subject := fmt.Sprintf("chat.%s", chatID)
			if err := s.nc.Publish(subject, natsMsgJSON); err != nil {
				s.logger.Error("failed to publish message to NATS", "error", err)
			}
		}
		switch msg.Role {
		case model.RoleUser:
			messageHistory = append(messageHistory, openai.UserMessage(msg.Text))
		case model.RoleAssistant:
			messageHistory = append(messageHistory, openai.AssistantMessage(msg.Text))
		}
	}

	if len(messages) == 0 {
		return nil, fmt.Errorf("no messages provided in history")
	}

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
		"message_id": uuid.New().String(),
	}

	s.logger.Info("Executing agent with message history", "reasoning", isReasoning)
	response, err := s.Execute(ctx, origin, messageHistory, preToolCallback, postToolCallback, onDelta, isReasoning)
	if err != nil {
		return nil, err
	}
	s.logger.Debug(
		"Agent response for message history",
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

	assistantMessageDb := repository.Message{
		ID:           assistantMessageId,
		ChatID:       chatID,
		Text:         response.Content,
		Role:         model.RoleAssistant.String(),
		CreatedAtStr: time.Now().Format(time.RFC3339Nano),
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

	assistantMessage := &model.Message{
		ID:          idAssistant,
		Text:        &response.Content,
		ImageUrls:   response.ImageURLs,
		CreatedAt:   now.Format(time.RFC3339),
		Role:        model.RoleAssistant,
		ToolCalls:   assistantMessageDb.ToModel().ToolCalls,
		ToolResults: assistantMessageDb.ToModel().ToolResults,
	}

	assistantMessageJson, err := json.Marshal(assistantMessage)
	if err != nil {
		return nil, err
	}
	err = s.nc.Publish(subject, assistantMessageJson)
	if err != nil {
		return nil, err
	}

	go func() {
		err := s.IndexConversation(context.Background(), chatID)
		if err != nil {
			s.logger.Error("failed to index conversation", "chat_id", chatID, "error", err)
		}
	}()

	return assistantMessage, nil
}

func (s *Service) GetChats(ctx context.Context) ([]*model.Chat, error) {
	return s.storage.GetChats(ctx)
}

func (s *Service) GetChat(ctx context.Context, chatID string) (model.Chat, error) {
	return s.storage.GetChat(ctx, chatID)
}

func (s *Service) CreateChat(ctx context.Context, name string, category model.ChatCategory, holonThreadID *string) (model.Chat, error) {
	return s.storage.CreateChat(ctx, name, category, holonThreadID)
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

func (s *Service) IndexConversation(ctx context.Context, chatID string) error {
	messages, err := s.storage.GetMessagesByChatId(ctx, chatID)
	if err != nil {
		return err
	}

	slidingWindow := 10
	messagesWindow := helpers.SafeLastN(messages, slidingWindow)

	var conversationMessages []memory.ConversationMessage
	var people []string
	peopleMap := make(map[string]bool)
	primaryUser := "primaryUser"

	for _, message := range messagesWindow {
		if message.Role.String() == "system" {
			continue
		}

		var speaker string
		if message.Role == model.RoleUser {
			speaker = primaryUser
		} else {
			speaker = "assistant"
		}

		if !peopleMap[speaker] {
			people = append(people, speaker)
			peopleMap[speaker] = true
		}

		createdAt, err := time.Parse(time.RFC3339, message.CreatedAt)
		if err != nil {
			createdAt = time.Now()
		}

		if message.Text != nil {
			conversationMessages = append(conversationMessages, memory.ConversationMessage{
				Speaker: speaker,
				Content: *message.Text,
				Time:    createdAt,
			})
		}
	}

	if len(conversationMessages) == 0 {
		s.logger.Info("No messages to index", "chat_id", chatID)
		return nil
	}

	doc := memory.ConversationDocument{
		FieldID:      uuid.New().String(),
		FieldSource:  "chat",
		People:       people,
		User:         primaryUser,
		Conversation: conversationMessages,
		FieldMetadata: map[string]string{
			"chat_id": chatID,
		},
	}

	s.logger.Info("Indexing conversation", "chat_id", chatID, "messages_count", len(conversationMessages))

	return s.memoryService.Store(ctx, []memory.Document{&doc}, nil)
}

func (s *Service) SendAssistantMessage(
	ctx context.Context,
	chatID string,
	message string,
) (*model.Message, error) {
	now := time.Now()

	if chatID == "" {
		chat, err := s.storage.CreateChat(ctx, "New chat", model.ChatCategoryText, nil)
		if err != nil {
			return nil, err
		}
		s.logger.Info("Created new chat", "chat_id", chat.ID)
		chatID = chat.ID
	}

	assistantMessageId := uuid.New().String()
	createdAt := time.Now().Format(time.RFC3339)

	assistantMessageDb := repository.Message{
		ID:           assistantMessageId,
		ChatID:       chatID,
		Text:         message,
		Role:         model.RoleAssistant.String(),
		CreatedAtStr: now.Format(time.RFC3339Nano),
	}

	idAssistant, err := s.storage.AddMessageToChat(ctx, assistantMessageDb)
	if err != nil {
		return nil, err
	}

	assistantNatsMsg := model.Message{
		ID:        assistantMessageId,
		Text:      &message,
		ImageUrls: []string{},
		CreatedAt: createdAt,
		Role:      model.RoleAssistant,
	}

	assistantNatsMsgJSON, err := json.Marshal(assistantNatsMsg)
	if err != nil {
		s.logger.Error("failed to marshal assistant NATS message", "error", err)
	} else {
		subject := fmt.Sprintf("chat.%s", chatID)
		if err := s.nc.Publish(subject, assistantNatsMsgJSON); err != nil {
			s.logger.Error("failed to publish assistant message to NATS", "error", err)
		}
	}

	go func() {
		err := s.IndexConversation(context.Background(), chatID)
		if err != nil {
			s.logger.Error("failed to index conversation", "chat_id", chatID, "error", err)
		}
	}()

	return &model.Message{
		ID:        idAssistant,
		Text:      &message,
		Role:      model.RoleAssistant,
		ImageUrls: []string{},
		CreatedAt: createdAt,
	}, nil
}

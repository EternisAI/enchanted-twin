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
	UpdateChatPrivacyDict(ctx context.Context, chatID string, privacyDictJson *string) error
	DeleteChat(ctx context.Context, chatID string) error
	GetMessagesByChatId(ctx context.Context, chatId string) ([]*model.Message, error)
	AddMessageToChat(ctx context.Context, message repository.Message) (string, error)
	ReplaceMessagesByChatId(ctx context.Context, chatID string, messages []repository.Message) error
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
	anonymizerType   string
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
	anonymizerType string,
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
		anonymizerType:   anonymizerType,
	}
}

func (s *Service) Execute(
	ctx context.Context,
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

	var toolsList []tools.Tool
	if !reasoning {
		toolsList = s.toolRegistry.Excluding("send_to_chat").GetAll()
	}

	response, err := agent.ExecuteStreamWithPrivacy(ctx, ai.EmptyConversationID, messageHistory, toolsList, onDelta, reasoning)
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

	canSearchWeb := s.checkWebSearchCapability()
	availableTools := s.getAvailableTools()

	systemPrompt, err := prompts.BuildTwinChatSystemPrompt(prompts.TwinChatSystemPrompt{
		UserName:          userProfile.Name,
		Bio:               userProfile.Bio,
		EmailAccounts:     emailAccounts,
		ChatID:            &chatID,
		CurrentTime:       time.Now().Format(time.RFC3339),
		IsVoice:           isVoice,
		UserMemoryProfile: userMemoryProfile,
		HolonThreadID:     holonThreadId,
		CanSearchWeb:      canSearchWeb,
		AvailableTools:    availableTools,
	})
	if err != nil {
		return "", err
	}
	return systemPrompt, nil
}

func (s *Service) checkWebSearchCapability() bool {
	webSearchTools := []string{"perplexity_ask", "web_search", "search_web", "google_search", "search_internet"}

	for _, toolName := range webSearchTools {
		if _, exists := s.toolRegistry.Get(toolName); exists {
			return true
		}
	}

	return false
}

func (s *Service) getAvailableTools() []prompts.ToolInfo {
	if s.toolRegistry == nil {
		return []prompts.ToolInfo{}
	}

	allTools := s.toolRegistry.GetAll()
	toolInfos := make([]prompts.ToolInfo, 0, len(allTools))

	for _, tool := range allTools {
		def := tool.Definition()
		if def.Type == "function" {
			toolInfos = append(toolInfos, prompts.ToolInfo{
				Name:        def.Function.Name,
				Description: def.Function.Description.Value,
			})
		}
	}

	return toolInfos
}

func (s *Service) SendMessage(
	ctx context.Context,
	chatID string,
	message string,
	isReasoning bool,
	isVoice bool,
) (*model.Message, error) {
	startTime := time.Now()
	s.logger.Info("SendMessage started", "chatID", chatID, "messageLength", len(message), "isReasoning", isReasoning, "isVoice", isVoice)

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

	chatSetupTime := time.Since(startTime)
	s.logger.Info("Chat setup completed", "duration", chatSetupTime, "chatID", chatID, "messageCount", len(messages))

	userProfileStart := time.Now()
	userMemoryProfile, err := s.identityService.GetUserProfile(ctx)
	if err != nil {
		s.logger.Error("failed to get user memory profile", "error", err)
		userMemoryProfile = ""
	}
	userProfileTime := time.Since(userProfileStart)
	s.logger.Info("User profile loaded", "duration", userProfileTime, "profileLength", len(userMemoryProfile))

	systemPromptStart := time.Now()
	systemPrompt, err := s.buildSystemPrompt(ctx, chatID, isVoice, userMemoryProfile)
	if err != nil {
		return nil, err
	}
	systemPromptTime := time.Since(systemPromptStart)
	s.logger.Info("System prompt built", "duration", systemPromptTime, "promptLength", len(systemPrompt))
	s.logger.Debug("System prompt", "prompt", systemPrompt, "isVoice", isVoice, "isReasoning", isReasoning)

	messageHistoryStart := time.Now()
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
	messageHistoryTime := time.Since(messageHistoryStart)
	s.logger.Info("Message history prepared", "duration", messageHistoryTime, "historyLength", len(messageHistory))

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
	toolCallErrorsMap := make(map[string]string)
	postToolCallback := func(toolCall openai.ChatCompletionMessageToolCall, toolResult types.ToolResult) {
		var errorField *string
		if toolResult.Error() != "" {
			errorField = helpers.Ptr(toolResult.Error())
			toolCallErrorsMap[toolCall.ID] = toolResult.Error()
		}

		tcJson, err := json.Marshal(model.ToolCall{
			ID:        toolCall.ID,
			Name:      toolCall.Function.Name,
			MessageID: assistantMessageId,
			Result: &model.ToolCallResult{
				Content:   helpers.Ptr(toolResult.Content()),
				ImageUrls: toolResult.ImageURLs(),
			},
			IsCompleted: true,
			Error:       errorField,
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
			MessageID:                      assistantMessageId,
			ImageUrls:                      delta.ImageURLs,
			Chunk:                          delta.ContentDelta,
			Role:                           model.RoleAssistant,
			IsComplete:                     delta.IsCompleted,
			CreatedAt:                      &createdAt,
			AccumulatedMessage:             delta.AccumulatedAnonymizedMessage,
			DeanonymizedAccumulatedMessage: delta.AccumulatedDeanonymizedMessage,
		}
		_ = helpers.NatsPublish(s.nc, fmt.Sprintf("chat.%s.stream", chatID), payload)
	}

	agentSetupStart := time.Now()
	s.logger.Info("Executing agent", "reasoning", isReasoning)

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
		s.logger.Error(" Failed to store user message in database", "messageID", userMsgID, "chatID", chatID, "error", err)
		return nil, err
	}

	userNatsMsg := model.Message{
		ID:        userMsgID,
		Text:      &message,
		ImageUrls: []string{},
		CreatedAt: createdAt,
		Role:      model.RoleUser,
	}
	_ = helpers.NatsPublish(s.nc, fmt.Sprintf("chat.%s", chatID), userNatsMsg)

	agent := agent.NewAgent(
		s.logger,
		s.nc,
		s.aiService,
		s.completionsModel,
		s.reasoningModel,
		preToolCallback,
		postToolCallback,
	)

	toolsList := s.toolRegistry.Excluding("send_to_chat").GetAll()
	// toolsList := []tools.Tool{}
	agentSetupTime := time.Since(agentSetupStart)
	s.logger.Info("Agent setup completed", "duration", agentSetupTime, "toolsCount", len(toolsList))

	agentExecutionStart := time.Now()
	response, err := agent.ExecuteStreamWithPrivacy(ctx, chatID, messageHistory, toolsList, onDelta, isReasoning)
	agentExecutionTime := time.Since(agentExecutionStart)
	if err != nil {
		s.logger.Error("Agent execution failed", "duration", agentExecutionTime, "error", err)
		// send message to stop progress indicator
		payload := model.MessageStreamPayload{
			MessageID:                      assistantMessageId,
			Chunk:                          "",
			Role:                           model.RoleAssistant,
			IsComplete:                     true,
			CreatedAt:                      &createdAt,
			AccumulatedMessage:             "",
			DeanonymizedAccumulatedMessage: "",
		}
		_ = helpers.NatsPublish(s.nc, fmt.Sprintf("chat.%s.stream", chatID), payload)
		return nil, err
	}
	s.logger.Info("Agent execution completed", "duration", agentExecutionTime, "contentLength", len(response.Content), "toolCallsCount", len(response.ToolCalls), "toolResultsCount", len(response.ToolResults), "imageURLsCount", len(response.ImageURLs), "replacementRulesCount", len(response.ReplacementRules))

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
		toolCall := &model.ToolCall{
			ID:          v.ID,
			Name:        v.Function.Name,
			IsCompleted: true,
		}

		// Add error information if available
		if errorMsg, hasError := toolCallErrorsMap[v.ID]; hasError {
			toolCall.Error = helpers.Ptr(errorMsg)
		}

		toolCalls[i] = toolCall
	}

	messageContent := response.Content
	if messageContent == "" && len(response.ToolResults) > 0 {
		messageContent = "Task completed successfully."
	}

	// Handle privacy dictionary update based on anonymizer type
	privacyStart := time.Now()
	var privacyDictJson *string

	if s.anonymizerType == "mock" {
		// Use mock anonymizer for development/testing
		privacyDictJson, err = MockAnonymizer(ctx, chatID, message)
		if err != nil {
			s.logger.Error("failed to generate mock privacy dictionary", "error", err)
		}
	} else if len(response.ReplacementRules) > 0 {
		// Use real anonymization rules from the agent response
		// response.ReplacementRules are in format {token: original}, already correct format
		privacyDictJson, err = createPrivacyDictFromReplacementRules(chatID, response.ReplacementRules)
		if err != nil {
			s.logger.Error("failed to generate privacy dictionary from replacement rules", "error", err)
		}
	}

	// Update privacy dictionary if we have one
	if privacyDictJson != nil {
		err = s.storage.UpdateChatPrivacyDict(ctx, chatID, privacyDictJson)
		if err != nil {
			s.logger.Error("failed to update chat privacy dictionary", "error", err)
		} else {
			s.logger.Info("updated chat privacy dictionary", "chat_id", chatID, "type", s.anonymizerType)

			go func() {
				privacyUpdate := model.PrivacyDictUpdate{
					ChatID:          chatID,
					PrivacyDictJSON: *privacyDictJson,
				}

				subjectPrivacyDict := fmt.Sprintf("chat.%s.privacy_dict", chatID)
				if err := helpers.NatsPublish(s.nc, subjectPrivacyDict, privacyUpdate); err != nil {
					s.logger.Error("failed to publish privacy dictionary update", "error", err, "subject", subjectPrivacyDict, "chatID", chatID)
				} else {
					s.logger.Info("published privacy dictionary update", "subject", subjectPrivacyDict, "chatID", chatID)
				}
			}()
		}
	}
	privacyTime := time.Since(privacyStart)
	s.logger.Info("Privacy dictionary processing completed", "duration", privacyTime, "hasPrivacyDict", privacyDictJson != nil)

	// assistant message
	assistantMessageDb := repository.Message{
		ID:           assistantMessageId, // Use the same ID as in streaming/publish
		ChatID:       chatID,
		Text:         messageContent,
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

			// Add error information if available
			if errorMsg, hasError := toolCallErrorsMap[toolCall.ID]; hasError {
				toolCall.Error = helpers.Ptr(errorMsg)
			}

			toolCalls = append(toolCalls, toolCall)
		}

		toolCallsJson, err := json.Marshal(toolCalls)
		if err != nil {
			s.logger.Error("❌ Failed to marshal tool calls", "error", err, "toolCallsCount", len(toolCalls))
			return nil, errors.Wrap(err, "failed to marshal tool calls")
		}
		s.logger.Debug("📝 Tool calls JSON for database", "json", string(toolCallsJson))
		assistantMessageDb.ToolCallsStr = helpers.Ptr(string(toolCallsJson))
	}
	if len(response.ToolResults) > 0 {
		s.logger.Info("📝 Marshaling tool results for database storage",
			"toolResultsCount", len(response.ToolResults),
			"toolResultsWithErrors", func() int {
				count := 0
				for _, tr := range response.ToolResults {
					if tr.Error() != "" {
						count++
					}
				}
				return count
			}())

		toolResultsJson, err := json.Marshal(response.ToolResults)
		if err != nil {
			s.logger.Error("❌ Failed to marshal tool results", "error", err, "toolResultsCount", len(response.ToolResults))
			return nil, errors.Wrap(err, "failed to marshal tool results")
		}
		s.logger.Debug("📝 Tool results JSON for database", "json", string(toolResultsJson))
		assistantMessageDb.ToolResultsStr = helpers.Ptr(string(toolResultsJson))
	}
	if len(response.ImageURLs) > 0 {
		s.logger.Info("📝 Marshaling image URLs for database storage", "imageURLsCount", len(response.ImageURLs))
		imageURLsJson, err := json.Marshal(response.ImageURLs)
		if err != nil {
			s.logger.Error("❌ Failed to marshal image URLs", "error", err, "imageURLsCount", len(response.ImageURLs))
			return nil, errors.Wrap(err, "failed to marshal image URLs")
		}
		s.logger.Debug("📝 Image URLs JSON for database", "json", string(imageURLsJson))
		assistantMessageDb.ImageURLsStr = helpers.Ptr(string(imageURLsJson))
	}

	idAssistant, err := s.storage.AddMessageToChat(ctx, assistantMessageDb)
	if err != nil {
		s.logger.Error("❌ Failed to store assistant message in database",
			"messageID", assistantMessageId,
			"chatID", chatID,
			"error", err)
		return nil, err
	}

	assistantMessageJson, err := json.Marshal(model.Message{
		ID:          idAssistant,
		Text:        &messageContent,
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

	// Index the conversation asynchronously
	go func() {
		indexStart := time.Now()
		err := s.IndexConversation(context.Background(), chatID)
		indexTime := time.Since(indexStart)
		if err != nil {
			s.logger.Error("failed to index conversation", "chat_id", chatID, "error", err, "duration", indexTime)
		} else {
			s.logger.Info("conversation indexed", "chat_id", chatID, "duration", indexTime)
		}
	}()

	totalTime := time.Since(startTime)
	s.logger.Info("SendMessage completed", "totalDuration", totalTime, "chatID", chatID, "messageID", idAssistant)

	return &model.Message{
		ID:          idAssistant,
		Text:        &messageContent,
		Role:        model.RoleAssistant,
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

func (s *Service) CreateChat(ctx context.Context, name string, category model.ChatCategory, holonThreadID *string) (model.Chat, error) {
	return s.storage.CreateChat(ctx, name, category, holonThreadID)
}

func (s *Service) UpdateChatPrivacyDict(ctx context.Context, chatID string, privacyDictJson *string) error {
	return s.storage.UpdateChatPrivacyDict(ctx, chatID, privacyDictJson)
}

func (s *Service) DeleteChat(ctx context.Context, chatID string) error {
	return s.storage.DeleteChat(ctx, chatID)
}

func (s *Service) GetMessagesByChatId(
	ctx context.Context,
	chatID string,
) ([]*model.Message, error) {
	return s.storage.GetMessagesByChatId(ctx, chatID)
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
		ai.Background,
	)
	if err != nil {
		return nil, err
	}

	suggestionsList := make([]*model.ChatSuggestionsCategory, 0)
	for _, choice := range choice.Message.ToolCalls {
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
		FieldID:      chatID,
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

// createPrivacyDictFromReplacementRules converts anonymization replacement rules to privacy dictionary format.
func createPrivacyDictFromReplacementRules(chatID string, replacementRules map[string]string) (*string, error) {
	// Create the privacy dictionary from replacement rules
	// The replacement rules are in format: {token: original_text} e.g. {"PERSON_001": "John Smith"}
	privacyDict := make(map[string]interface{})

	// Add the replacement rules to the dictionary
	// replacementRules format: {token: original_text} e.g. {"Emma": "Alice", "maksim": "innokentii"}
	// privacyDict format: {original: token} e.g. {"Alice": "Emma", "innokentii": "maksim"} for frontend display
	for token, originalText := range replacementRules {
		privacyDict[originalText] = token
	}

	// Add metadata similar to MockAnonymizer
	privacyDict["_metadata"] = map[string]interface{}{
		"chat_id":      chatID,
		"last_updated": time.Now().Format(time.RFC3339),
		"total_rules":  len(replacementRules), // Count of actual replacement rules
		"version":      "v1",
		"type":         "real_anonymization",
	}

	jsonData, err := json.Marshal(privacyDict)
	if err != nil {
		return nil, err
	}

	jsonString := string(jsonData)
	return &jsonString, nil
}

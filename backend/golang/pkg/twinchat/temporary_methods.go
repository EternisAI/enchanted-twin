package twinchat

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/openai/openai-go"

	"github.com/EternisAI/enchanted-twin/graph/model"
	"github.com/EternisAI/enchanted-twin/pkg/agent"
	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
	"github.com/EternisAI/enchanted-twin/pkg/helpers"
	"github.com/EternisAI/enchanted-twin/pkg/twinchat/repository"
)

// ProcessMessageHistoryStream processes a list of messages and returns a channel for streaming the response.
// NOTE: Reconsider.
func (s *Service) ProcessMessageHistoryStream(
	ctx context.Context,
	chatID string,
	messages []*model.MessageInput,
	isOnboarding bool,
) (<-chan model.MessageStreamPayload, error) {
	now := time.Now()

	// Create chat if it doesn't exist
	if chatID == "" {
		category := model.ChatCategoryText
		chat, err := s.storage.CreateChat(ctx, "New chat", category, nil)
		if err != nil {
			return nil, err
		}
		s.logger.Info("Created new chat for message history stream", "chat_id", chat.ID)
		chatID = chat.ID
	}

	// Prepare messages for database storage
	dbMessages := make([]repository.Message, 0, len(messages))
	for i, msg := range messages {
		msgID := uuid.New().String()
		// Ensure each message has a unique timestamp by adding microseconds based on index
		createdAt := now.Add(time.Microsecond * time.Duration(i)).Format(time.RFC3339Nano)
		dbMessage := repository.Message{
			ID:           msgID,
			ChatID:       chatID,
			Text:         msg.Text,
			Role:         msg.Role.String(),
			CreatedAtStr: createdAt,
		}
		dbMessages = append(dbMessages, dbMessage)
	}

	// Efficiently replace all messages in a single transaction
	err := s.storage.ReplaceMessagesByChatId(ctx, chatID, dbMessages)
	if err != nil {
		return nil, fmt.Errorf("failed to replace messages: %w", err)
	}

	userMemoryProfile, err := s.identityService.GetUserProfile(ctx)
	if err != nil {
		s.logger.Error("failed to get user memory profile", "error", err)
		userMemoryProfile = ""
	}

	var systemPrompt string
	if isOnboarding {
		systemPrompt = `You are an onboarding agent. Your job is to welcome new users and gather some basic information about them.

You need to ask exactly 3 questions in a friendly and conversational manner:
1. What is your name?
2. What is your favorite color?
3. What is your favorite animal?

After the user has answered all three questions, you should call the finalize_onboarding tool with:
- name: the user's name (required)
- context: a summary of their other answers (e.g., "favorite color: blue, favorite animal: cat")

Be warm, welcoming, and conversational. Ask one question at a time and wait for the user's response before moving to the next question.`
	} else {
		systemPrompt, err = s.buildSystemPrompt(ctx, chatID, true, userMemoryProfile)
		if err != nil {
			return nil, err
		}
	}

	messageHistory := make([]openai.ChatCompletionMessageParamUnion, 0)
	messageHistory = append(
		messageHistory,
		openai.SystemMessage(systemPrompt),
	)

	// Publish messages to NATS and build message history for AI
	for i, msg := range messages {
		dbMsg := dbMessages[i]
		natsMsg := model.Message{
			ID:        dbMsg.ID,
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

	// Create streaming channel
	streamChan := make(chan model.MessageStreamPayload, 10)

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

	// Custom onDelta that sends to our stream channel
	onDelta := func(delta agent.StreamDelta) {
		payload := model.MessageStreamPayload{
			MessageID:  assistantMessageId,
			ImageUrls:  delta.ImageURLs,
			Chunk:      delta.ContentDelta,
			Role:       model.RoleAssistant,
			IsComplete: false,
			CreatedAt:  &createdAt,
		}

		// Send to our stream channel
		select {
		case streamChan <- payload:
		case <-ctx.Done():
			s.logger.Debug("Context canceled in onDelta callback", "error", ctx.Err(), "message_id", assistantMessageId)
			return
		default:
			s.logger.Warn("Stream channel full, dropping chunk")
		}

		// Also publish to NATS for other subscribers
		_ = helpers.NatsPublish(s.nc, fmt.Sprintf("chat.%s.stream", chatID), payload)
	}

	// Start processing in a goroutine
	go func() {
		defer close(streamChan)
		s.logger.Debug("ProcessMessageHistoryStream goroutine started", "context_err", ctx.Err(), "chat_id", chatID)

		// Create custom agent for onboarding with specific tools
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
		if isOnboarding {
			// For onboarding, only provide the finalize_onboarding tool
			toolsList = []tools.Tool{NewFinalizeOnboardingTool()}
		} else {
			// For normal conversations, use the full tool registry excluding send_to_chat
			toolsList = s.toolRegistry.Excluding("send_to_chat").GetAll()
		}

		response, err := agent.ExecuteStream(ctx, messageHistory, toolsList, onDelta, false)
		if err != nil {
			s.logger.Error("Agent execution failed in stream", "error", err)
			return
		}

		s.logger.Info("Agent response", "content", response.Content)

		// Save the final assistant message to database
		subject := fmt.Sprintf("chat.%s", chatID)
		toolResults := make([]string, len(response.ToolResults))
		for i, v := range response.ToolResults {
			toolResultsJson, err := json.Marshal(v)
			if err != nil {
				s.logger.Error("failed to marshal tool results", "error", err)
				continue
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
				s.logger.Error("failed to marshal tool calls", "error", err)
			} else {
				assistantMessageDb.ToolCallsStr = helpers.Ptr(string(toolCallsJson))
			}
		}
		if len(response.ToolResults) > 0 {
			toolResultsJson, err := json.Marshal(response.ToolResults)
			if err != nil {
				s.logger.Error("failed to marshal tool results", "error", err)
			} else {
				assistantMessageDb.ToolResultsStr = helpers.Ptr(string(toolResultsJson))
			}
		}
		if len(response.ImageURLs) > 0 {
			imageURLsJson, err := json.Marshal(response.ImageURLs)
			if err != nil {
				s.logger.Error("failed to marshal image URLs", "error", err)
			} else {
				assistantMessageDb.ImageURLsStr = helpers.Ptr(string(imageURLsJson))
			}
		}

		idAssistant, err := s.storage.AddMessageToChat(ctx, assistantMessageDb)
		if err != nil {
			s.logger.Error("failed to save assistant message", "error", err)
			return
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
			s.logger.Error("failed to marshal assistant message for NATS", "error", err)
		} else {
			s.logger.Info("Publishing to NATS", "subject", subject)
			err = s.nc.Publish(subject, assistantMessageJson)
			if err != nil {
				s.logger.Error("Failed to publish to NATS", "error", err)
			}
		}

		// Index conversation asynchronously
		go func() {
			err := s.IndexConversation(context.Background(), chatID)
			if err != nil {
				s.logger.Error("failed to index conversation", "chat_id", chatID, "error", err)
			}
		}()

		finalPayload := model.MessageStreamPayload{
			MessageID:  assistantMessageId,
			ImageUrls:  []string{},
			Chunk:      "",
			Role:       model.RoleAssistant,
			IsComplete: true,
			CreatedAt:  &createdAt,
		}

		select {
		case streamChan <- finalPayload:
			s.logger.Debug("Sent final completion signal", "message_id", assistantMessageId)
		case <-ctx.Done():
			s.logger.Debug("Context canceled before sending final completion", "error", ctx.Err())
		default:
			s.logger.Warn("Stream channel full, dropping final completion signal")
		}

		_ = helpers.NatsPublish(s.nc, fmt.Sprintf("chat.%s.stream", chatID), finalPayload)
	}()

	return streamChan, nil
}

// NOTE: Do we need it?
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

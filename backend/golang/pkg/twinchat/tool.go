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

	"github.com/EternisAI/enchanted-twin/graph/model"
	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
	"github.com/EternisAI/enchanted-twin/pkg/twinchat/repository"
)

// ChatMessageTool implements a tool for adding messages to a chat.
type ChatMessageTool struct {
	logger  *log.Logger
	storage repository.Repository
	nc      *nats.Conn
}

// Definition returns the tool definition.
func (t *ChatMessageTool) Definition() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name: "send_chat_message",
			Description: param.NewOpt(
				"Communicate with your human twin by sending a message to the specified chat",
			),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"chat_id": map[string]any{
						"type":        "string",
						"description": "ID of the chat to send the message to (eg. \"chatId:2f0b10c4-7de1-43a1-85b5-ceafbab9d271\"). Either chat_id or chat_name must be provided.",
					},
					"chat_name": map[string]any{
						"type":        "string",
						"description": "Name of the chat to send the message to. If chat not found by name, a new chat with this name will be created. Either chat_id or chat_name must be provided.",
					},
					"text": map[string]any{
						"type":        "string",
						"description": "The text of the message to send to the chat.",
					},
					"image_urls": map[string]any{
						"type":        "array",
						"description": "Optional list of image URLs to include with the message",
						"items": map[string]any{
							"type": "string",
						},
					},
				},
				"required": []string{"text"},
			},
		},
	}
}

// Execute adds a message to a chat.
func (t *ChatMessageTool) Execute(
	ctx context.Context,
	args map[string]any,
) (types.ToolResult, error) {
	message, ok := args["text"].(string)
	if !ok || message == "" {
		return nil, fmt.Errorf("message `text` is required")
	}

	var chatID string
	var err error

	// Check if chat_id is provided
	if chatIDValue, ok := args["chat_id"].(string); ok && chatIDValue != "" {
		chatID = chatIDValue
	} else if chatNameValue, ok := args["chat_name"].(string); ok && chatNameValue != "" {
		// Try to find existing chat by name
		chat, err := t.storage.GetChatByName(ctx, chatNameValue)
		if err == nil {
			// Chat found by name
			chatID = chat.ID
		} else {
			// Create new chat with the provided name
			chat, err := t.storage.CreateChat(ctx, chatNameValue)
			if err != nil {
				return nil, fmt.Errorf("failed to create chat with name %s: %w", chatNameValue, err)
			}
			chatID = chat.ID
		}
	} else {
		// Neither chat_id nor chat_name provided, return an error
		return nil, fmt.Errorf("either `chat_id` or `chat_name` must be provided to create a chat")
	}

	// Always use "assistant" role since only agents can use this tool
	role := "assistant"

	// Extract optional image URLs
	var imageURLs []string
	if imageURLsRaw, ok := args["image_urls"].([]any); ok {
		for _, imageURLRaw := range imageURLsRaw {
			if imageURL, ok := imageURLRaw.(string); ok {
				imageURLs = append(imageURLs, imageURL)
			}
		}
	}

	// Generate a unique ID for the message
	messageID := uuid.New().String()
	currentTime := time.Now().Format(time.RFC3339)

	// We'll use the string role directly in the repository
	// For NATS publishing, we'll set the role when creating the natsMessage

	// Create the message
	dbMessage := repository.Message{
		ID:           messageID,
		ChatID:       chatID,
		Text:         message,
		Role:         role, // Use the string role directly
		CreatedAtStr: currentTime,
	}

	// Add image URLs if any
	if len(imageURLs) > 0 {
		imageURLsJSON, err := json.Marshal(imageURLs)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal image URLs: %w", err)
		}
		dbMessage.ImageURLsStr = &[]string{string(imageURLsJSON)}[0]
	}

	// Add message to the database
	id, err := t.storage.AddMessageToChat(ctx, dbMessage)
	if err != nil {
		return nil, fmt.Errorf("failed to add message to chat: %w", err)
	}

	// Prepare the message for NATS
	natsMessage := model.Message{
		ID:        id,
		Text:      &message,
		ImageUrls: imageURLs,
		CreatedAt: currentTime,
	}

	// Set role based on input role
	switch role {
	case "user":
		natsMessage.Role = model.RoleUser
	case "assistant":
		natsMessage.Role = model.RoleAssistant
	}

	// Marshal the message for NATS
	natsMessageJSON, err := json.Marshal(natsMessage)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal NATS message: %w", err)
	}

	// Publish the message to NATS
	subject := fmt.Sprintf("chat.%s", chatID)
	err = t.nc.Publish(subject, natsMessageJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to publish message to NATS: %w", err)
	}

	return &types.StructuredToolResult{
		ToolName:   "send_chat_message",
		ToolParams: args,
		Output: map[string]any{
			"content": fmt.Sprintf("Message sent to chat %s with ID %s", chatID, id),
		},
	}, nil
}

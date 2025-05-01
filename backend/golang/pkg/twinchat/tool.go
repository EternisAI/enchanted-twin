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
	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/twinchat/repository"
)

// ChatMessageTool implements a tool for adding messages to a chat.
type ChatMessageTool struct {
	logger  *log.Logger
	storage repository.Repository
	nc      *nats.Conn
}

// NewChatMessageTool creates a new chat message tool.
func NewChatMessageTool(
	logger *log.Logger,
	storage repository.Repository,
	nc *nats.Conn,
) *ChatMessageTool {
	return &ChatMessageTool{
		logger:  logger,
		storage: storage,
		nc:      nc,
	}
}

// Definition returns the tool definition.
func (t *ChatMessageTool) Definition() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name: "send_chat_message",
			Description: param.NewOpt(
				"Sends a message to a specified chat, allows specifying the message role (system, user, or assistant)",
			),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"chat_id": map[string]any{
						"type":        "string",
						"description": "ID of the chat to send the message to",
					},
					"message": map[string]any{
						"type":        "string",
						"description": "The message text to send",
					},
					"role": map[string]any{
						"type":        "string",
						"description": "The role of the message (user, assistant, or system)",
						"enum":        []string{"user", "assistant", "system"},
					},
					"image_urls": map[string]any{
						"type":        "array",
						"description": "Optional list of image URLs to include with the message",
						"items": map[string]any{
							"type": "string",
						},
					},
				},
				"required": []string{"chat_id", "message", "role"},
			},
		},
	}
}

// Execute adds a message to a chat.
func (t *ChatMessageTool) Execute(
	ctx context.Context,
	args map[string]any,
) (tools.ToolResult, error) {
	// Validate required parameters
	chatID, ok := args["chat_id"].(string)
	if !ok || chatID == "" {
		return tools.ToolResult{}, fmt.Errorf("chat_id is required")
	}

	message, ok := args["message"].(string)
	if !ok || message == "" {
		return tools.ToolResult{}, fmt.Errorf("message is required")
	}

	role, ok := args["role"].(string)
	if !ok || role == "" {
		return tools.ToolResult{}, fmt.Errorf("role is required")
	}

	// Validate role enum
	if role != "user" && role != "assistant" && role != "system" {
		return tools.ToolResult{}, fmt.Errorf(
			"role must be one of 'user', 'assistant', or 'system'",
		)
	}

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
			return tools.ToolResult{}, fmt.Errorf("failed to marshal image URLs: %w", err)
		}
		dbMessage.ImageURLsStr = &[]string{string(imageURLsJSON)}[0]
	}

	// Add message to the database
	id, err := t.storage.AddMessageToChat(ctx, dbMessage)
	if err != nil {
		return tools.ToolResult{}, fmt.Errorf("failed to add message to chat: %w", err)
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
		return tools.ToolResult{}, fmt.Errorf("failed to marshal NATS message: %w", err)
	}

	// Publish the message to NATS
	subject := fmt.Sprintf("chat.%s", chatID)
	err = t.nc.Publish(subject, natsMessageJSON)
	if err != nil {
		return tools.ToolResult{}, fmt.Errorf("failed to publish message to NATS: %w", err)
	}

	return tools.ToolResult{
		Content: fmt.Sprintf("Message sent to chat %s with ID %s", chatID, id),
	}, nil
}

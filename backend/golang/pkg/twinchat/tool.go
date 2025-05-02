package twinchat

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/EternisAI/enchanted-twin/graph/model"
	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
	"github.com/EternisAI/enchanted-twin/pkg/twinchat/repository"
	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
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
				"Sends a message to a specified chat as the assistant",
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
					"image_urls": map[string]any{
						"type":        "array",
						"description": "Optional list of image URLs to include with the message",
						"items": map[string]any{
							"type": "string",
						},
					},
				},
				"required": []string{"chat_id", "message"},
			},
		},
	}
}

// Execute adds a message to a chat.
func (t *ChatMessageTool) Execute(
	ctx context.Context,
	args map[string]any,
) (types.ToolResult, error) {
	// Validate required parameters
	chatID, ok := args["chat_id"].(string)
	if !ok || chatID == "" {
		return nil, fmt.Errorf("chat_id is required")
	}

	message, ok := args["message"].(string)
	if !ok || message == "" {
		return nil, fmt.Errorf("message is required")
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

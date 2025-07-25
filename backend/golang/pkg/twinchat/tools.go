package twinchat

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
	"github.com/pkg/errors"

	"github.com/EternisAI/enchanted-twin/graph/model"
	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
	"github.com/EternisAI/enchanted-twin/pkg/helpers"
	"github.com/EternisAI/enchanted-twin/pkg/twinchat/repository"
)

type chatStore interface {
	AddMessageToChat(ctx context.Context, msg repository.Message) (string, error)
	CreateChat(ctx context.Context, name string, category model.ChatCategory, holonThreadID *string) (model.Chat, error)
}

type sendToChat struct {
	chatStorage chatStore
	nc          *nats.Conn
}

func NewSendToChatTool(chatStorage chatStore, nc *nats.Conn) *sendToChat {
	return &sendToChat{
		chatStorage: chatStorage,
		nc:          nc,
	}
}

func (e *sendToChat) Execute(ctx context.Context, inputs map[string]any) (types.ToolResult, error) {
	message, ok := inputs["message"].(string)
	if !ok {
		return nil, errors.New("message is not a string")
	}

	chatId, ok := inputs["chat_id"].(string)
	if !ok {
		return nil, errors.New("chat_id is not a string")
	}

	if chatId == "" {
		chat, err := e.chatStorage.CreateChat(ctx, "Network message", model.ChatCategoryText, nil)
		if err != nil {
			return nil, err
		}
		chatId = chat.ID
	}

	dbMessage := repository.Message{
		ChatID:       chatId,
		Text:         message,
		Role:         "assistant",
		CreatedAtStr: time.Now().Format(time.RFC3339),
	}

	var imageURLs []string
	if imageURLsRaw, ok := inputs["image_urls"]; ok {
		if arr, ok := imageURLsRaw.([]any); ok {
			for _, v := range arr {
				if s, ok := v.(string); ok {
					imageURLs = append(imageURLs, s)
				}
			}
		}
	}

	if len(imageURLs) > 0 {
		imageURLsJSON, err := json.Marshal(imageURLs)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal image URLs: %w", err)
		}
		dbMessage.ImageURLsStr = helpers.Ptr(string(imageURLsJSON))
	}

	// Note: This is from the send_to_chat tool, not regular chat flow
	id, err := e.chatStorage.AddMessageToChat(ctx, dbMessage)
	if err != nil {
		return nil, fmt.Errorf("❌ Failed to store send_to_chat message to database: %w", err)
	}

	natsMessage := model.Message{
		ID:        id,
		Text:      &message,
		Role:      model.RoleAssistant,
		ImageUrls: imageURLs,
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	err = helpers.NatsPublish(e.nc, fmt.Sprintf("chat.%s", chatId), natsMessage)
	if err != nil {
		return nil, fmt.Errorf("failed to publish message to NATS: %w", err)
	}

	return &types.StructuredToolResult{
		ToolName: "send_to_chat",
		ToolParams: map[string]any{
			"chat_id": chatId,
		},
	}, nil
}

func (e *sendToChat) Definition() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name:        "send_to_chat",
			Description: param.NewOpt("This tool sends a message to the user's chat"),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"message": map[string]string{
						"type":        "string",
						"description": "The message to send to the user's chat",
					},
					"chat_id": map[string]string{
						"type":        "string",
						"description": "The ID of the chat to send the message to. No chat_id specified would send the message to a new chat.",
					},
					"image_urls": map[string]any{
						"type":        "array",
						"description": "Optional list of image URLs to include with the message",
						"items": map[string]any{
							"type": "string",
						},
					},
				},
				"required": []string{"message", "chat_id"},
			},
		},
	}
}

type finalizeOnboarding struct{}

func NewFinalizeOnboardingTool() *finalizeOnboarding {
	return &finalizeOnboarding{}
}

func (f *finalizeOnboarding) Execute(ctx context.Context, inputs map[string]any) (types.ToolResult, error) {
	name, ok := inputs["name"].(string)
	if !ok || name == "" {
		return &types.StructuredToolResult{
			ToolName:   "finalize_onboarding",
			ToolParams: inputs,
			ToolError:  "name parameter is required and must be a non-empty string",
		}, fmt.Errorf("name parameter is required")
	}

	context, _ := inputs["context"].(string) // Optional parameter

	// Create the structured response
	onboardingData := map[string]any{
		"name":    name,
		"context": context,
	}

	onboardingJSON, err := json.Marshal(onboardingData)
	if err != nil {
		return &types.StructuredToolResult{
			ToolName:   "finalize_onboarding",
			ToolParams: inputs,
			ToolError:  fmt.Sprintf("Failed to marshal onboarding data: %v", err),
		}, fmt.Errorf("failed to marshal onboarding data: %v", err)
	}

	return &types.StructuredToolResult{
		ToolName: "finalize_onboarding",
		ToolParams: map[string]any{
			"status": "completed",
		},
		Output: map[string]any{
			"content": string(onboardingJSON),
		},
	}, nil
}

func (f *finalizeOnboarding) Definition() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name:        "finalize_onboarding",
			Description: param.NewOpt("Call this tool to finalize the onboarding process after collecting the user's name and other information. Extract the name and any additional context (like favorite color, animal, etc.) from the conversation."),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "The user's name that they provided during onboarding",
					},
					"context": map[string]any{
						"type":        "string",
						"description": "Additional information the user shared (e.g., 'favorite color: blue, favorite animal: cat')",
					},
				},
				"required": []string{"name"},
			},
		},
	}
}

package telegram

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/charmbracelet/log"
	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
	"github.com/skip2/go-qrcode"

	agenttypes "github.com/EternisAI/enchanted-twin/pkg/agent/types"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/pkg/db/sqlc/config"
)

type TelegramSendMessageTool struct {
	Logger          *log.Logger
	Store           *config.Queries
	ChatServerUrl   string
	TelegramBotName string
}

func NewTelegramSendMessageTool(logger *log.Logger, store *config.Queries, chatServerUrl string, telegramBotName string) (*TelegramSendMessageTool, error) {
	if telegramBotName == "" {
		return nil, fmt.Errorf("telegram bot name is required")
	}
	return &TelegramSendMessageTool{Logger: logger, Store: store, ChatServerUrl: chatServerUrl, TelegramBotName: telegramBotName}, nil
}

func generateQRCodePNGDataURL(data string) (string, error) {
	png, err := qrcode.Encode(data, qrcode.Medium, 256) // 256 px square
	if err != nil {
		return "", err
	}
	b64 := base64.StdEncoding.EncodeToString(png)
	return "data:image/png;base64," + b64, nil
}

func (t *TelegramSendMessageTool) Execute(ctx context.Context, input map[string]any) (agenttypes.ToolResult, error) {
	message, ok := input["message"].(string)
	if !ok {
		return &agenttypes.StructuredToolResult{
			ToolName:   "telegram",
			ToolParams: input,
			ToolError:  "message parameter is required and must be a string",
		}, fmt.Errorf("message parameter is required and must be a string")
	}

	chatUUID, err := t.Store.GetConfigValue(ctx, TelegramChatUUIDKey)
	if err != nil || !chatUUID.Valid || chatUUID.String == "" {
		t.Logger.Error("error getting chat UUID", "error", err)
		return &agenttypes.StructuredToolResult{
			ToolName:   "telegram",
			ToolParams: input,
			ToolError:  fmt.Sprintf("error getting chat UUID: %v", err),
		}, fmt.Errorf("error getting chat UUID: %w", err)
	}

	telegramEnabled, err := t.Store.GetConfigValue(ctx, TelegramEnabled)
	if err != nil || !telegramEnabled.Valid || telegramEnabled.String != "true" {
		chatURL := GetChatURL(t.TelegramBotName, chatUUID.String)
		qr, err := generateQRCodePNGDataURL(chatURL)
		if err != nil {
			t.Logger.Error("failed to generate QR code,", "error", err)
		}

		return &agenttypes.StructuredToolResult{
			ToolName:   "telegram",
			ToolParams: input,
			Output: map[string]any{
				"content": fmt.Sprintf(
					"You need to start the conversation first. Open %s or scan the QR code below.",
					chatURL,
				),
				"images": []string{qr},
			},
		}, nil
	}

	_, err = PostMessage(ctx, chatUUID.String, message, t.ChatServerUrl)
	if err != nil {
		t.Logger.Error("failed to send message", "error", err)
		return &agenttypes.StructuredToolResult{
			ToolName:   "telegram",
			ToolParams: input,
			ToolError:  fmt.Sprintf("failed to send message: %v", err),
		}, fmt.Errorf("failed to send message: %w", err)
	}

	return &agenttypes.StructuredToolResult{
		ToolName:   "telegram",
		ToolParams: input,
		Output: map[string]any{
			"content": fmt.Sprintf("Message sent successfully to chat %s", chatUUID.String),
		},
	}, nil
}

func (t *TelegramSendMessageTool) Definition() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name:        "telegram_send_message",
			Description: param.NewOpt("Send a Telegram message to your human"),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"message": map[string]string{"type": "string"},
				},
				"required": []string{"message"},
			},
		},
	}
}

type TelegramSetupTool struct {
	Logger          *log.Logger
	Store           *db.Store
	ChatServerUrl   string
	TelegramBotName string
}

func NewTelegramSetupTool(logger *log.Logger, store *db.Store, chatServerUrl string, telegramBotName string) (*TelegramSetupTool, error) {
	if telegramBotName == "" {
		return nil, fmt.Errorf("telegram bot name is required")
	}
	return &TelegramSetupTool{Logger: logger, Store: store, ChatServerUrl: chatServerUrl, TelegramBotName: telegramBotName}, nil
}

func (t *TelegramSetupTool) Execute(ctx context.Context, input map[string]any) (agenttypes.ToolResult, error) {
	chatUUID, err := t.Store.GetValue(ctx, TelegramChatUUIDKey)
	if err != nil || chatUUID == "" {
		t.Logger.Error("error getting chat UUID", "error", err)
		return &agenttypes.StructuredToolResult{
			ToolName:   "telegram",
			ToolParams: input,
			ToolError:  fmt.Sprintf("error getting chat UUID: %v", err),
		}, fmt.Errorf("error getting chat UUID: %w", err)
	}

	telegramEnabled, err := GetTelegramEnabled(ctx, config.New(t.Store.DB().DB))

	t.Logger.Info("Telegram enabled", "enabled", telegramEnabled)

	if err != nil || telegramEnabled != "true" {
		chatURL := GetChatURL(t.TelegramBotName, chatUUID)
		qr, qErr := generateQRCodePNGDataURL(chatURL)
		if qErr != nil {
			t.Logger.Error("failed to generate QR code,", "error", qErr)
		}

		return &agenttypes.StructuredToolResult{
			ToolName:   "telegram",
			ToolParams: input,
			Output: map[string]any{
				"content": fmt.Sprintf(
					"You need to start the conversation first. Open %s or scan the QR code below.",
					chatURL,
				),
				"images": []string{qr},
			},
		}, nil
	}

	return &agenttypes.StructuredToolResult{
		ToolName:   "telegram_setup",
		ToolParams: input,
		Output: map[string]any{
			"content": fmt.Sprintf("Telegram is already set up and ready to use for chat %s", chatUUID),
		},
	}, nil
}

func (t *TelegramSetupTool) Definition() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name:        "telegram_setup",
			Description: param.NewOpt("Setup the Telegram chat by sending a QR code. This tool DOES NOT send telegram messages. Use telegram_send_message to send messages."),
			Parameters: openai.FunctionParameters{
				"type":       "object",
				"properties": map[string]any{},
				"required":   []string{},
			},
		},
	}
}

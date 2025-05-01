package tools

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/charmbracelet/log"
	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
	"github.com/skip2/go-qrcode" // ← add

	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/pkg/helpers"
	types "github.com/EternisAI/enchanted-twin/types"
)

type TelegramTool struct {
	Logger        *log.Logger
	Token         string
	Store         *db.Store
	ChatServerUrl string
}

func NewTelegramTool(
	logger *log.Logger,
	token string,
	store *db.Store,
	chatServerUrl string,
) *TelegramTool {
	if token == "" {
		logger.Error("TELEGRAM_TOKEN environment variable not set")
	}
	return &TelegramTool{Logger: logger, Token: token, Store: store, ChatServerUrl: chatServerUrl}
}

func generateQRCodePNGDataURL(data string) (string, error) {
	png, err := qrcode.Encode(data, qrcode.Medium, 256) // 256 px square
	if err != nil {
		return "", err
	}
	b64 := base64.StdEncoding.EncodeToString(png)
	return "data:image/png;base64," + b64, nil
}

func (t *TelegramTool) Execute(ctx context.Context, input map[string]any) (ToolResult, error) {
	if t.Token == "" {
		return ToolResult{}, fmt.Errorf("telegram token not set")
	}

	message, ok := input["message"].(string)
	if !ok {
		return ToolResult{}, fmt.Errorf("message parameter is required and must be a string")
	}

	chatUUID, err := t.Store.GetValue(ctx, types.TelegramChatUUIDKey)
	if err != nil || chatUUID == "" {
		t.Logger.Error("error getting chat UUID", "error", err)
		return ToolResult{}, fmt.Errorf("error getting chat UUID: %w", err)
	}

	fmt.Println("chatUUID", chatUUID)
	telegramEnabled, err2 := helpers.GetTelegramEnabled(ctx, t.Store)

	fmt.Println("telegramEnabled", telegramEnabled)
	if err2 != nil || telegramEnabled != "true" {
		t.Logger.Error("telegram is not enabled", "error", err2)

		chatURL := helpers.GetChatURL(types.TelegramBotName, chatUUID)
		qr, qErr := generateQRCodePNGDataURL(chatURL)
		if qErr != nil {
			t.Logger.Error("failed to generate QR code,", "error", qErr)
		}

		return ToolResult{
			Content: fmt.Sprintf(
				"You need to start the conversation first. Open %s or scan the QR code below.",
				chatURL,
			),
			ImageURLs: []string{qr},
		}, nil
	}

	_, err = helpers.PostMessage(ctx, chatUUID, message, t.ChatServerUrl)
	if err != nil {
		t.Logger.Error("failed to send message", "error", err)
		return ToolResult{}, fmt.Errorf("failed to send message: %w", err)
	}

	return ToolResult{Content: fmt.Sprintf("Message sent successfully to chat %s", chatUUID)}, nil
}

func (t *TelegramTool) Definition() openai.ChatCompletionToolParam {
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

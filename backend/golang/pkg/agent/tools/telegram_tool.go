package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/EternisAI/enchanted-twin/pkg/db"
	telegram "github.com/EternisAI/enchanted-twin/pkg/telegram"
	"github.com/charmbracelet/log"
	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
	"github.com/skip2/go-qrcode" // ← add
)

type TelegramTool struct {
	Logger *log.Logger
	Token  string
	Store  *db.Store
}

func NewTelegramTool(logger *log.Logger, token string, store *db.Store) *TelegramTool {
	if token == "" {
		logger.Error("TELEGRAM_TOKEN environment variable not set")
	}
	return &TelegramTool{Logger: logger, Token: token, Store: store}
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

	chatID, err := t.Store.GetValue(ctx, telegram.TelegramChatIDKey)
	if err != nil || chatID == "" {
		chatUUID, err := t.Store.GetValue(ctx, telegram.TelegramChatUUIDKey)
		if err != nil {
			return ToolResult{}, fmt.Errorf("error getting value from store: %w", err)
		}

		chatURL := telegram.GetChatURL(telegram.TelegramBotName, chatUUID)
		qr, qErr := generateQRCodePNGDataURL(chatURL)
		if qErr != nil {
			t.Logger.Error("failed to generate QR code", "error", qErr)
		}

		return ToolResult{
			Content: fmt.Sprintf(
				"You need to start the conversation first. Open %s or scan the QR code below.",
				chatURL,
			),
			ImageURLs: []string{qr},
		}, nil
	}

	// Construct Telegram API request
	url := fmt.Sprintf("%s/bot%s/sendMessage", telegram.TelegramAPIBase, t.Token)
	body := map[string]any{
		"chat_id":    chatID,
		"text":       message,
		"parse_mode": "HTML",
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return ToolResult{}, fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return ToolResult{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	t.Logger.Info("Sending message to Telegram", "url", url, "body", body)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ToolResult{}, fmt.Errorf("send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return ToolResult{}, fmt.Errorf("telegram API non-OK status: %d", resp.StatusCode)
	}

	var result struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ToolResult{}, fmt.Errorf("decode response: %w", err)
	}
	if !result.OK {
		return ToolResult{}, fmt.Errorf("telegram API error: %s", result.Description)
	}

	return ToolResult{Content: fmt.Sprintf("Message sent successfully to chat %s", chatID)}, nil
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

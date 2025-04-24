package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/charmbracelet/log"
	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"

	telegram "github.com/EternisAI/enchanted-twin/pkg/telegram"
)

type TelegramTool struct {
	Logger *log.Logger
	Token  string
	Store  *db.Store
}

func NewTelegramTool(logger *log.Logger, token string) *TelegramTool {
	if token == "" {
		logger.Error("TELEGRAM_TOKEN environment variable not set")
	}
	return &TelegramTool{
		Logger: logger,
		Token:  token,
	}
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
		chatURL := telegram.GetChatURL(telegram.TelegramBotName, chatUUID)
		if err != nil {
			return ToolResult{}, fmt.Errorf("You need to start the conversation first. Please go to the following URL and start the conversation: %s", chatURL)
		}
	}

	// Construct the Telegram API URL
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.Token)

	// Prepare the request body
	body := map[string]interface{}{
		"chat_id":    chatID,
		"text":       message,
		"parse_mode": "HTML",
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return ToolResult{}, fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return ToolResult{}, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	t.Logger.Info("Sending message to Telegram", "message", req)
	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return ToolResult{}, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return ToolResult{}, fmt.Errorf("telegram API returned non-OK status: %d", resp.StatusCode)
	}

	// Parse response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ToolResult{}, fmt.Errorf("failed to decode response: %w", err)
	}

	// Check if the message was sent successfully
	ok, _ = result["ok"].(bool)
	if !ok {
		return ToolResult{}, fmt.Errorf("telegram API returned error: %v", result["description"])
	}

	return ToolResult{
		Content: fmt.Sprintf("Message sent successfully to chat %s", chatID),
	}, nil
}

func (t *TelegramTool) Definition() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name:        "telegram",
			Description: param.NewOpt("Send a message to a Telegram channel"),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"message": map[string]string{
						"type": "string",
					},
				},
				"required": []string{"message"},
			},
		},
	}
}

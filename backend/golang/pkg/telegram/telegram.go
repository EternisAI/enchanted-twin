package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/agent"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	types "github.com/EternisAI/enchanted-twin/types"
	"github.com/charmbracelet/log"
	"github.com/openai/openai-go"
)

type Update struct {
	UpdateID int `json:"update_id"`
	Message  struct {
		MessageID int    `json:"message_id"`
		From      User   `json:"from"`
		Chat      Chat   `json:"chat"`
		Date      int    `json:"date"`
		Text      string `json:"text"`
	} `json:"message"`
}

type User struct {
	ID        int    `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

type Chat struct {
	ID        int    `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type Message struct {
	MessageID int    `json:"message_id"`
	From      User   `json:"from"`
	Chat      Chat   `json:"chat"`
	Date      int    `json:"date"`
	Text      string `json:"text"`
}

type GetUpdatesResponse struct {
	OK     bool      `json:"ok"`
	Result []Message `json:"result"`
}

type TelegramService struct {
	Logger           *log.Logger
	Token            string
	Client           *http.Client
	Store            *db.Store
	AiService        *ai.Service
	CompletionsModel string
	Memory           memory.Storage
	AuthStorage      *db.Store
}

type TelegramServiceInput struct {
	Logger           *log.Logger
	Token            string
	Client           *http.Client
	Store            *db.Store
	AiService        *ai.Service
	CompletionsModel string
	Memory           memory.Storage
	AuthStorage      *db.Store
}

func NewTelegramService(input TelegramServiceInput) *TelegramService {
	return &TelegramService{
		Logger:           input.Logger,
		Token:            input.Token,
		Store:            input.Store,
		Client:           &http.Client{Timeout: time.Second * 30},
		AiService:        input.AiService,
		CompletionsModel: input.CompletionsModel,
		Memory:           input.Memory,
		AuthStorage:      input.AuthStorage,
	}
}

func (s *TelegramService) Start(ctx context.Context) error {
	if s.Token == "" {
		return fmt.Errorf("telegram token not set")
	}

	lastUpdateID := 0

	for {
		select {
		case <-ctx.Done():
			return nil
		default:

			url := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?offset=%d&timeout=30", s.Token, lastUpdateID+1)

			req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
			if err != nil {
				s.Logger.Error("Failed to create request", "error", err)
				time.Sleep(time.Second * 5)
				continue
			}

			resp, err := s.Client.Do(req)
			if err != nil {
				s.Logger.Error("Failed to send request", "error", err)
				time.Sleep(time.Second * 5)
				continue
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				s.Logger.Error("failed to read response body", "error", err)
				time.Sleep(time.Second * 5)
				continue
			}
			err = resp.Body.Close()
			if err != nil {
				s.Logger.Error("failed to read response body", "error", err)
				time.Sleep(time.Second * 5)
				continue
			}

			var result struct {
				OK          bool     `json:"ok"`
				Result      []Update `json:"result"`
				Description string   `json:"description"`
				ErrorCode   int      `json:"error_code"`
			}

			if err := json.Unmarshal(body, &result); err != nil {
				s.Logger.Error("Failed to decode response", "error", err, "body", string(body))
				time.Sleep(time.Second * 5)
				continue
			}

			if !result.OK {
				s.Logger.Error("Telegram API returned error",
					"error_code", result.ErrorCode,
					"description", result.Description,
					"body", string(body),
				)
				time.Sleep(time.Second * 5)
				continue
			}

			chatID, err := s.GetChatID(ctx)
			if err != nil {
				s.Logger.Error("Failed to get chat ID", "error", err)
				time.Sleep(time.Second * 5)
				continue
			}
			s.Logger.Info("Chat ID", "chat_id", chatID)

			for _, update := range result.Result {
				lastUpdateID = update.UpdateID
				s.Logger.Info("Received message",
					"message_id", update.Message.MessageID,
					"from", update.Message.From.Username,
					"chat_id", update.Message.Chat.ID,
					"text", update.Message.Text,
				)

				if update.Message.Text != "" {
					var uuid string

					if _, err := fmt.Sscanf(update.Message.Text, "/start %s", &uuid); err == nil {

						storedUUID, err := s.GetChatUUID(ctx)
						if err != nil {
							continue
						}

						if uuid == storedUUID {
							err = s.Store.SetValue(ctx, types.TelegramChatIDKey, fmt.Sprintf("%d", update.Message.Chat.ID))
							if err != nil {
								s.Logger.Error("Failed to set chat ID", "error", err)
								continue
							}
							s.Logger.Info("Chat ID set successfully", "chat_id", update.Message.Chat.ID)
						}
					}

					response, err := s.Execute(ctx, s.TransformToOpenAIMessages([]Message{update.Message}), update.Message.Text)
					if err != nil {
						s.Logger.Error("Failed to execute message", "error", err)
						continue
					}
					err = s.SendMessage(ctx, update.Message.Chat.ID, response.Content)
					if err != nil {
						s.Logger.Error("Failed to send message", "error", err)
						continue
					}

				}

			}

			if len(result.Result) == 0 {
				time.Sleep(time.Second * 5)
			}
		}
	}
}

func (s *TelegramService) GetChatID(ctx context.Context) (string, error) {
	chatID, err := s.Store.GetValue(ctx, types.TelegramChatIDKey)
	if err != nil {
		return "", err
	}
	return chatID, nil
}

func (s *TelegramService) GetChatUUID(ctx context.Context) (string, error) {
	chatUUID, err := s.Store.GetValue(ctx, types.TelegramChatUUIDKey)
	if err != nil {
		return "", err
	}
	return chatUUID, nil
}

func (s *TelegramService) ConstructMessageHistory(ctx context.Context, chatID string, message string) ([]openai.ChatCompletionMessageParamUnion, error) {
	messageHistory := []openai.ChatCompletionMessageParamUnion{}

	return messageHistory, nil
}

func (s *TelegramService) Execute(ctx context.Context, messageHistory []openai.ChatCompletionMessageParamUnion, message string) (agent.AgentResponse, error) {
	newAgent := agent.NewAgent(s.Logger, nil, s.AiService, s.CompletionsModel, nil, nil)

	twitterReverseChronTimelineTool := tools.NewTwitterTool(*s.Store)
	tools := []tools.Tool{
		&tools.SearchTool{},
		&tools.ImageTool{},
		tools.NewMemorySearchTool(s.Logger, s.Memory),
		tools.NewTelegramTool(s.Logger, s.Token, s.Store),
		twitterReverseChronTimelineTool,
	}

	response, err := newAgent.Execute(ctx, messageHistory, tools)
	if err != nil {
		return agent.AgentResponse{}, err
	}
	s.Logger.Debug("Agent response", "content", response.Content, "tool_calls", len(response.ToolCalls), "tool_results", len(response.ToolResults))

	return response, nil
}

func (s *TelegramService) GetLatestMessages(ctx context.Context) ([]Message, error) {
	chatID, err := s.GetChatID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat ID: %w", err)
	}
	url := fmt.Sprintf("%s/bot%s/getUpdates?chat_id=%s&limit=10", types.TelegramAPIBase, s.Token, chatID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var result GetUpdatesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !result.OK {
		return nil, fmt.Errorf("telegram API returned error: %s", string(body))
	}

	return result.Result, nil
}

func (s *TelegramService) TransformToOpenAIMessages(messages []Message) []openai.ChatCompletionMessageParamUnion {
	openAIMessages := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages))

	for _, msg := range messages {
		// Skip empty messages
		if msg.Text == "" {
			continue
		}

		// Determine the role based on the message sender
		if msg.From.Username == "enchanted_twin_bot" {
			openAIMessages = append(openAIMessages, openai.AssistantMessage(msg.Text))
		} else {
			openAIMessages = append(openAIMessages, openai.UserMessage(msg.Text))
		}
	}

	return openAIMessages
}

func (s *TelegramService) SendMessage(ctx context.Context, chatID int, message string) error {
	url := fmt.Sprintf("%s/bot%s/sendMessage", types.TelegramAPIBase, s.Token)
	body := map[string]any{
		"chat_id":    chatID,
		"text":       message,
		"parse_mode": "HTML",
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	s.Logger.Info("Sending message to Telegram", "url", url, "body", body)

	resp, err := s.Client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API non-OK status: %d", resp.StatusCode)
	}

	var result struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if !result.OK {
		return fmt.Errorf("telegram API error: %s", result.Description)
	}

	return nil
}

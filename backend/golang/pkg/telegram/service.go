package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/charmbracelet/log"
)

const (
	// TelegramChatUUIDKey allows to identifies the chat with a specific user, after the first message
	TelegramChatUUIDKey = "telegram_chat_uuid"
	// TelegramChatIDKey is the telegram chat id to be used for sending messages
	TelegramChatIDKey = "telegram_chat_id"
	// TelegramBotName is the telegram bot name to be used for sending messages
	TelegramBotName = "HelloIamBernieBot"

	TelegramAPIBase = "https://api.telegram.org"
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

type TelegramService struct {
	logger *log.Logger
	token  string
	client *http.Client
	store  *db.Store
}

func NewTelegramService(logger *log.Logger, token string, store *db.Store) *TelegramService {
	return &TelegramService{
		logger: logger,
		token:  token,
		store:  store,
		client: &http.Client{
			Timeout: time.Second * 30,
		},
	}
}

func (s *TelegramService) Start(ctx context.Context) error {
	if s.token == "" {
		return fmt.Errorf("telegram token not set")
	}

	lastUpdateID := 0

	for {
		select {
		case <-ctx.Done():
			return nil
		default:

			url := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?offset=%d&timeout=30", s.token, lastUpdateID+1)

			req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
			if err != nil {
				s.logger.Error("Failed to create request", "error", err)
				time.Sleep(time.Second * 5)
				continue
			}

			resp, err := s.client.Do(req)
			if err != nil {
				s.logger.Error("Failed to send request", "error", err)
				time.Sleep(time.Second * 5)
				continue
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				s.logger.Error("failed to read response body", "error", err)
				time.Sleep(time.Second * 5)
				continue
			}
			err = resp.Body.Close()
			if err != nil {
				s.logger.Error("failed to read response body", "error", err)
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
				s.logger.Error("Failed to decode response", "error", err, "body", string(body))
				time.Sleep(time.Second * 5)
				continue
			}

			if !result.OK {
				// s.logger.Error("Telegram API returned error",
				// 	"error_code", result.ErrorCode,
				// 	"description", result.Description,
				// 	"body", string(body),
				// )
				time.Sleep(time.Second * 5)
				continue
			}

			chatID, err := s.GetChatID(ctx)
			if err != nil {
				s.logger.Error("Failed to get chat ID", "error", err)
				time.Sleep(time.Second * 5)
				continue
			}
			s.logger.Info("Chat ID", "chat_id", chatID)

			for _, update := range result.Result {
				lastUpdateID = update.UpdateID
				s.logger.Info("Received message",
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
							s.logger.Error("Failed to get stored chat UUID", "error", err)
							continue
						}

						if uuid == storedUUID {
							err = s.store.SetValue(ctx, TelegramChatIDKey, fmt.Sprintf("%d", update.Message.Chat.ID))
							if err != nil {
								s.logger.Error("Failed to set chat ID", "error", err)
								continue
							}
							s.logger.Info("Chat ID set successfully", "chat_id", update.Message.Chat.ID)
						}
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
	chatID, err := s.store.GetValue(ctx, TelegramChatIDKey)
	if err != nil {
		return "", err
	}
	return chatID, nil
}

func (s *TelegramService) GetChatUUID(ctx context.Context) (string, error) {
	chatUUID, err := s.store.GetValue(ctx, TelegramChatUUIDKey)
	if err != nil {
		return "", err
	}
	return chatUUID, nil
}

func GetChatURL(botName string, chatUUID string) string {
	return fmt.Sprintf("https://t.me/%s?start=%s", botName, chatUUID)
}

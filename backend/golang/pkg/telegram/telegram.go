// Owner: slimane@eternis.ai
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/gorilla/websocket"
	"github.com/nats-io/nats.go"
	"github.com/openai/openai-go"

	"github.com/EternisAI/enchanted-twin/pkg/agent"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/pkg/helpers"
	types "github.com/EternisAI/enchanted-twin/types"
)

var ErrSubscriptionNilTextMessage = errors.New("subscription stopped due to nil text message")

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
	LastMessages     []Message
	NatsClient       *nats.Conn
	ChatServerUrl    string
	ToolsRegistry    *tools.ToolMapRegistry
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
	NatsClient       *nats.Conn
	ChatServerUrl    string
	ToolsRegistry    *tools.ToolMapRegistry
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
		LastMessages:     []Message{},
		NatsClient:       input.NatsClient,
		ChatServerUrl:    input.ChatServerUrl,
		ToolsRegistry:    input.ToolsRegistry,
	}
}

func (s *TelegramService) Start(ctx context.Context) error {
	if s.Token == "" {
		return fmt.Errorf("telegram token not set")
	}

	lastUpdateID := 0

	s.Logger.Info("Starting telegram service")

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			url := fmt.Sprintf(
				"https://api.telegram.org/bot%s/getUpdates?offset=%d&timeout=30",
				s.Token,
				lastUpdateID+1,
			)

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

			s.Logger.Info("Received updates", "body", string(body))
			if err := json.Unmarshal(body, &result); err != nil {
				s.Logger.Error("Failed to decode response", "error", err)
				time.Sleep(time.Second * 5)
				continue
			}

			s.Logger.Error("Received updates", "result", result)
			if !result.OK {
				s.Logger.Error("Telegram API returned error",
					"error_code", result.ErrorCode,
					"description", result.Description,
					"body", string(body),
				)
				time.Sleep(time.Second * 5)
				continue
			}

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

					s.Logger.Info("Chat ID", "chat_id", update.Message.Chat)

					chatID := update.Message.Chat.ID
					if _, err := fmt.Sscanf(update.Message.Text, "/start %s", &uuid); err == nil {
						s.Logger.Info("Creating chat", "chat_id", chatID, "uuid", uuid)
						_, err := s.CreateChat(ctx, chatID, uuid)
						if err != nil {
							s.Logger.Error("Failed to create chat", "error", err)
							continue
						}
						err = s.SendMessage(ctx, chatID, "Send any message to start the conversation")
						if err != nil {
							s.Logger.Error("Failed to send message", "error", err)
							continue
						}
					}

					if s.NatsClient != nil {
						subject := fmt.Sprintf("telegram.chat.%d", chatID)
						messageBytes, err := json.Marshal(update.Message)
						if err != nil {
							s.Logger.Error("Failed to marshal message", "error", err)
							continue
						}

						err = s.NatsClient.Publish(subject, messageBytes)
						if err != nil {
							s.Logger.Error("Failed to publish message to NATS", "error", err)
							continue
						}
						s.Logger.Info("Published message to NATS", "subject", subject)
					}
				}
			}

			if len(result.Result) == 0 {
				time.Sleep(time.Second * 5)
			}
		}
	}
}

func (s *TelegramService) CreateChat(
	ctx context.Context,
	chatID int,
	chatUUID string,
) (int, error) {
	err := s.Store.SetValue(ctx, fmt.Sprintf("telegram_chat_id_%s", chatUUID), strconv.Itoa(chatID))
	if err != nil {
		return 0, err
	}
	return chatID, nil
}

func (s *TelegramService) GetChatID(ctx context.Context) (int, error) {
	chatUUID, err := s.Store.GetValue(ctx, types.TelegramChatUUIDKey)
	if err != nil {
		return 0, err
	}
	chatID, err := s.GetChatIDFromChatUUID(ctx, chatUUID)
	if err != nil {
		return 0, err
	}
	return chatID, nil
}

func (s *TelegramService) GetChatIDFromChatUUID(
	ctx context.Context,
	chatUUID string,
) (int, error) {
	chatIDStr, err := s.Store.GetValue(ctx, fmt.Sprintf("telegram_chat_id_%s", chatUUID))
	if err != nil {
		return 0, err
	}
	chatID, err := strconv.Atoi(chatIDStr)
	if err != nil {
		return 0, fmt.Errorf("invalid chat ID format: %w", err)
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

func (s *TelegramService) Execute(
	ctx context.Context,
	messageHistory []openai.ChatCompletionMessageParamUnion,
	message string,
) (agent.AgentResponse, error) {
	newAgent := agent.NewAgent(s.Logger, nil, s.AiService, s.CompletionsModel, nil, nil)

	toolsList := []tools.Tool{}
	for _, name := range s.ToolsRegistry.Excluding("sleep", "sleep_until").List() {
		if tool, exists := s.ToolsRegistry.Get(name); exists {
			toolsList = append(toolsList, tool)
		}
	}

	origin := map[string]any{
		"source": "telegram",
	}
	response, err := newAgent.Execute(ctx, origin, messageHistory, toolsList)
	if err != nil {
		return agent.AgentResponse{}, err
	}
	s.Logger.Debug(
		"Agent response",
		"content",
		response.Content,
		"tool_calls",
		len(response.ToolCalls),
		"tool_results",
		len(response.ToolResults),
	)

	return response, nil
}

func (s *TelegramService) GetLatestMessages(ctx context.Context) ([]Message, error) {
	lastUpdateID, err := s.Store.GetValue(ctx, types.TelegramLastUpdateIDKey)
	if err != nil {
		lastUpdateID = "0"
	}

	lastUpdateIDInt, err := strconv.Atoi(lastUpdateID)
	if err != nil {
		return nil, fmt.Errorf("failed to convert last update ID to int: %w", err)
	}

	url := fmt.Sprintf(
		"%s/bot%s/getUpdates?offset=%d&limit=10",
		types.TelegramAPIBase,
		s.Token,
		lastUpdateIDInt,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			s.Logger.Warn("Failed to close response body", "error", err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var result struct {
		OK     bool     `json:"ok"`
		Result []Update `json:"result"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !result.OK {
		return nil, fmt.Errorf("telegram API returned error: %s", string(body))
	}

	messages := make([]Message, 0, len(result.Result))
	for _, update := range result.Result {
		messages = append(messages, update.Message)
		if update.UpdateID > lastUpdateIDInt {
			lastUpdateIDInt = update.UpdateID
		}
	}

	err = s.Store.SetValue(ctx, types.TelegramLastUpdateIDKey, strconv.Itoa(lastUpdateIDInt))
	if err != nil {
		s.Logger.Error("Failed to store last update ID", "error", err)
	}

	return messages, nil
}

func (s *TelegramService) TransformToOpenAIMessages(
	messages []Message,
) []openai.ChatCompletionMessageParamUnion {
	openAIMessages := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages))

	for _, msg := range messages {
		if msg.Text == "" {
			continue
		}

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
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			s.Logger.Warn("Failed to close response body", "error", err)
		}
	}()

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

func (s *TelegramService) transformWebSocketDataToMessage(ctx context.Context, data struct {
	ID        string
	Text      *string
	Role      string
	CreatedAt string
}, chatUUID string,
) (*Message, error) {
	if data.Text == nil {
		return nil, fmt.Errorf("received WebSocket data with nil text")
	}
	messageText := *data.Text

	messageID, err := strconv.Atoi(data.ID)
	if err != nil {
		s.Logger.Warn(
			"Failed to convert WebSocket message ID (GraphQL ID) to int. Using 0.",
			"error",
			err,
			"graphQL_ID",
			data.ID,
		)
		messageID = 0
	}

	fromUser := User{
		Username: data.Role,
	}

	chatInfo := Chat{
		ID: 0,
	}

	chatIDInt, err := s.GetChatIDFromChatUUID(ctx, chatUUID)
	if err == nil {
		chatInfo.ID = chatIDInt
	}

	parsedTime, err := time.Parse(time.RFC3339, data.CreatedAt)
	if err != nil {
		s.Logger.Error(
			"Failed to parse CreatedAt timestamp from WebSocket",
			"error",
			err,
			"timestamp",
			data.CreatedAt,
		)
		return nil, fmt.Errorf("failed to parse CreatedAt timestamp from WebSocket: %w", err)
	}
	date := int(parsedTime.Unix())

	msg := &Message{
		MessageID: messageID,
		From:      fromUser,
		Chat:      chatInfo,
		Date:      date,
		Text:      messageText,
	}

	return msg, nil
}

func (s *TelegramService) Subscribe(ctx context.Context, chatUUID string) error {
	if s.Logger == nil {
		return fmt.Errorf("logger is nil")
	}

	wsURL := strings.Replace(s.ChatServerUrl, "http", "ws", 1)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket (%s): %w", wsURL, err)
	}
	defer func() {
		err := conn.Close()
		if err != nil {
			s.Logger.Warn("Failed to close WebSocket connection", "error", err)
		}
	}()

	initMsg := map[string]interface{}{
		"type": "connection_init",
		"payload": map[string]interface{}{
			"headers": map[string]string{},
		},
	}

	if err := conn.WriteJSON(initMsg); err != nil {
		return fmt.Errorf("failed to send connection initialization: %w", err)
	}

	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		s.Logger.Warn("Failed to set read deadline", "error", err)
	}
	var ackResponse struct {
		Type string `json:"type"`
	}
	if err := conn.ReadJSON(&ackResponse); err != nil {
		return fmt.Errorf("failed to read connection acknowledgment: %w", err)
	}

	if ackResponse.Type != "connection_ack" {
		return fmt.Errorf("unexpected response type: %s", ackResponse.Type)
	}

	subscription := map[string]interface{}{
		"type": "start",
		"id":   "1",
		"payload": map[string]interface{}{
			"query": `
				subscription TelegramMessageAdded($chatUUID: ID!) {
					telegramMessageAdded(chatUUID: $chatUUID) {
						id
						text
						role
						createdAt
					}
				}
			`,
			"variables": map[string]interface{}{
				"chatUUID": chatUUID,
			},
			"operationName": "TelegramMessageAdded",
		},
	}

	if err := conn.WriteJSON(subscription); err != nil {
		return fmt.Errorf("failed to send subscription request: %w", err)
	}

	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		s.Logger.Warn("Failed to reset read deadline", "error", err)
	}
	if err := conn.SetWriteDeadline(time.Time{}); err != nil {
		s.Logger.Warn("Failed to reset write deadline", "error", err)
	}

	readerExitChan := make(chan error, 1)

	go func() {
		var exitErr error
		defer func() {
			readerExitChan <- exitErr
			close(readerExitChan)
		}()

		reconnectDelay := time.Second
		maxReconnectDelay := time.Minute
		reconnectAttempts := 0
		maxReconnectAttempts := 5
		lastSuccessfulConnection := time.Now()

		connectionAcknowledged := true

		for {
			select {
			case <-ctx.Done():
				exitErr = ctx.Err()
				return
			default:

				if err := conn.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
					s.Logger.Warn("Failed to set read deadline in loop", "error", err)
				}

				var response struct {
					Type    string `json:"type"`
					ID      string `json:"id"`
					Payload struct {
						Data struct {
							TelegramMessageAdded struct {
								ID        string
								Text      *string
								Role      string
								CreatedAt string
							} `json:"telegramMessageAdded"`
						} `json:"data"`
						Errors []struct {
							Message string `json:"message"`
						} `json:"errors"`
					} `json:"payload"`
				}

				if err := conn.ReadJSON(&response); err != nil {
					if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
						if err := conn.SetReadDeadline(time.Time{}); err != nil {
							s.Logger.Warn(
								"Failed to reset read deadline after timeout",
								"error",
								err,
							)
						}
						continue
					}

					if err := conn.Close(); err != nil {
						s.Logger.Warn("Failed to close connection after read error", "error", err)
					}

					if time.Since(lastSuccessfulConnection) > time.Minute {
						reconnectAttempts = 0
					}

					if reconnectAttempts >= maxReconnectAttempts {
						exitErr = fmt.Errorf(
							"max reconnection attempts reached after error: %w",
							err,
						)
						return
					}

				reconnectLoop:
					for {
						select {
						case <-ctx.Done():
							exitErr = ctx.Err()
							return
						default:

							actualDelay := time.Duration(math.Min(float64(reconnectDelay), float64(maxReconnectDelay)))
							time.Sleep(actualDelay)
							reconnectDelay *= 2

							newConn, _, dialErr := websocket.DefaultDialer.Dial(wsURL, nil)
							if dialErr != nil {
								s.Logger.Error("Failed to reconnect", "error", dialErr, "attempt", reconnectAttempts+1)
								reconnectAttempts++
								if reconnectAttempts >= maxReconnectAttempts {
									s.Logger.Error("Max reconnection attempts reached after failed dial", "error", dialErr)
									exitErr = fmt.Errorf("max reconnection attempts reached after dial error: %w", dialErr)
									return
								}
								continue
							}

							reconnectDelay = time.Second
							reconnectAttempts = 0
							lastSuccessfulConnection = time.Now()
							connectionAcknowledged = false

							if err := newConn.WriteJSON(initMsg); err != nil {
								s.Logger.Error("Failed to send connection initialization on reconnect", "error", err)
								if closeErr := newConn.Close(); closeErr != nil {
									s.Logger.Warn("Failed to close new connection after init write error", "error", closeErr)
								}
								continue reconnectLoop
							}

							if err := newConn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
								s.Logger.Warn("Failed to set read deadline on reconnect ack", "error", err)
							}
							if err := newConn.ReadJSON(&ackResponse); err != nil {
								s.Logger.Error("Failed to read connection acknowledgment on reconnect", "error", err)
								if closeErr := newConn.Close(); closeErr != nil {
									s.Logger.Warn("Failed to close new connection after ack read error", "error", closeErr)
								}
								continue reconnectLoop
							}
							if err := newConn.SetReadDeadline(time.Time{}); err != nil {
								s.Logger.Warn("Failed to reset read deadline after reconnect ack", "error", err)
							}

							if ackResponse.Type != "connection_ack" {
								s.Logger.Error("Unexpected response type on reconnect", "type", ackResponse.Type)
								if closeErr := newConn.Close(); closeErr != nil {
									s.Logger.Warn("Failed to close new connection after unexpected ack type", "error", closeErr)
								}
								continue reconnectLoop
							}
							connectionAcknowledged = true

							if err := newConn.WriteJSON(subscription); err != nil {
								s.Logger.Error("Failed to resend subscription", "error", err)
								if closeErr := newConn.Close(); closeErr != nil {
									s.Logger.Warn("Failed to close new connection after subscription write error", "error", closeErr)
								}
								continue reconnectLoop
							}

							if err := newConn.SetWriteDeadline(time.Time{}); err != nil {
								s.Logger.Warn("Failed to reset write deadline on reconnect", "error", err)
							}

							conn = newConn
							break reconnectLoop
						}
					}

					continue
				}
				if err := conn.SetReadDeadline(time.Time{}); err != nil {
					s.Logger.Warn(
						"Failed to reset read deadline after successful read",
						"error",
						err,
					)
				}

				if response.Type == "data" {
					// s.Logger.Info("telegramenabled", "Received data", "response", response)
					if response.Payload.Data.TelegramMessageAdded.Text == nil {
						// s.Logger.Info("telegramenabled", "Received nil text message")
						exitErr = ErrSubscriptionNilTextMessage
						return
					}

					s.Logger.Info(
						"Received message",
						"message",
						response.Payload.Data.TelegramMessageAdded.Text,
					)
					newMessage, err := s.transformWebSocketDataToMessage(
						ctx,
						response.Payload.Data.TelegramMessageAdded,
						chatUUID,
					)
					if err != nil {
						continue
					}

					s.LastMessages = append(s.LastMessages, *newMessage)

					if newMessage.Chat.ID != 0 {
						fmt.Println("Setting telegram enabled to true")
						telegramEnabled, err := helpers.GetTelegramEnabled(ctx, s.Store)
						if err != nil {
							s.Logger.Info("telegramenabled", "Error getting telegram enabled", "error", err)
						}
						if telegramEnabled != "true" {
							err := s.Store.SetValue(ctx, types.TelegramEnabled, fmt.Sprintf("%t", true))
							if err != nil {
								s.Logger.Error("Error setting telegram enabled", "error", err)
							}
						}
					}

					agentResponse, err := s.Execute(
						ctx,
						s.TransformToOpenAIMessages(s.LastMessages),
						newMessage.Text,
					)
					if err != nil {
						continue
					}

					if agentResponse.Content != "" {
						_, err := helpers.PostMessage(
							ctx,
							chatUUID,
							agentResponse.Content,
							s.ChatServerUrl,
						)
						if err != nil {
							s.Logger.Error("Error with GraphQL mutation response", "error", err)
						} else {
							s.Logger.Info("Successfully sent agent response via GraphQL mutation")

							agentMessage := Message{
								MessageID: 0,
								From:      User{Username: "enchanted_twin_bot"},
								Chat:      newMessage.Chat,
								Date:      int(time.Now().Unix()),
								Text:      agentResponse.Content,
							}
							s.LastMessages = append(s.LastMessages, agentMessage)

							if len(s.LastMessages) > 10 {
								s.LastMessages = s.LastMessages[len(s.LastMessages)-10:]
							}
						}
					}
				} else if response.Type == "connection_ack" {
					connectionAcknowledged = true
				} else if response.Type == "ka" {
				} else if response.Type == "error" {
					if !connectionAcknowledged {
						exitErr = fmt.Errorf("received error before connection ack: %v", response.Payload.Errors)
						if err := conn.Close(); err != nil {
							s.Logger.Warn("Failed to close connection after error before ack", "error", err)
						}
						return
					}
				} else {
					s.Logger.Info("Received message of unhandled type", "type", response.Type, "response", response)
				}
			}
		}
	}()

	select {
	case <-ctx.Done():

		return ctx.Err()
	case err := <-readerExitChan:

		return err
	}
}

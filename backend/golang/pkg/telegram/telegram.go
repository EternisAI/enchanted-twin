// Owner: slimane@eternis.ai
package telegram

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/nats-io/nats.go"
	"github.com/openai/openai-go"

	agent "github.com/EternisAI/enchanted-twin/pkg/agent"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/config"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	configtable "github.com/EternisAI/enchanted-twin/pkg/db/sqlc/config"
)

const (
	// TelegramEnabled is the flag to enable telegram.
	TelegramEnabled = "telegram_enabled"
	// TelegramChatUUIDKey allows to identifies the chat with a specific user, after the first message.
	TelegramChatUUIDKey = "telegram_chat_uuid"
	// TelegramLastUpdateIDKey is used to track the last update ID for Telegram messages.
	TelegramLastUpdateIDKey = "telegram_last_update_id"
	// TelegramAPIBase is the base url for the telegram api.
	TelegramAPIBase = "https://api.telegram.org"
)

var ErrSubscriptionNilTextMessage = errors.New("subscription stopped due to nil text message")

type rateLimiter struct {
	mutex    sync.Mutex
	lastLog  time.Time
	interval time.Duration
}

func newRateLimiter(interval time.Duration) *rateLimiter {
	return &rateLimiter{interval: interval}
}

func (r *rateLimiter) shouldLog() bool {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	now := time.Now()
	if now.Sub(r.lastLog) >= r.interval {
		r.lastLog = now
		return true
	}
	return false
}

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
	Logger              *log.Logger
	Client              *http.Client
	Store               *db.Store
	AiService           *ai.Service
	CompletionsModel    string
	Memory              memory.Storage
	AuthStorage         *db.Store
	LastMessages        []Message
	NatsClient          *nats.Conn
	ChatServerUrl       string
	ToolsRegistry       *tools.ToolMapRegistry
	panicLogLimiter     *rateLimiter
	unhandledLogLimiter *rateLimiter
}

type safeWebSocketConn struct {
	conn  *websocket.Conn
	mutex sync.RWMutex
}

func (s *safeWebSocketConn) getConn() *websocket.Conn {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.conn
}

func (s *safeWebSocketConn) setConn(newConn *websocket.Conn) *websocket.Conn {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	oldConn := s.conn
	s.conn = newConn
	return oldConn
}

func (s *safeWebSocketConn) safeReadJSON(v interface{}, logger *log.Logger, rateLimiter *rateLimiter) error {
	conn := s.getConn()
	if conn == nil {
		return fmt.Errorf("connection is nil")
	}

	defer func() {
		if r := recover(); r != nil {
			if logger != nil && rateLimiter != nil && rateLimiter.shouldLog() {
				logger.Error("Recovered from panic in safeReadJSON", "panic", r)
			}
		}
	}()

	return conn.ReadJSON(v)
}

func (s *safeWebSocketConn) safeWriteJSON(v interface{}, logger *log.Logger) error {
	conn := s.getConn()
	if conn == nil {
		return fmt.Errorf("connection is nil")
	}

	defer func() {
		if r := recover(); r != nil {
			if logger != nil {
				logger.Error("Recovered from panic in safeWriteJSON", "panic", r)
			}
		}
	}()

	return conn.WriteJSON(v)
}

func (s *safeWebSocketConn) safeSetReadDeadline(t time.Time, logger *log.Logger) error {
	conn := s.getConn()
	if conn == nil {
		return fmt.Errorf("connection is nil")
	}

	defer func() {
		if r := recover(); r != nil {
			if logger != nil {
				logger.Error("Recovered from panic in safeSetReadDeadline", "panic", r)
			}
		}
	}()

	return conn.SetReadDeadline(t)
}

func (s *safeWebSocketConn) safeSetWriteDeadline(t time.Time, logger *log.Logger) error {
	conn := s.getConn()
	if conn == nil {
		return fmt.Errorf("connection is nil")
	}

	defer func() {
		if r := recover(); r != nil {
			if logger != nil {
				logger.Error("Recovered from panic in safeSetWriteDeadline", "panic", r)
			}
		}
	}()

	return conn.SetWriteDeadline(t)
}

func (s *safeWebSocketConn) safeClose(logger *log.Logger) error {
	conn := s.getConn()
	if conn == nil {
		return nil
	}

	defer func() {
		if r := recover(); r != nil {
			if logger != nil {
				logger.Error("Recovered from panic in safeClose", "panic", r)
			}
		}
	}()

	return conn.Close()
}

type TelegramServiceInput struct {
	Logger           *log.Logger
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
		Logger:              input.Logger,
		Store:               input.Store,
		Client:              &http.Client{Timeout: time.Second * 30},
		AiService:           input.AiService,
		CompletionsModel:    input.CompletionsModel,
		Memory:              input.Memory,
		AuthStorage:         input.AuthStorage,
		LastMessages:        []Message{},
		NatsClient:          input.NatsClient,
		ChatServerUrl:       input.ChatServerUrl,
		ToolsRegistry:       input.ToolsRegistry,
		panicLogLimiter:     newRateLimiter(30 * time.Second),
		unhandledLogLimiter: newRateLimiter(30 * time.Second),
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

func (s *TelegramService) GetChatIDFromChatUUID(
	ctx context.Context,
	chatUUID string,
) (int, error) {
	chatIDStr, err := s.Store.GetValue(ctx, fmt.Sprintf("telegram_chat_id_%s", chatUUID))
	if err != nil || chatIDStr == "" {
		return 0, err
	}
	chatID, err := strconv.Atoi(chatIDStr)
	if err != nil {
		return 0, fmt.Errorf("invalid chat ID format: %w", err)
	}
	return chatID, nil
}

func (s *TelegramService) GetChatUUID(ctx context.Context) (string, error) {
	chatUUID, err := s.Store.GetValue(ctx, TelegramChatUUIDKey)
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
	newAgent := agent.NewAgent(s.Logger, nil, s.AiService, s.CompletionsModel, s.CompletionsModel, nil, nil)

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

	rawConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket (%s): %w", wsURL, err)
	}

	safeConn := &safeWebSocketConn{conn: rawConn}
	defer func() {
		if err := safeConn.safeClose(s.Logger); err != nil {
			s.Logger.Warn("Failed to close WebSocket connection", "error", err)
		}
	}()

	initMsg := map[string]interface{}{
		"type": "connection_init",
		"payload": map[string]interface{}{
			"headers": map[string]string{},
		},
	}

	if err := safeConn.safeWriteJSON(initMsg, s.Logger); err != nil {
		s.Logger.Error("Failed to send connection initialization", "error", err)
		return fmt.Errorf("failed to send connection initialization: %w", err)
	}

	if err := safeConn.safeSetReadDeadline(time.Now().Add(5*time.Second), s.Logger); err != nil {
		s.Logger.Warn("Failed to set read deadline", "error", err)
	}

	var ackResponse struct {
		Type string `json:"type"`
	}
	if err := safeConn.safeReadJSON(&ackResponse, s.Logger, s.panicLogLimiter); err != nil {
		s.Logger.Error("Failed to read connection acknowledgment", "error", err)
		return fmt.Errorf("failed to read connection acknowledgment: %w", err)
	}

	if ackResponse.Type != "connection_ack" {
		s.Logger.Error("Unexpected connection response type", "expected", "connection_ack", "received", ackResponse.Type)
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

	if err := safeConn.safeWriteJSON(subscription, s.Logger); err != nil {
		s.Logger.Error("Failed to send subscription request", "error", err)
		return fmt.Errorf("failed to send subscription request: %w", err)
	}

	if err := safeConn.safeSetReadDeadline(time.Time{}, s.Logger); err != nil {
		s.Logger.Warn("Failed to reset read deadline", "error", err)
	}
	if err := safeConn.safeSetWriteDeadline(time.Time{}, s.Logger); err != nil {
		s.Logger.Warn("Failed to reset write deadline", "error", err)
	}

	readerExitChan := make(chan error, 1)

	go func() {
		var exitErr error
		defer func() {
			if r := recover(); r != nil {
				s.Logger.Error("Recovered from panic in WebSocket goroutine", "panic", r)
				exitErr = fmt.Errorf("panic in WebSocket goroutine: %v", r)
			}
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
				if err := safeConn.safeSetReadDeadline(time.Now().Add(30*time.Second), s.Logger); err != nil {
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

				if err := safeConn.safeReadJSON(&response, s.Logger, s.panicLogLimiter); err != nil {
					s.Logger.Error("Failed to read WebSocket message", "error", err)

					if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
						s.Logger.Debug("Read timeout occurred, continuing")
						if err := safeConn.safeSetReadDeadline(time.Time{}, s.Logger); err != nil {
							s.Logger.Warn(
								"Failed to reset read deadline after timeout",
								"error",
								err,
							)
						}
						continue
					}

					s.Logger.Warn("Closing connection due to read error")
					if err := safeConn.safeClose(s.Logger); err != nil {
						s.Logger.Warn("Failed to close connection after read error", "error", err)
					}

					if time.Since(lastSuccessfulConnection) > time.Minute {
						s.Logger.Debug("Resetting reconnect attempts due to time elapsed")
						reconnectAttempts = 0
					}

					if reconnectAttempts >= maxReconnectAttempts {
						s.Logger.Error("Maximum reconnection attempts reached", "attempts", reconnectAttempts)
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

							tempSafeConn := &safeWebSocketConn{conn: newConn}

							s.Logger.Debug("Sending connection initialization on reconnect")
							if err := tempSafeConn.safeWriteJSON(initMsg, s.Logger); err != nil {
								s.Logger.Error("Failed to send connection initialization on reconnect", "error", err)
								if closeErr := tempSafeConn.safeClose(s.Logger); closeErr != nil {
									s.Logger.Warn("Failed to close new connection after init write error", "error", closeErr)
								}
								continue reconnectLoop
							}

							if err := tempSafeConn.safeSetReadDeadline(time.Now().Add(10*time.Second), s.Logger); err != nil {
								s.Logger.Warn("Failed to set read deadline on reconnect ack", "error", err)
							}

							s.Logger.Debug("Reading connection acknowledgment on reconnect")
							if err := tempSafeConn.safeReadJSON(&ackResponse, s.Logger, s.panicLogLimiter); err != nil {
								s.Logger.Error("Failed to read connection acknowledgment on reconnect", "error", err)
								if closeErr := tempSafeConn.safeClose(s.Logger); closeErr != nil {
									s.Logger.Warn("Failed to close new connection after ack read error", "error", closeErr)
								}
								continue reconnectLoop
							}
							if err := tempSafeConn.safeSetReadDeadline(time.Time{}, s.Logger); err != nil {
								s.Logger.Warn("Failed to reset read deadline after reconnect ack", "error", err)
							}

							s.Logger.Debug("Received acknowledgment on reconnect", "type", ackResponse.Type)
							if ackResponse.Type != "connection_ack" {
								s.Logger.Error("Unexpected response type on reconnect", "type", ackResponse.Type)
								if closeErr := tempSafeConn.safeClose(s.Logger); closeErr != nil {
									s.Logger.Warn("Failed to close new connection after unexpected ack type", "error", closeErr)
								}
								continue reconnectLoop
							}
							connectionAcknowledged = true

							s.Logger.Debug("Resending subscription on reconnect")
							if err := tempSafeConn.safeWriteJSON(subscription, s.Logger); err != nil {
								s.Logger.Error("Failed to resend subscription", "error", err)
								if closeErr := tempSafeConn.safeClose(s.Logger); closeErr != nil {
									s.Logger.Warn("Failed to close new connection after subscription write error", "error", closeErr)
								}
								continue reconnectLoop
							}

							if err := tempSafeConn.safeSetWriteDeadline(time.Time{}, s.Logger); err != nil {
								s.Logger.Warn("Failed to reset write deadline on reconnect", "error", err)
							}

							oldConn := safeConn.setConn(newConn)
							if oldConn != nil {
								if err := oldConn.Close(); err != nil {
									s.Logger.Warn("Failed to close old connection during reconnect", "error", err)
								}
							}
							break reconnectLoop
						}
					}

					continue
				}

				if err := safeConn.safeSetReadDeadline(time.Time{}, s.Logger); err != nil {
					s.Logger.Warn(
						"Failed to reset read deadline after successful read",
						"error",
						err,
					)
				}

				if response.Type == "data" {
					if response.Payload.Data.TelegramMessageAdded.Text == nil {
						exitErr = ErrSubscriptionNilTextMessage
						return
					}

					newMessage, err := s.transformWebSocketDataToMessage(
						ctx,
						response.Payload.Data.TelegramMessageAdded,
						chatUUID,
					)
					if err != nil {
						s.Logger.Error("Failed to transform WebSocket data to message", "error", err)
						continue
					}

					if newMessage == nil {
						s.Logger.Warn("Transformed message is nil, skipping")
						continue
					}

					s.Logger.Debug("Adding message to history", "messageCount", len(s.LastMessages)+1)
					s.LastMessages = append(s.LastMessages, *newMessage)

					telegramEnabled, _ := GetTelegramEnabled(ctx, configtable.New(s.Store.DB().DB))
					s.Logger.Debug("Checking telegram enabled status", "enabled", telegramEnabled)

					if telegramEnabled != "true" {
						configQueries := configtable.New(s.Store.DB().DB)
						err := configQueries.SetConfigValue(ctx, configtable.SetConfigValueParams{
							Key:   TelegramEnabled,
							Value: sql.NullString{String: "true", Valid: true},
						})
						if err != nil {
							s.Logger.Error("Error setting telegram enabled", "error", err)
						} else {
							s.Logger.Info("Telegram enabled successfully")
						}
					}

					s.Logger.Debug("Executing agent with message history", "messageCount", len(s.LastMessages))
					agentResponse, err := s.Execute(
						ctx,
						s.TransformToOpenAIMessages(s.LastMessages),
						newMessage.Text,
					)
					if err != nil {
						s.Logger.Error("Agent execution failed", "error", err)
						continue
					}

					s.Logger.Debug("Agent execution completed", "responseLength", len(agentResponse.Content))
					if agentResponse.Content != "" {
						_, err := PostMessage(
							ctx,
							chatUUID,
							agentResponse.Content,
							s.ChatServerUrl,
						)
						if err != nil {
							s.Logger.Error("Error with GraphQL mutation response", "error", err)
						} else {
							agentMessage := Message{
								MessageID: 0,
								From:      User{Username: "enchanted_twin_bot"},
								Chat:      newMessage.Chat,
								Date:      int(time.Now().Unix()),
								Text:      agentResponse.Content,
							}
							s.LastMessages = append(s.LastMessages, agentMessage)

							if len(s.LastMessages) > 10 {
								s.Logger.Debug("Trimming message history", "oldCount", len(s.LastMessages), "newCount", 10)
								s.LastMessages = s.LastMessages[len(s.LastMessages)-10:]
							}
						}
					} else {
						s.Logger.Debug("Agent response is empty, not sending")
					}
				} else if response.Type == "connection_ack" {
					s.Logger.Debug("Received connection acknowledgment")
					connectionAcknowledged = true
				} else if response.Type == "ka" {
				} else if response.Type == "error" {
					s.Logger.Error("Received error message", "errors", response.Payload.Errors)
					if !connectionAcknowledged {
						s.Logger.Error("Received error before connection acknowledgment")
						exitErr = fmt.Errorf("received error before connection ack: %v", response.Payload.Errors)
						if err := safeConn.safeClose(s.Logger); err != nil {
							s.Logger.Warn("Failed to close connection after error before ack", "error", err)
						}
						return
					}
				} else {
					if s.unhandledLogLimiter.shouldLog() {
						s.Logger.Info("Received message of unhandled type", "type", response.Type, "response", response)
					}
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

func GetChatURL(botName string, chatUUID string) string {
	return fmt.Sprintf("https://t.me/%s?start=%s", botName, chatUUID)
}

func PostMessage(
	ctx context.Context,
	chatUUID string,
	message string,
	chatServerUrl string,
) (interface{}, error) {
	// Note: We can't use the logger here as it's not passed to this function
	// Consider refactoring to accept a logger parameter in the future

	client := &http.Client{}
	mutationPayload := map[string]interface{}{
		"query": `
			mutation SendTelegramMessage($chatUUID: ID!, $text: String!) {
				sendTelegramMessage(chatUUID: $chatUUID, text: $text)
			}
		`,
		"variables": map[string]interface{}{
			"chatUUID": chatUUID,
			"text":     message,
		},
		"operationName": "SendTelegramMessage",
	}

	mutationBody, err := json.Marshal(mutationPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal GraphQL mutation payload: %v", err)
	}

	gqlURL := chatServerUrl
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		gqlURL,
		bytes.NewBuffer(mutationBody),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create GraphQL request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send GraphQL mutation request: %v", err)
	}
	if resp == nil {
		return nil, fmt.Errorf("received nil response from GraphQL request")
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %v", err)
		}
		return nil, fmt.Errorf(
			"GraphQL mutation request failed: status %v, body: %v",
			resp.StatusCode,
			string(bodyBytes),
		)
	}

	var gqlResponse struct {
		Data   interface{} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	err = json.NewDecoder(resp.Body).Decode(&gqlResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to decode GraphQL mutation response: %v", err)
	}

	if len(gqlResponse.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL mutation returned errors: %v", gqlResponse.Errors)
	}

	return gqlResponse, nil
}

func GetTelegramEnabled(ctx context.Context, store *configtable.Queries) (string, error) {
	telegramEnabled, err := store.GetConfigValue(ctx, TelegramEnabled)
	if err != nil || !telegramEnabled.Valid || telegramEnabled.String != "true" {
		return "", fmt.Errorf("error getting telegram enabled: %w", err)
	}
	return telegramEnabled.String, nil
}

func MonitorAndRegisterTelegramTool(ctx context.Context, telegramService *TelegramService, logger *log.Logger, toolRegistry *tools.ToolMapRegistry, store *configtable.Queries, envs *config.Config) {
	logger.Info("Starting Telegram tool monitor and registration")

	keys, err := store.GetAllConfigKeys(context.Background())
	if err != nil {
		logger.Error("Error getting all config keys", "error", err)
		return
	}

	logger.Debug("Retrieved config keys", "count", len(keys))

	if !slices.Contains(keys, "telegram_chat_id") {
		logger.Info("Setting up initial Telegram configuration")

		err = store.SetConfigValue(context.Background(), configtable.SetConfigValueParams{
			Key: "telegram_chat_id",
		})
		if err != nil {
			logger.Error("Error setting telegram chat id", "error", err)
			return
		}

		chatUUID := uuid.New().String()
		logger.Info("Generated new chat UUID", "chatUUID", chatUUID)

		err = store.SetConfigValue(context.Background(), configtable.SetConfigValueParams{
			Key:   TelegramChatUUIDKey,
			Value: sql.NullString{String: chatUUID, Valid: true},
		})
		if err != nil {
			logger.Error("Error setting telegram chat uuid", "error", err)
			return
		}

		logger.Info("Initial Telegram configuration completed")
	} else {
		logger.Debug("Telegram configuration already exists")
	}

	monitorCount := 0
	for {
		monitorCount++

		telegramEnabled, errTelegramEnabled := GetTelegramEnabled(context.Background(), store)
		_, exists := toolRegistry.Get("telegram_send_message")

		if errTelegramEnabled == nil && telegramEnabled == "true" && !exists {
			logger.Info("Telegram is enabled but tool not registered, creating tool")

			telegramTool, err := NewTelegramSendMessageTool(logger, store, envs.TelegramChatServer, envs.TelegramBotName)
			if err == nil {
				logger.Debug("Telegram tool created successfully, registering")
				err = toolRegistry.Register(telegramTool)
				if err == nil {
					logger.Info("Telegram send message tool registered successfully")
					return
				} else {
					logger.Error("Failed to register telegram tool", "error", err)
				}
			} else {
				logger.Error("Error creating telegram send message tool", "error", err)
			}
		} else if exists {
			logger.Debug("Telegram tool already exists, monitoring complete")
			return
		}

		time.Sleep(2 * time.Second)
	}
}

func SubscribePoller(telegramService *TelegramService, logger *log.Logger) {
	appCtx, appCancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer appCancel()

	subscriptionCount := 0
	retryDelay := time.Second
	maxRetryDelay := 10 * time.Minute

	for {
		chatUUID, err := telegramService.GetChatUUID(context.Background())
		if err != nil {
			logger.Debug("No chat UUID found, skipping subscription attempt", "error", err)

			select {
			case <-time.After(retryDelay):
				continue
			case <-appCtx.Done():
				logger.Info("Stopping Telegram subscription poller due to application shutdown signal")
				return
			}
		}

		logger.Info("Starting subscription attempt", "chatUUID", chatUUID, "attempt", subscriptionCount+1, "retryDelay", retryDelay)
		subscriptionCount++

		err = telegramService.Subscribe(appCtx, chatUUID)

		if err == nil {
			logger.Info("Subscription ended normally")
			retryDelay = time.Second
		} else if errors.Is(err, ErrSubscriptionNilTextMessage) {
			retryDelay = time.Duration(math.Min(float64(retryDelay*2), float64(maxRetryDelay)))
		} else if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			if appCtx.Err() != nil {
				logger.Info("Subscription stopped due to application shutdown")
				return
			}
			logger.Info("Subscription stopped due to context cancellation, will retry", "error", err, "nextRetryIn", retryDelay)
			retryDelay = time.Duration(math.Min(float64(retryDelay*2), float64(maxRetryDelay)))
		} else {
			logger.Error("Subscription failed with unexpected error, will retry", "error", err, "nextRetryIn", retryDelay)
			retryDelay = time.Duration(math.Min(float64(retryDelay*2), float64(maxRetryDelay)))
		}

		select {
		case <-time.After(retryDelay):
			continue
		case <-appCtx.Done():
			logger.Info("Stopping Telegram subscription poller due to application shutdown signal")
			return
		}
	}
}

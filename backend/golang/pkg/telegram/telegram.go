package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/agent"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	types "github.com/EternisAI/enchanted-twin/types"
	"github.com/charmbracelet/log"
	"github.com/gorilla/websocket"
	"github.com/nats-io/nats.go"
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
	ID        string `json:"id"`
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
						chatID := fmt.Sprintf("%d", update.Message.Chat.ID)
						_, err := s.CreateChat(ctx, chatID, uuid)
						if err != nil {
							s.Logger.Error("Failed to create chat", "error", err)
							continue
						}

					}

					s.Logger.Info("Chat ID", "chat_id", update.Message.Chat)

					// Publish message to NATS
					if s.NatsClient != nil {
						subject := fmt.Sprintf("telegram.chat.%d", update.Message.Chat.ID)
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

func (s *TelegramService) CreateChat(ctx context.Context, chatID string, chatUUID string) (string, error) {
	fmt.Println("chatID", chatID)
	fmt.Println("chatUUID", chatUUID)
	err := s.Store.SetValue(ctx, fmt.Sprintf("telegram_chat_id_%s", chatUUID), chatID)
	if err != nil {
		return "", err
	}
	return chatID, nil
}

func (s *TelegramService) GetChatID(ctx context.Context) (string, error) {
	chatUUID, err := s.Store.GetValue(ctx, types.TelegramChatUUIDKey)
	if err != nil {
		return "", err
	}
	chatID, err := s.GetChatIDFromChatUUID(ctx, chatUUID)
	if err != nil {
		return "", err
	}
	return chatID, nil
}

func (s *TelegramService) GetChatIDFromChatUUID(ctx context.Context, chatUUID string) (string, error) {
	chatID, err := s.Store.GetValue(ctx, fmt.Sprintf("telegram_chat_id_%s", chatUUID))
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
	// Get the last update ID from storage
	lastUpdateID, err := s.Store.GetValue(ctx, types.TelegramLastUpdateIDKey)
	if err != nil {
		// If no last update ID exists, start from 0
		lastUpdateID = "0"
	}

	lastUpdateIDInt, err := strconv.Atoi(lastUpdateID)
	if err != nil {
		return nil, fmt.Errorf("failed to convert last update ID to int: %w", err)
	}

	url := fmt.Sprintf("%s/bot%s/getUpdates?offset=%d&limit=10", types.TelegramAPIBase, s.Token, lastUpdateIDInt)

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

	// Convert Updates to Messages and update the last update ID
	messages := make([]Message, 0, len(result.Result))
	for _, update := range result.Result {
		messages = append(messages, update.Message)
		// Update the last update ID
		if update.UpdateID > lastUpdateIDInt {
			lastUpdateIDInt = update.UpdateID
		}
	}

	// Store the new last update ID
	err = s.Store.SetValue(ctx, types.TelegramLastUpdateIDKey, strconv.Itoa(lastUpdateIDInt))
	if err != nil {
		s.Logger.Error("Failed to store last update ID", "error", err)
	}

	return messages, nil
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

func (s *TelegramService) SendMessage(ctx context.Context, chatID string, message string) error {
	chatIDInt, err := strconv.Atoi(chatID)
	if err != nil {
		return fmt.Errorf("invalid chat ID format: %w", err)
	}

	url := fmt.Sprintf("%s/bot%s/sendMessage", types.TelegramAPIBase, s.Token)
	body := map[string]any{
		"chat_id":    chatIDInt,
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

// transformWebSocketDataToMessage converts the data received from the WebSocket
// subscription into a standard Message struct.
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
		s.Logger.Warn("Failed to convert WebSocket message ID (GraphQL ID) to int. Using 0.", "error", err, "graphQL_ID", data.ID)
		messageID = 0
	}

	fromUser := User{
		Username: data.Role,
	}

	chatInfo := Chat{
		ID: chatUUID,
	}

	parsedTime, err := time.Parse(time.RFC3339, data.CreatedAt)
	if err != nil {
		s.Logger.Error("Failed to parse CreatedAt timestamp from WebSocket", "error", err, "timestamp", data.CreatedAt)
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
	if s == nil {
		return fmt.Errorf("telegram service is nil")
	}

	if s.Logger == nil {
		return fmt.Errorf("logger is nil")
	}

	s.Logger.Info("Starting WebSocket subscription", "chatUUID", chatUUID)

	wsURL := strings.Replace(s.ChatServerUrl, "http", "ws", 1)
	s.Logger.Info("Attempting to connect to WebSocket", "url", wsURL)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket (%s): %w", wsURL, err)
	}
	defer conn.Close()

	s.Logger.Info("WebSocket connection established")

	initMsg := map[string]interface{}{
		"type": "connection_init",
		"payload": map[string]interface{}{
			"headers": map[string]string{},
		},
	}

	s.Logger.Info("Sending connection initialization message")
	if err := conn.WriteJSON(initMsg); err != nil {
		return fmt.Errorf("failed to send connection initialization: %w", err)
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
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

	s.Logger.Info("Sending subscription request", "chatUUID", chatUUID, "subscription", subscription)
	if err := conn.WriteJSON(subscription); err != nil {
		return fmt.Errorf("failed to send subscription request: %w", err)
	}
	s.Logger.Info("Subscription request sent successfully")

	conn.SetReadDeadline(time.Time{})
	conn.SetWriteDeadline(time.Time{})

	// Handle incoming messages with reconnection logic
	go func() {
		reconnectDelay := time.Second
		maxReconnectDelay := time.Minute
		reconnectAttempts := 0
		maxReconnectAttempts := 5
		lastSuccessfulConnection := time.Now()

		connectionAcknowledged := true

		for {
			select {
			case <-ctx.Done():
				s.Logger.Info("Context cancelled, stopping WebSocket subscription")
				return
			default:
				var response struct {
					Type    string `json:"type"`
					ID      string `json:"id"`
					Payload struct {
						Data struct {
							TelegramMessageAdded struct { // Renamed nested struct for clarity
								ID        string
								Text      *string
								Role      string
								CreatedAt string
							} `json:"telegramMessageAdded"` // Match GraphQL response field
						} `json:"data"`
						Errors []struct {
							Message string `json:"message"`
						} `json:"errors"`
					} `json:"payload"`
				}

				if err := conn.ReadJSON(&response); err != nil {
					s.Logger.Error("!!! conn.ReadJSON returned error !!!", "error", err)

					if time.Since(lastSuccessfulConnection) > time.Minute {
						reconnectAttempts = 0
					}

					if reconnectAttempts >= maxReconnectAttempts {
						s.Logger.Error("Max reconnection attempts reached, stopping subscription")
						return
					}

					for {
						select {
						case <-ctx.Done():
							s.Logger.Info("Context cancelled during reconnection attempt")
							return
						default:
							// Exponential backoff
							time.Sleep(reconnectDelay)
							reconnectDelay = time.Duration(math.Min(float64(reconnectDelay*2), float64(maxReconnectDelay)))

							s.Logger.Info("Attempting to reconnect", "attempt", reconnectAttempts+1, "delay", reconnectDelay)
							// Try to reconnect
							newConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
							if err != nil {
								s.Logger.Error("Failed to reconnect", "error", err, "attempt", reconnectAttempts+1)
								reconnectAttempts++
								continue
							}

							// Reset reconnect delay and attempts on successful connection
							reconnectDelay = time.Second
							reconnectAttempts = 0
							lastSuccessfulConnection = time.Now()
							connectionAcknowledged = false

							// Send connection initialization message
							s.Logger.Info("Sending connection initialization message on reconnect")
							if err := newConn.WriteJSON(initMsg); err != nil {
								s.Logger.Error("Failed to send connection initialization on reconnect", "error", err)
								newConn.Close()
								reconnectAttempts++
								continue
							}

							// Wait for connection acknowledgment
							newConn.SetReadDeadline(time.Now().Add(5 * time.Second))
							if err := newConn.ReadJSON(&ackResponse); err != nil {
								s.Logger.Error("Failed to read connection acknowledgment on reconnect", "error", err)
								newConn.Close()
								reconnectAttempts++
								continue
							}

							if ackResponse.Type != "connection_ack" {
								s.Logger.Error("Unexpected response type on reconnect", "type", ackResponse.Type)
								newConn.Close()
								reconnectAttempts++
								continue
							}

							// Send subscription request again
							s.Logger.Info("Sending subscription request on reconnect", "chatUUID", chatUUID)
							if err := newConn.WriteJSON(subscription); err != nil {
								s.Logger.Error("Failed to resend subscription", "error", err)
								newConn.Close()
								reconnectAttempts++
								continue
							}

							// Reset deadlines for the main message loop
							newConn.SetReadDeadline(time.Time{})
							newConn.SetWriteDeadline(time.Time{})

							// Update connection
							conn = newConn
							connectionAcknowledged = true
							s.Logger.Info("Successfully reconnected to WebSocket", "attempt", reconnectAttempts)
							break
						}
					}

				}

				if response.Type == "data" {
					// Check if the text is nil before attempting transformation
					if response.Payload.Data.TelegramMessageAdded.Text == nil {
						s.Logger.Warn("Received WebSocket data message with nil text, skipping.", "message_id", response.Payload.Data.TelegramMessageAdded.ID)
						continue
					}

					// Use the new transformation function
					newMessage, err := s.transformWebSocketDataToMessage(ctx, response.Payload.Data.TelegramMessageAdded, chatUUID)
					if err != nil {
						s.Logger.Error("Failed to transform WebSocket data to message", "error", err)
						continue // Skip processing this message
					}

					s.Logger.Info("Received and transformed message", "message", newMessage.Text)

					// Append the properly typed message to LastMessages
					s.LastMessages = append(s.LastMessages, *newMessage) // Dereference pointer

					// Execute the message
					agentResponse, err := s.Execute(ctx, s.TransformToOpenAIMessages(s.LastMessages), newMessage.Text)
					if err != nil {
						s.Logger.Error("Failed to execute message", "error", err)
						// Should we remove the last message if execution fails? Depends on desired behavior.
						continue
					}
					s.Logger.Info("Agent response", "content", agentResponse.Content, "tool_calls", len(agentResponse.ToolCalls), "tool_results", len(agentResponse.ToolResults))

					// Send the agent's response back via GraphQL mutation
					if agentResponse.Content != "" {
						mutationPayload := map[string]interface{}{
							"query": `
								mutation SendTelegramMessage($chatUUID: ID!, $text: String!) {
									sendTelegramMessage(chatUUID: $chatUUID, text: $text)
								}
							`,
							"variables": map[string]interface{}{
								"chatUUID": chatUUID,
								"text":     agentResponse.Content,
							},
							"operationName": "SendTelegramMessage",
						}
						mutationBody, err := json.Marshal(mutationPayload)
						if err != nil {
							s.Logger.Error("Failed to marshal GraphQL mutation payload", "error", err)
							continue // Or handle error differently
						}

						gqlURL := s.ChatServerUrl
						req, err := http.NewRequestWithContext(ctx, http.MethodPost, gqlURL, bytes.NewBuffer(mutationBody))
						if err != nil {
							s.Logger.Error("Failed to create GraphQL request", "error", err)
							continue
						}
						req.Header.Set("Content-Type", "application/json")

						resp, err := s.Client.Do(req)
						if err != nil {
							s.Logger.Error("Failed to send GraphQL mutation request", "error", err)
							continue
						}
						defer resp.Body.Close()

						if resp.StatusCode != http.StatusOK {
							bodyBytes, _ := io.ReadAll(resp.Body)
							s.Logger.Error("GraphQL mutation request failed", "status_code", resp.StatusCode, "response_body", string(bodyBytes))
							continue
						}

						// Optionally decode and check the GraphQL response body for errors
						var gqlResponse struct {
							Data   interface{} `json:"data"`
							Errors []struct {
								Message string `json:"message"`
							} `json:"errors"`
						}
						if err := json.NewDecoder(resp.Body).Decode(&gqlResponse); err != nil {
							s.Logger.Error("Failed to decode GraphQL mutation response", "error", err)
							// Continue even if decoding fails? Or handle more strictly?
						} else if len(gqlResponse.Errors) > 0 {
							s.Logger.Error("GraphQL mutation returned errors", "errors", gqlResponse.Errors)
							// Handle GraphQL-level errors
						} else {
							s.Logger.Info("Successfully sent agent response via GraphQL mutation")
							// Add the agent's response to the history
							agentMessage := Message{
								MessageID: 0,                                    // Placeholder ID for agent response
								From:      User{Username: "enchanted_twin_bot"}, // TODO: Use a more definitive bot identifier if possible
								Chat:      newMessage.Chat,                      // Use the same chat context
								Date:      int(time.Now().Unix()),
								Text:      agentResponse.Content,
							}
							s.LastMessages = append(s.LastMessages, agentMessage)

							// Optional: Limit history size (e.g., keep last 10 messages)
							if len(s.LastMessages) > 10 {
								s.LastMessages = s.LastMessages[len(s.LastMessages)-10:]
							}
						}
					}

				} else if response.Type == "connection_ack" {
					s.Logger.Info("Received connection acknowledgment")
					connectionAcknowledged = true
				} else if response.Type == "ka" {
					// Keep-alive message, no need to log
				} else if response.Type == "error" {
					s.Logger.Error("Received error from server", "errors", response.Payload.Errors)
					// If we get an error and haven't received a connection_ack, try to reconnect
					if !connectionAcknowledged {
						s.Logger.Warn("No connection acknowledgment received before error, attempting to reconnect")
						conn.Close() // Close the connection to trigger reconnection logic below
						// No explicit 'continue' here, let the ReadJSON error handle reconnection attempt
					}
				} else {
					s.Logger.Info("Received message of type", "type", response.Type, "response", response)
				}
			}
		}
	}()

	s.Logger.Info("Reader goroutine started. Subscribe function will block until context is done.")
	<-ctx.Done() // Block here until the context is cancelled
	s.Logger.Info("Subscribe context finished. Function will return, closing connection via defer.")
	return ctx.Err() // Return context error (e.g., context.Canceled) or nil
}

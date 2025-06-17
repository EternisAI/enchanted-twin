package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/log"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/processor"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
	"github.com/EternisAI/enchanted-twin/pkg/db"
)

type Contact struct {
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	PhoneNumber  string `json:"phone_number"`
	Date         string `json:"date"`
	DateUnixtime string `json:"date_unixtime"`
}

type TextEntity struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type Message struct {
	ID            int          `json:"id"`
	Type          string       `json:"type"`
	Date          string       `json:"date"`
	DateUnixtime  string       `json:"date_unixtime"`
	From          string       `json:"from"`
	FromID        string       `json:"from_id"`
	ForwardedFrom string       `json:"forwarded_from,omitempty"`
	SavedFrom     string       `json:"saved_from,omitempty"`
	Text          interface{}  `json:"text"`
	TextEntities  []TextEntity `json:"text_entities"`
}

type Chat struct {
	Type     string    `json:"type"`
	ID       int       `json:"id"`
	Messages []Message `json:"messages"`
	Name     string    `json:"name"`
}

type PersonalInformation struct {
	UserID      int    `json:"user_id"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	PhoneNumber string `json:"phone_number"`
	Username    string `json:"username"`
	Bio         string `json:"bio"`
}

type TelegramData struct {
	PersonalInformation PersonalInformation `json:"personal_information"`
	Contacts            struct {
		About string    `json:"about"`
		List  []Contact `json:"list"`
	} `json:"contacts"`
	Chats struct {
		About string `json:"about"`
		List  []Chat `json:"list"`
	} `json:"chats"`
}

type TelegramProcessor struct {
	store  *db.Store
	logger *log.Logger
}

type conversationData struct {
	chatId       string
	chatType     string
	chatName     string
	messages     []messageData
	people       map[string]bool
	firstMessage time.Time
	lastMessage  time.Time
}

type messageData struct {
	ID            int       `json:"id"`
	MessageType   string    `json:"messageType"`
	From          string    `json:"from"`
	To            string    `json:"to"`
	Text          string    `json:"text"`
	Timestamp     time.Time `json:"timestamp"`
	ForwardedFrom string    `json:"forwardedFrom"`
	SavedFrom     string    `json:"savedFrom"`
	MyMessage     bool      `json:"myMessage"`
}

func NewTelegramProcessor(store *db.Store, logger *log.Logger) processor.Processor {
	return &TelegramProcessor{store: store, logger: logger}
}

func (s *TelegramProcessor) Name() string {
	return "telegram"
}

func (s *TelegramProcessor) extractUsername(ctx context.Context, telegramData TelegramData) (string, error) {
	if s.store == nil {
		return "", fmt.Errorf("store is nil")
	}

	extractedUsername := ""
	if telegramData.PersonalInformation.Username != "" {
		userIDStr := ""
		if telegramData.PersonalInformation.UserID != 0 {
			userIDStr = strconv.Itoa(telegramData.PersonalInformation.UserID)
		}

		sourceUsername := db.SourceUsername{
			Source:   s.Name(),
			Username: telegramData.PersonalInformation.Username,
		}

		if userIDStr != "" {
			sourceUsername.UserID = &userIDStr
		}
		if telegramData.PersonalInformation.FirstName != "" {
			sourceUsername.FirstName = &telegramData.PersonalInformation.FirstName
		}
		if telegramData.PersonalInformation.LastName != "" {
			sourceUsername.LastName = &telegramData.PersonalInformation.LastName
		}
		if telegramData.PersonalInformation.PhoneNumber != "" {
			sourceUsername.PhoneNumber = &telegramData.PersonalInformation.PhoneNumber
		}
		if telegramData.PersonalInformation.Bio != "" {
			sourceUsername.Bio = &telegramData.PersonalInformation.Bio
		}

		if err := s.store.SetSourceUsername(ctx, sourceUsername); err != nil {
			s.logger.Warn("Failed to save username to database", "error", err)

			return "", err
		}

		extractedUsername = strings.TrimPrefix(telegramData.PersonalInformation.Username, "@")
	}

	s.logger.Info(" extractedUsername", "extractedUsername", extractedUsername)

	return extractedUsername, nil
}

func (s *TelegramProcessor) ProcessDirectory(ctx context.Context, filepath string) ([]types.Record, error) {
	return nil, fmt.Errorf("process directory not supported for Telegram")
}

func (s *TelegramProcessor) Sync(ctx context.Context, accessToken string) ([]types.Record, bool, error) {
	return nil, false, fmt.Errorf("sync operation not supported for Telegram")
}

func (s *TelegramProcessor) ProcessFile(ctx context.Context, filepath string) ([]types.Record, error) {
	if s.store == nil {
		return nil, fmt.Errorf("store is nil")
	}

	fileInfo, err := os.Stat(filepath)
	if err != nil {
		return nil, err
	}

	var jsonFilePath string
	if fileInfo.IsDir() {
		entries, err := os.ReadDir(filepath)
		if err != nil {
			return nil, fmt.Errorf("error reading directory %s: %v", filepath, err)
		}

		for _, entry := range entries {
			if !entry.IsDir() && entry.Name() == "result.json" {
				jsonFilePath = fmt.Sprintf("%s/result.json", filepath)
				break
			}
		}

		if jsonFilePath == "" {
			var candidates []string
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
					candidates = append(candidates, entry.Name())
				}
			}

			if len(candidates) == 0 {
				return nil, fmt.Errorf("no JSON files found in directory %s", filepath)
			}

			if len(candidates) > 1 {
				return nil, fmt.Errorf("multiple JSON files found in directory %s, but no result.json file. Please specify the exact file path or ensure result.json exists. Found files: %v", filepath, candidates)
			}

			jsonFilePath = fmt.Sprintf("%s/%s", filepath, candidates[0])
		}
	} else {
		jsonFilePath = filepath
	}

	jsonData, err := os.ReadFile(jsonFilePath)
	if err != nil {
		return nil, err
	}

	var telegramData TelegramData
	if err := json.Unmarshal(jsonData, &telegramData); err != nil {
		return nil, err
	}

	effectiveUserName, err := s.extractUsername(ctx, telegramData)
	if err != nil {
		return nil, err
	}

	estimatedRecords := len(telegramData.Contacts.List) + len(telegramData.Chats.List)
	records := make([]types.Record, 0, estimatedRecords)
	conversationMap := make(map[string]*conversationData)

	for _, contact := range telegramData.Contacts.List {
		timestamp, err := parseTimestamp(contact.Date, contact.DateUnixtime)
		if err != nil {
			s.logger.Warn("Failed to parse contact timestamp", "error", err)
			continue
		}

		contactData := map[string]interface{}{
			"type":        "contact",
			"firstName":   contact.FirstName,
			"lastName":    contact.LastName,
			"phoneNumber": contact.PhoneNumber,
		}

		record := types.Record{
			Data:      contactData,
			Timestamp: timestamp,
			Source:    s.Name(),
		}

		records = append(records, record)
	}

	totalChats := len(telegramData.Chats.List)
	s.logger.Info("Processing chats", "totalChats", totalChats)

	processedChats := 0
	totalMessages := 0
	processedMessages := 0

	for _, chat := range telegramData.Chats.List {
		totalMessages += len(chat.Messages)
	}
	s.logger.Info("Total messages to process", "count", totalMessages)

	const batchSize = 1000
	const progressInterval = 10

	for chatIndex, chat := range telegramData.Chats.List {
		if chat.Type == "private_supergroup" {
			s.logger.Info("Skipping private_supergroup chat", "chatId", chat.ID, "chatName", chat.Name)
			processedChats++
			continue
		}

		chatId := strconv.Itoa(chat.ID)
		chatMessageCount := len(chat.Messages)

		if chatMessageCount == 0 {
			processedChats++
			continue
		}

		s.logger.Debug("Processing chat",
			"chatId", chat.ID,
			"chatName", chat.Name,
			"messageCount", chatMessageCount,
			"progress", fmt.Sprintf("%d/%d", chatIndex+1, totalChats))

		conv := &conversationData{
			chatId:       chatId,
			chatType:     chat.Type,
			chatName:     chat.Name,
			messages:     make([]messageData, 0, chatMessageCount),
			people:       make(map[string]bool),
			firstMessage: time.Now(),
			lastMessage:  time.Time{},
		}

		for i := 0; i < len(chat.Messages); i += batchSize {
			end := i + batchSize
			if end > len(chat.Messages) {
				end = len(chat.Messages)
			}

			for j := i; j < end; j++ {
				message := chat.Messages[j]

				timestamp, err := parseTimestamp(message.Date, message.DateUnixtime)
				if err != nil {
					s.logger.Warn("Failed to parse message timestamp", "error", err, "messageId", message.ID)
					processedMessages++
					continue
				}

				var fullText string
				if len(message.TextEntities) > 0 {
					var textBuilder strings.Builder
					for _, entity := range message.TextEntities {
						textBuilder.WriteString(entity.Text)
					}
					fullText = textBuilder.String()
				} else {
					switch textValue := message.Text.(type) {
					case string:
						fullText = textValue
					case []interface{}:
						var textBuilder strings.Builder
						for _, item := range textValue {
							if textEntity, ok := item.(map[string]interface{}); ok {
								if text, ok := textEntity["text"].(string); ok {
									textBuilder.WriteString(text)
								}
							}
						}
						fullText = textBuilder.String()
					}
				}

				if fullText == "" {
					processedMessages++
					continue
				}

				// Identify user messages by from_id matching user_id
				expectedFromID := fmt.Sprintf("user%d", telegramData.PersonalInformation.UserID)
				myMessage := message.FromID == expectedFromID

				to := ""
				if myMessage {
					to = chat.Name
				} else {
					to = effectiveUserName
				}

				msg := messageData{
					ID:            message.ID,
					MessageType:   message.Type,
					From:          message.From,
					To:            to,
					Text:          fullText,
					Timestamp:     timestamp,
					ForwardedFrom: message.ForwardedFrom,
					SavedFrom:     message.SavedFrom,
					MyMessage:     myMessage,
				}

				conv.messages = append(conv.messages, msg)
				// Add all speakers to people list (including the user)
				conv.people[message.From] = true

				if len(conv.messages) == 1 || timestamp.Before(conv.firstMessage) {
					conv.firstMessage = timestamp
				}
				if timestamp.After(conv.lastMessage) {
					conv.lastMessage = timestamp
				}

				processedMessages++
			}

			if totalMessages > 10000 && (processedMessages%5000 == 0 || processedMessages == totalMessages) {
				s.logger.Info("Message processing progress",
					"processed", processedMessages,
					"total", totalMessages,
					"percentage", fmt.Sprintf("%.1f%%", float64(processedMessages)/float64(totalMessages)*100))
			}
		}

		if len(conv.messages) > 0 {
			conversationMap[chatId] = conv
		}

		processedChats++

		if totalChats > 50 && (processedChats%progressInterval == 0 || processedChats == totalChats) {
			s.logger.Info("Chat processing progress",
				"processed", processedChats,
				"total", totalChats,
				"percentage", fmt.Sprintf("%.1f%%", float64(processedChats)/float64(totalChats)*100))
		}
	}

	s.logger.Info("Completed message processing",
		"totalChats", processedChats,
		"totalMessages", processedMessages,
		"conversations", len(conversationMap))

	// Use first name as user display name
	userDisplayName := telegramData.PersonalInformation.FirstName

	for _, conv := range conversationMap {
		var people []string
		for person := range conv.people {
			if person != "" {
				people = append(people, person)
			}
		}

		// Skip conversations where the user is the only participant
		if len(people) == 1 && people[0] == userDisplayName {
			s.logger.Debug("Skipping conversation with only user messages", "chatId", conv.chatId, "chatName", conv.chatName)
			continue
		}

		// Skip system conversations with Telegram
		if slices.Contains(people, "Telegram") {
			s.logger.Debug("Skipping system conversation with Telegram", "chatId", conv.chatId, "chatName", conv.chatName)
			continue
		}

		conversationDataMap := map[string]interface{}{
			"type":     "conversation",
			"chatId":   conv.chatId,
			"chatType": conv.chatType,
			"chatName": conv.chatName,
			"messages": conv.messages,
			"people":   people,
			"user":     userDisplayName,
		}

		record := types.Record{
			Data:      conversationDataMap,
			Timestamp: conv.firstMessage,
			Source:    s.Name(),
		}

		records = append(records, record)
	}

	s.logger.Info("Processing completed", "totalRecords", len(records))
	return records, nil
}

func parseTimestamp(dateStr, unixStr string) (time.Time, error) {
	formats := []string{
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05.000",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02T15:04:05.000-07:00",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	if unixStr != "" {
		if unixSec, err := strconv.ParseInt(unixStr, 10, 64); err == nil {
			return time.Unix(unixSec, 0), nil
		}
	}

	return time.Time{}, fmt.Errorf("failed to parse timestamp: %s", dateStr)
}

func (s *TelegramProcessor) ToDocuments(ctx context.Context, records []types.Record) ([]memory.Document, error) {
	var documents []memory.Document

	for _, record := range records {
		if record.Data["type"] == "conversation" {
			chatIdInterface, ok := record.Data["chatId"]
			if !ok {
				continue
			}

			var chatId string
			switch v := chatIdInterface.(type) {
			case int:
				chatId = fmt.Sprintf("%d", v)
			case float64:
				chatId = fmt.Sprintf("%.0f", v)
			case string:
				chatId = v
			default:
				continue
			}

			messagesInterface, ok := record.Data["messages"]
			if !ok {
				continue
			}

			people, ok := record.Data["people"].([]string)
			if !ok {
				if peopleInterface, ok := record.Data["people"].([]interface{}); ok {
					people = make([]string, len(peopleInterface))
					for i, p := range peopleInterface {
						if str, ok := p.(string); ok {
							people[i] = str
						}
					}
				}
			}

			// Extract user from the record data (which now contains the display name)
			userFromRecord, _ := record.Data["user"].(string)

			conversationDoc := &memory.ConversationDocument{
				FieldID:      chatId,
				FieldSource:  "telegram",
				FieldTags:    []string{"social", "chat"},
				People:       people,
				User:         userFromRecord,
				Conversation: []memory.ConversationMessage{},
				FieldMetadata: map[string]string{
					"type":   "conversation",
					"source": "telegram",
				},
			}

			// Handle messages - they can be either []messageData (direct struct) or []interface{} (from JSON)
			if messagesSlice, ok := messagesInterface.([]messageData); ok {
				// Direct messageData structs (from ProcessFile)
				for _, msg := range messagesSlice {
					conversationDoc.Conversation = append(conversationDoc.Conversation, memory.ConversationMessage{
						Speaker: msg.From,
						Content: msg.Text,
						Time:    msg.Timestamp,
					})
				}
			} else if messagesSlice, ok := messagesInterface.([]interface{}); ok {
				// Interface slice (from JSON deserialization)
				for _, msgInterface := range messagesSlice {
					if msgMap, ok := msgInterface.(map[string]interface{}); ok {
						from, _ := msgMap["from"].(string)
						content, _ := msgMap["text"].(string)

						var timestamp time.Time
						if timestampInterface, ok := msgMap["timestamp"]; ok {
							switch v := timestampInterface.(type) {
							case time.Time:
								timestamp = v
							case string:
								if parsed, err := time.Parse(time.RFC3339, v); err == nil {
									timestamp = parsed
								}
							}
						}

						conversationDoc.Conversation = append(conversationDoc.Conversation, memory.ConversationMessage{
							Speaker: from,
							Content: content,
							Time:    timestamp,
						})
					}
				}
			}

			documents = append(documents, conversationDoc)
		}

		if record.Data["type"] == "contact" {
			firstName, ok := record.Data["firstName"].(string)
			if !ok {
				firstName = ""
			}
			lastName, ok := record.Data["lastName"].(string)
			if !ok {
				lastName = ""
			}
			phoneNumber, ok := record.Data["phoneNumber"].(string)
			if !ok {
				phoneNumber = ""
			}

			// Create more descriptive content that clearly indicates this is contact information
			fullName := strings.TrimSpace(firstName + " " + lastName)
			contactContent := fmt.Sprintf("CONTACT ENTRY: %s", fullName)
			if phoneNumber != "" {
				contactContent += fmt.Sprintf(" (Phone: %s)", phoneNumber)
			}
			contactContent += " - This is a contact from the user's Telegram contact list, not information about the primary user."

			// Generate a unique ID for the contact
			contactID := fmt.Sprintf("telegram-contact-%s", phoneNumber)
			if phoneNumber == "" {
				// Fallback to name-based ID if no phone number
				contactID = fmt.Sprintf("telegram-contact-%s", strings.ReplaceAll(fullName, " ", "-"))
			}

			textDoc := &memory.TextDocument{
				FieldID:        contactID,
				FieldSource:    "telegram",
				FieldContent:   contactContent,
				FieldTimestamp: &record.Timestamp,
				FieldTags:      []string{"social", "contact", "contact_list"},
				FieldMetadata: map[string]string{
					"type":                "contact",
					"document_type":       "contact_entry",
					"data_category":       "contact_list",
					"is_primary_user":     "false",
					"contact_source":      "telegram_contacts",
					"firstName":           firstName,
					"lastName":            lastName,
					"phoneNumber":         phoneNumber,
					"extraction_guidance": "This is contact list data - extract relationship facts only, never facts about primaryUser",
				},
			}
			documents = append(documents, textDoc)
		}
	}

	return documents, nil
}

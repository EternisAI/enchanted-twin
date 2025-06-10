package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
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
	Text          []TextEntity `json:"text_entities"`
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

func NewTelegramProcessor(store *db.Store, logger *log.Logger) (processor.Processor, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger is nil")
	}
	return &TelegramProcessor{store: store, logger: logger}, nil
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

		extractedUsername = telegramData.PersonalInformation.Username
	}

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

	var records []types.Record

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

	for _, chat := range telegramData.Chats.List {
		for _, message := range chat.Messages {
			timestamp, err := parseTimestamp(message.Date, message.DateUnixtime)
			if err != nil {
				s.logger.Warn("Failed to parse message timestamp", "error", err)
				continue
			}

			var fullText string
			for _, entity := range message.Text {
				fullText += entity.Text
			}

			var myMessage bool
			if effectiveUserName == "" {
				myMessage = false
			} else {
				normalizedEffectiveUserName := strings.TrimPrefix(effectiveUserName, "@")
				normalizedMessageFrom := strings.TrimPrefix(message.From, "@")
				myMessage = strings.EqualFold(normalizedMessageFrom, normalizedEffectiveUserName)
			}

			to := ""
			if myMessage {
				to = chat.Name
			} else {
				to = effectiveUserName
			}

			messageData := map[string]interface{}{
				"type":        "message",
				"messageId":   message.ID,
				"messageType": message.Type,
				"from":        message.From,
				"to":          to,
				"text":        fullText,
				"chatType":    chat.Type,
				"chatId":      chat.ID,
				"myMessage":   myMessage,
			}

			if message.ForwardedFrom != "" {
				messageData["forwardedFrom"] = message.ForwardedFrom
			}
			if message.SavedFrom != "" {
				messageData["savedFrom"] = message.SavedFrom
			}

			if len(message.Text) > 0 {
				record := types.Record{
					Data:      messageData,
					Timestamp: timestamp,
					Source:    s.Name(),
				}

				records = append(records, record)
			}
		}
	}

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
	conversationMap := make(map[string]*memory.ConversationDocument)
	var textDocuments []memory.TextDocument

	sourceUsername, err := s.store.GetSourceUsername(ctx, s.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to get source username: %v", err)
	}

	var extractedUser string
	if sourceUsername != nil {
		extractedUser = sourceUsername.Username
	}

	for _, record := range records {
		if record.Data["type"] == "message" {
			message, ok := record.Data["text"].(string)
			if !ok || message == "" {
				continue
			}
			from, ok := record.Data["from"].(string)
			if !ok || from == "" {
				continue
			}
			to, ok := record.Data["to"].(string)
			if !ok || to == "" {
				continue
			}

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

			conversation, exists := conversationMap[chatId]
			if !exists {
				conversationMap[chatId] = &memory.ConversationDocument{
					FieldID:      chatId,
					FieldSource:  "telegram",
					FieldTags:    []string{"social", "chat"},
					People:       []string{from, to},
					User:         extractedUser,
					Conversation: []memory.ConversationMessage{},
					FieldMetadata: map[string]string{
						"type":   "conversation",
						"source": "telegram",
					},
				}
				conversation = conversationMap[chatId]
			}

			conversation.Conversation = append(conversation.Conversation, memory.ConversationMessage{
				Speaker: from,
				Content: message,
				Time:    record.Timestamp,
			})

			peopleMap := make(map[string]bool)
			for _, person := range conversation.People {
				peopleMap[person] = true
			}
			if !peopleMap[from] {
				conversation.People = append(conversation.People, from)
			}
			if !peopleMap[to] {
				conversation.People = append(conversation.People, to)
			}
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
			textDocuments = append(textDocuments, memory.TextDocument{
				FieldSource:    "telegram",
				FieldContent:   firstName + " " + lastName,
				FieldTimestamp: &record.Timestamp,
				FieldTags:      []string{"social", "contact"},
				FieldMetadata: map[string]string{
					"type":        "contact",
					"firstName":   firstName,
					"lastName":    lastName,
					"phoneNumber": phoneNumber,
				},
			})
		}
	}

	var documents []memory.Document
	for _, conversation := range conversationMap {
		documents = append(documents, conversation)
	}

	for _, textDoc := range textDocuments {
		documents = append(documents, &textDoc)
	}

	return documents, nil
}

package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
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

type TelegramData struct {
	Contacts struct {
		About string    `json:"about"`
		List  []Contact `json:"list"`
	} `json:"contacts"`
	Chats struct {
		About string `json:"about"`
		List  []Chat `json:"list"`
	} `json:"chats"`
}

type Source struct{}

func New() *Source {
	return &Source{}
}

func (s *Source) Name() string {
	return "telegram"
}

// and falls back to unix timestamp if available.
func parseTimestamp(dateStr, unixStr string) (time.Time, error) {
	formats := []string{
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05.000",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02T15:04:05.000-07:00",
	}

	// Try parsing with various formats
	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	// If none of the formats work and we have a unix timestamp, use that
	if unixStr != "" {
		if unixSec, err := strconv.ParseInt(unixStr, 10, 64); err == nil {
			return time.Unix(unixSec, 0), nil
		}
	}

	return time.Time{}, fmt.Errorf("failed to parse timestamp: %s", dateStr)
}

func (s *Source) ProcessFile(filepath string, userName string) ([]types.Record, error) {
	jsonData, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	var telegramData TelegramData
	if err := json.Unmarshal(jsonData, &telegramData); err != nil {
		return nil, err
	}

	var records []types.Record

	for _, contact := range telegramData.Contacts.List {
		timestamp, err := parseTimestamp(contact.Date, contact.DateUnixtime)
		if err != nil {
			// Log the error instead of silently continuing
			fmt.Printf("Warning: Failed to parse contact timestamp: %v\n", err)
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
				// Log the error instead of silently continuing
				fmt.Printf("Warning: Failed to parse message timestamp: %v\n", err)
				continue
			}

			var fullText string
			for _, entity := range message.Text {
				fullText += entity.Text
			}

			myMessage := strings.EqualFold(message.From, userName)

			to := ""
			if myMessage {
				to = chat.Name
			} else {
				to = userName
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

func ToDocuments(records []types.Record) ([]memory.TextDocument, error) {
	textDocuments := []memory.TextDocument{}
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

			textDocuments = append(textDocuments, memory.TextDocument{
				FieldContent:   message,
				FieldTimestamp: &record.Timestamp,
				FieldTags:      []string{"social", "telegram", "chat"},
				FieldMetadata: map[string]string{
					"type":   "message",
					"from":   from,
					"to":     to,
					"source": "telegram",
				},
			})
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
				FieldContent:   firstName + " " + lastName,
				FieldTimestamp: &record.Timestamp,
				FieldTags:      []string{"social", "telegram", "contact"},
				FieldMetadata: map[string]string{
					"type":        "contact",
					"firstName":   firstName,
					"lastName":    lastName,
					"phoneNumber": phoneNumber,
				},
			})
		}
	}

	return textDocuments, nil
}

func (s *Source) Sync(ctx context.Context) ([]types.Record, error) {
	return nil, fmt.Errorf("sync operation not supported for Telegram")
}

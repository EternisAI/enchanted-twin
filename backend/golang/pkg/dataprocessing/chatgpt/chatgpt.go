package chatgpt

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
)

type ChatGPTConversation struct {
	Title      string                        `json:"title"`
	CreateTime float64                       `json:"create_time"`
	UpdateTime float64                       `json:"update_time"`
	Mapping    map[string]ChatGPTMessageNode `json:"mapping"`
}

type ChatGPTMessageNode struct {
	ID       string          `json:"id"`
	Message  *ChatGPTMessage `json:"message"` // Pointer because it can be null
	Parent   *string         `json:"parent"`  // Pointer because root node has null parent
	Children []string        `json:"children"`
}

type ChatGPTMessage struct {
	ID         string   `json:"id"`
	Author     Author   `json:"author"`
	CreateTime *float64 `json:"create_time"`
	Content    Content  `json:"content"`
	Metadata   Metadata `json:"metadata"`
	Recipient  string   `json:"recipient"`
}

type Author struct {
	Role     string                 `json:"role"`
	Name     interface{}            `json:"name"`
	Metadata map[string]interface{} `json:"metadata"`
}

type Content struct {
	ContentType        string        `json:"content_type"`
	Parts              []interface{} `json:"parts,omitempty"`
	Language           string        `json:"language,omitempty"`
	ResponseFormatName interface{}   `json:"response_format_name,omitempty"`
	Text               string        `json:"text,omitempty"`
}

type Metadata map[string]interface{}

type ChatGPTDataSource struct {
	inputPath string
}

type ConversationMessage struct {
	Role string
	Text string
}

func New(inputPath string) *ChatGPTDataSource {
	return &ChatGPTDataSource{
		inputPath: inputPath,
	}
}

func (s *ChatGPTDataSource) Name() string {
	return "chatgpt"
}

func (s *ChatGPTDataSource) ProcessFileConversations(
	filePath string,
	username string,
) ([]types.Record, error) {
	jsonData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var conversations []ChatGPTConversation
	if err := json.Unmarshal(jsonData, &conversations); err != nil {
		return nil, err
	}

	var records []types.Record

	for _, conversation := range conversations {
		timestamp, err := parseTimestamp(strconv.FormatFloat(conversation.CreateTime, 'f', -1, 64))
		if err != nil {
			continue
		}

		var messages []ConversationMessage

		for _, messageNode := range conversation.Mapping {
			if messageNode.Message == nil {
				continue
			}

			if messageNode.Message.Author.Role == "tool" {
				continue
			}

			content := messageNode.Message.Content
			if content.ContentType != "text" {
				continue
			}

			var textContentBuilder strings.Builder
			for _, part := range content.Parts {
				if strPart, ok := part.(string); ok {
					textContentBuilder.WriteString(strPart)
				}
			}
			extractedText := textContentBuilder.String()

			if extractedText == "" {
				continue
			}

			role := messageNode.Message.Author.Role
			messages = append(messages, ConversationMessage{
				Role: role,
				Text: extractedText,
			})
		}

		if len(messages) == 0 {
			continue
		}

		conversationData := map[string]any{
			"title":    conversation.Title,
			"messages": messages,
		}

		record := types.Record{
			Data:      conversationData,
			Timestamp: timestamp,
			Source:    s.Name(),
		}
		records = append(records, record)
	}

	return records, nil
}

func (s *ChatGPTDataSource) ProcessDirectory(userName string) ([]types.Record, error) {
	var allRecords []types.Record

	err := filepath.Walk(s.inputPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if filepath.Ext(path) != ".json" {
			return nil
		}

		if filepath.Base(path) == "conversations.json" {
			records, err := s.ProcessFileConversations(path, userName)
			if err != nil {
				fmt.Printf("Warning: Failed to process file %s: %v\n", path, err)
				return nil
			}

			allRecords = append(allRecords, records...)
			return nil
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return allRecords, nil
}

func ToDocuments(records []types.Record) ([]memory.TextDocument, error) {
	textDocuments := make([]memory.TextDocument, 0, len(records))

	for _, record := range records {
		getString := func(key string) string {
			if val, ok := record.Data[key]; ok {
				if strVal, ok := val.(string); ok {
					return strVal
				}
			}
			return ""
		}

		title := getString("title")

		var conversationBuilder strings.Builder

		if messagesInterface, ok := record.Data["messages"]; ok {
			switch messages := messagesInterface.(type) {
			case []ConversationMessage:

				for _, message := range messages {
					trimmedText := strings.TrimSpace(message.Text)
					if trimmedText == "" {
						continue
					}
					conversationBuilder.WriteString(message.Role)
					conversationBuilder.WriteString(": ")
					conversationBuilder.WriteString(trimmedText)
					conversationBuilder.WriteString("\n\n")
				}
			case []interface{}:

				for _, msgInterface := range messages {
					if msgMap, ok := msgInterface.(map[string]interface{}); ok {
						role := ""
						text := ""
						if roleVal, ok := msgMap["Role"]; ok {
							if roleStr, ok := roleVal.(string); ok {
								role = roleStr
							}
						}
						if textVal, ok := msgMap["Text"]; ok {
							if textStr, ok := textVal.(string); ok {
								text = textStr
							}
						}
						trimmedText := strings.TrimSpace(text)
						if role != "" && trimmedText != "" {
							conversationBuilder.WriteString(role)
							conversationBuilder.WriteString(": ")
							conversationBuilder.WriteString(trimmedText)
							conversationBuilder.WriteString("\n\n")
						}
					}
				}
			}
		}

		conversationText := conversationBuilder.String()
		if conversationText == "" {
			continue
		}

		explainer := "This document is a ChatGPT conversation log between user and assistant.\n\n"
		fullContent := explainer + conversationText

		textDocuments = append(textDocuments, memory.TextDocument{
			FieldContent:   fullContent,
			FieldTimestamp: &record.Timestamp,
			FieldTags:      []string{"chat", "chatgpt", "conversation"},
			FieldMetadata: map[string]string{
				"type":   "conversation",
				"title":  title,
				"source": "chatgpt",
			},
		})
	}
	return textDocuments, nil
}

func parseTimestamp(ts string) (time.Time, error) {
	parts := strings.Split(ts, ".")
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("invalid timestamp format: %s", ts)
	}

	seconds, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, err
	}

	return time.Unix(seconds, 0), nil
}

func (s *ChatGPTDataSource) Sync(ctx context.Context) ([]types.Record, error) {
	return nil, fmt.Errorf("sync operation not supported for Chatgpt")
}

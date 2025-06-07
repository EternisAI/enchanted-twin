package chatgpt

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/processor"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
	"github.com/EternisAI/enchanted-twin/pkg/db"
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

type ConversationMessage struct {
	Role string
	Text string
}
type ChatGPTProcessor struct {
	store *db.Store
}

func NewChatGPTProcessor(store *db.Store) processor.Processor {
	return &ChatGPTProcessor{store: store}
}

func (s *ChatGPTProcessor) Name() string {
	return "chatgpt"
}

func (s *ChatGPTProcessor) ProcessFile(ctx context.Context, filePath string) ([]types.Record, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context canceled before processing: %w", err)
	}

	jsonData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var conversations []ChatGPTConversation
	if err := json.Unmarshal(jsonData, &conversations); err != nil {
		return nil, err
	}

	var records []types.Record

	for i, conversation := range conversations {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("context canceled during conversation processing: %w", err)
		}

		timestamp, err := parseTimestamp(strconv.FormatFloat(conversation.CreateTime, 'f', -1, 64))
		if err != nil {
			log.Printf("Warning: Failed to parse timestamp for conversation %d (create_time: %f): %v", i, conversation.CreateTime, err)
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

		id := strconv.FormatFloat(conversation.CreateTime, 'f', -1, 64)
		conversationData := map[string]any{
			"id":       id,
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

func (s *ChatGPTProcessor) ProcessDirectory(ctx context.Context, inputPath string) ([]types.Record, error) {
	// Look for conversations.json in the root or chatgpt subdirectory
	possiblePaths := []string{
		filepath.Join(inputPath, "conversations.json"),
		filepath.Join(inputPath, "chatgpt", "conversations.json"),
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return s.ProcessFile(ctx, path)
		}
	}

	// If not found in expected locations, search recursively
	var records []types.Record
	err := filepath.Walk(inputPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || filepath.Base(path) != "conversations.json" {
			return nil
		}

		fileRecords, err := s.ProcessFile(ctx, path)
		if err != nil {
			fmt.Printf("Warning: Failed to process file %s: %v\n", path, err)
			return nil
		}

		records = append(records, fileRecords...)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error walking directory %s: %w", inputPath, err)
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("no conversations.json file found in directory %s", inputPath)
	}

	return records, nil
}

func (s *ChatGPTProcessor) Sync(ctx context.Context, accessToken string) ([]types.Record, bool, error) {
	return nil, false, fmt.Errorf("sync operation not supported for ChatGPT")
}

func (s *ChatGPTProcessor) ToDocuments(ctx context.Context, records []types.Record) ([]memory.Document, error) {
	conversationDocuments := make([]memory.ConversationDocument, 0, len(records))

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
		id := getString("id")

		conversation := memory.ConversationDocument{
			FieldID:     id,
			FieldSource: "chatgpt",
			FieldTags:   []string{"chat", "chatgpt", "conversation"},
			FieldMetadata: map[string]string{
				"type":  "conversation",
				"title": title,
			},
		}

		if messagesInterface, ok := record.Data["messages"]; ok {
			var messages []ConversationMessage

			// Handle both []ConversationMessage and []interface{} cases
			switch v := messagesInterface.(type) {
			case []ConversationMessage:
				messages = v
			case []interface{}:
				for _, msgInterface := range v {
					if msgMap, ok := msgInterface.(map[string]interface{}); ok {
						role, ok := msgMap["Role"].(string)
						if !ok {
							log.Printf("Warning: Role is not a string for message: %v", msgMap)
							continue
						}
						text, ok := msgMap["Text"].(string)
						if !ok {
							log.Printf("Warning: Text is not a string for message: %v", msgMap)
							continue
						}
						if role != "" && text != "" {
							messages = append(messages, ConversationMessage{
								Role: role,
								Text: text,
							})
						}
					}
				}
			}

			for _, message := range messages {
				conversation.Conversation = append(conversation.Conversation, memory.ConversationMessage{
					Speaker: message.Role,
					Content: message.Text,
					Time:    record.Timestamp,
				})
			}
		}

		if len(conversation.Conversation) > 0 {
			conversationDocuments = append(conversationDocuments, conversation)
		}
	}

	return memory.ConversationDocumentsToDocuments(conversationDocuments), nil
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

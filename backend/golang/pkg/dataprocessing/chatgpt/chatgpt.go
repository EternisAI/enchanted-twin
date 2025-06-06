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
		if i%10 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, fmt.Errorf("context canceled during conversation processing: %w", err)
			}
		}

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
	var allRecords []types.Record

	err := filepath.Walk(inputPath, func(path string, info os.FileInfo, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}

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
			records, err := s.ProcessFile(ctx, path)
			if err != nil {
				if ctx.Err() != nil {
					return err
				}
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

func (s *ChatGPTProcessor) Sync(ctx context.Context, accessToken string) ([]types.Record, bool, error) {
	return nil, false, fmt.Errorf("sync operation not supported for Chatgpt")
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

		id, ok := record.Data["id"].(string)
		if !ok {
			log.Printf("Skipping conversation with missing ID (%v)", record.Data)
			id = ""
		}

		conversation := memory.ConversationDocument{
			FieldID:     id,
			FieldSource: "chatgpt",
			User:        "user",
			FieldTags:   []string{"chat", "chatgpt", "conversation"},
			FieldMetadata: map[string]string{
				"type":  "conversation",
				"title": title,
			},
		}

		if messagesInterface, ok := record.Data["messages"]; ok {
			// Validate that messagesInterface is actually a slice before proceeding
			if messagesInterface == nil {
				log.Printf("Warning: messages field is nil for conversation %s", id)
				continue
			}

			switch messages := messagesInterface.(type) {
			case []interface{}:
				if err := s.processInterfaceMessages(messages, &conversation, record.Timestamp); err != nil {
					log.Printf("Warning: Failed to process interface messages for conversation %s: %v", id, err)
				}
			case []ConversationMessage:
				for _, message := range messages {
					if message.Role != "" && message.Text != "" {
						conversation.Conversation = append(conversation.Conversation, memory.ConversationMessage{
							Speaker: message.Role,
							Content: message.Text,
							Time:    record.Timestamp,
						})
					} else {
						log.Printf("Warning: Skipping message with empty role or text in conversation %s", id)
					}
				}
			default:
				log.Printf("Warning: Unexpected messages type %T for conversation %s, skipping", messagesInterface, id)
				continue
			}
		} else {
			log.Printf("Warning: No messages field found for conversation %s", id)
		}

		conversationDocuments = append(conversationDocuments, conversation)
	}

	var documents []memory.Document
	for _, conversationDocument := range conversationDocuments {
		documents = append(documents, &conversationDocument)
	}

	return documents, nil
}

func (s *ChatGPTProcessor) processInterfaceMessages(messages []interface{}, conversation *memory.ConversationDocument, timestamp time.Time) error {
	for i, messageInterface := range messages {
		if messageInterface == nil {
			log.Printf("Warning: Skipping nil message at index %d", i)
			continue
		}

		messageMap, ok := messageInterface.(map[string]interface{})
		if !ok {
			log.Printf("Warning: Skipping message at index %d with unexpected type %T", i, messageInterface)
			continue
		}

		// Safely extract role with validation
		roleInterface, exists := messageMap["Role"]
		if !exists {
			log.Printf("Warning: Skipping message at index %d with missing 'Role' key", i)
			continue
		}

		role, ok := roleInterface.(string)
		if !ok {
			log.Printf("Warning: Skipping message at index %d with invalid role type %T (expected string)", i, roleInterface)
			continue
		}

		// Safely extract text with validation
		textInterface, exists := messageMap["Text"]
		if !exists {
			log.Printf("Warning: Skipping message at index %d with missing 'Text' key", i)
			continue
		}

		text, ok := textInterface.(string)
		if !ok {
			log.Printf("Warning: Skipping message at index %d with invalid text type %T (expected string)", i, textInterface)
			continue
		}

		// Validate content before adding
		if role == "" || text == "" {
			log.Printf("Warning: Skipping message at index %d with empty role or text", i)
			continue
		}

		conversation.Conversation = append(conversation.Conversation, memory.ConversationMessage{
			Speaker: role,
			Content: text,
			Time:    timestamp,
		})
	}

	return nil
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

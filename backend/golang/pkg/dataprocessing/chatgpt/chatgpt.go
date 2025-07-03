package chatgpt

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
	store  *db.Store
	logger *log.Logger
}

func NewChatGPTProcessor(store *db.Store, logger *log.Logger) (*ChatGPTProcessor, error) {
	if store == nil {
		return nil, fmt.Errorf("store is nil")
	}

	if logger == nil {
		return nil, fmt.Errorf("logger is nil")
	}
	return &ChatGPTProcessor{store: store, logger: logger}, nil
}

func (s *ChatGPTProcessor) Name() string {
	return "chatgpt"
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

// ProcessFile implements the DocumentProcessor interface - direct ConversationDocument output.
func (s *ChatGPTProcessor) ProcessFile(ctx context.Context, filePath string) ([]memory.ConversationDocument, error) {
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

	var documents []memory.ConversationDocument

	for i, conversation := range conversations {
		if conversation.Title == "" {
			s.logger.Debug("Skipping conversation with empty title", "index", i)
			continue
		}

		timestamp, err := parseTimestamp(strconv.FormatFloat(conversation.CreateTime, 'f', -1, 64))
		if err != nil {
			s.logger.Warn("Failed to parse timestamp for conversation", "index", i, "create_time", conversation.CreateTime, "error", err)
			continue
		}

		var messages []memory.ConversationMessage

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
			messages = append(messages, memory.ConversationMessage{
				Speaker: role,
				Content: extractedText,
				Time:    timestamp,
			})
		}

		if len(messages) == 0 {
			continue
		}

		id := strconv.FormatFloat(conversation.CreateTime, 'f', -1, 64)

		conversationDoc := memory.ConversationDocument{
			User:         "user",
			People:       []string{"user", "assistant"},
			FieldID:      id,
			FieldSource:  "chatgpt",
			FieldTags:    []string{"chat", "conversation"},
			Conversation: messages,
			FieldMetadata: map[string]string{
				"type":  "conversation",
				"title": conversation.Title,
			},
		}

		documents = append(documents, conversationDoc)
	}

	return documents, nil
}

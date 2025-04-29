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

// MessageNode represents a node in the conversation tree.
type ChatGPTMessageNode struct {
	ID       string          `json:"id"`
	Message  *ChatGPTMessage `json:"message"` // Pointer because it can be null
	Parent   *string         `json:"parent"`  // Pointer because root node has null parent
	Children []string        `json:"children"`
}

// Message represents the actual message content and metadata.
type ChatGPTMessage struct {
	ID         string   `json:"id"`
	Author     Author   `json:"author"`
	CreateTime *float64 `json:"create_time"`
	Content    Content  `json:"content"`
	Metadata   Metadata `json:"metadata"`
	Recipient  string   `json:"recipient"`
}

// Author represents the sender of the message.
type Author struct {
	Role     string                 `json:"role"`
	Name     interface{}            `json:"name"` // Use interface{} as type is unknown (null)
	Metadata map[string]interface{} `json:"metadata"`
}

// Content represents the message content.
type Content struct {
	ContentType        string      `json:"content_type"`
	Parts              []string    `json:"parts,omitempty"`                // Optional for non-text content
	Language           string      `json:"language,omitempty"`             // Optional for text content
	ResponseFormatName interface{} `json:"response_format_name,omitempty"` // Optional, type unknown (null)
	Text               string      `json:"text,omitempty"`                 // Optional for non-code content
}

// Metadata contains various metadata associated with the message.
// Using map[string]interface{} for flexibility as fields vary significantly.
type Metadata map[string]interface{}

type ChatGPTDataSource struct {
	inputPath string
}

func New(inputPath string) *ChatGPTDataSource {
	return &ChatGPTDataSource{
		inputPath: inputPath,
	}
}

func (s *ChatGPTDataSource) Name() string {
	return "chatgpt"
}

func (s *ChatGPTDataSource) ProcessFileConversations(filePath string, username string) ([]types.Record, error) {
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

		for _, message := range conversation.Mapping {
			// Skip nodes without a message
			if message.Message == nil {
				continue
			}

			// Now safe to access message.Message fields
			if message.Message.Author.Role == "tool" {
				continue
			}
			content := message.Message.Content
			if content.ContentType != "text" {
				continue
			}
			content.Text = strings.Join(content.Parts, "")

			role := message.Message.Author.Role

			conversationData := map[string]any{
				"title": conversation.Title,
				"text":  content.Text,
				"role":  role,
			}
			record := types.Record{
				Data:      conversationData,
				Timestamp: timestamp,
				Source:    s.Name(),
			}
			records = append(records, record)

		}

	}

	return records, nil
}

func (s *ChatGPTDataSource) ProcessDirectory(userName string) ([]types.Record, error) {
	var allRecords []types.Record

	err := filepath.Walk(s.inputPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Only process .json files
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

func (s *ChatGPTDataSource) ToDocuments(records []types.Record) ([]memory.TextDocument, error) {
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

		message := getString("text")
		authorUsername := getString("username")
		channelName := getString("channelName")

		// Skip records with missing required fields
		if message == "" || authorUsername == "" || channelName == "" {
			continue
		}

		message = fmt.Sprintf("From %s in channel %s: %s", authorUsername, channelName, message)

		textDocuments = append(textDocuments, memory.TextDocument{
			Content:   message,
			Timestamp: &record.Timestamp,
			Tags:      []string{"social", "slack", "chat"},
			Metadata: map[string]string{
				"type":           "message",
				"channelName":    channelName,
				"authorUsername": authorUsername,
				"source":         "slack",
			},
		})
	}
	return textDocuments, nil
}

func parseTimestamp(ts string) (time.Time, error) {
	// Slack timestamps are in the format "1735051993.888329"
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
	return nil, fmt.Errorf("sync operation not supported for Slack")
}

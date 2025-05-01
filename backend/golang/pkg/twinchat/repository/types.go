package repository

import (
	"encoding/json"

	"github.com/EternisAI/enchanted-twin/graph/model"
)

type JSONForSQLLite string

// ChatDB is used for database operations with proper db field mapping.
type ChatDB struct {
	ID        string `db:"id"`
	Name      string `db:"name"`
	CreatedAt string `db:"created_at"` // Maps to the database column created_at
}

// ToModel converts a ChatDB to a model.Chat.
func (c *ChatDB) ToModel() model.Chat {
	return model.Chat{
		ID:        c.ID,
		Name:      c.Name,
		CreatedAt: c.CreatedAt,
	}
}

// Message represents a message in the chat.
type Message struct {
	ID             string  `db:"id"`
	ChatID         string  `db:"chat_id"`
	Text           string  `db:"text"`
	Role           string  `db:"role"`
	ToolCallsStr   *string `db:"tool_calls"`
	ToolResultsStr *string `db:"tool_results"`
	ImageURLsStr   *string `db:"image_urls"` // Stored as JSON string
	CreatedAtStr   string  `db:"created_at"` // Stored as RFC3339 string
}

func (m *Message) ToModel() *model.Message {
	imageUrls := make([]string, 0)
	if m.ImageURLsStr != nil {
		if err := json.Unmarshal([]byte(*m.ImageURLsStr), &imageUrls); err != nil {
			imageUrls = []string{}
		}
	}

	toolCalls := make([]*model.ToolCall, 0)
	if m.ToolCallsStr != nil {
		if err := json.Unmarshal([]byte(*m.ToolCallsStr), &toolCalls); err != nil {
			toolCalls = []*model.ToolCall{}
		}
	}

	toolCallResults := make([]string, 0)
	if m.ToolResultsStr != nil {
		if err := json.Unmarshal([]byte(*m.ToolResultsStr), &toolCallResults); err != nil {
			toolCallResults = []string{}
		}
	}

	return &model.Message{
		ID:          m.ID,
		Text:        &m.Text,
		Role:        model.Role(m.Role),
		ImageUrls:   imageUrls,
		CreatedAt:   m.CreatedAtStr,
		ToolCalls:   toolCalls,
		ToolResults: toolCallResults,
	}
}

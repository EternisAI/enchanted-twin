package repository

import (
	"encoding/json"
	"time"

	"github.com/EternisAI/enchanted-twin/graph/model"
)

type JSONForSQLLite string

type Message struct {
	ID          string         `json:"id"`
	ChatID      string         `json:"chat_id"`
	Text        string         `json:"text"`
	Role        string         `json:"role"`
	ToolCalls   JSONForSQLLite `json:"tool_calls"`
	ToolArgs    JSONForSQLLite `json:"tool_args"`
	ToolResults JSONForSQLLite `json:"tool_results"`
	ImageURLs   []string       `json:"image_urls"`
	CreatedAt   time.Time      `json:"created_at"`
}

func (m *Message) ToModel() *model.Message {
	var toolArgs []string
	_ = json.Unmarshal([]byte(m.ToolArgs), &toolArgs)
	return &model.Message{
		ID:   m.ID,
		Text: &m.Text,
		Role: model.Role(m.Role),
		// ToolResults: m.ToolResults,
		ImageUrls: m.ImageURLs,
		CreatedAt: m.CreatedAt.Format(time.RFC3339),
		// ToolCalls:   &m.ToolCalls,
	}
}

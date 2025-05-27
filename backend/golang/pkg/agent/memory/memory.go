// Owner: johan@eternis.ai
package memory

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ConversationMessage represents a single message in a conversation
type ConversationMessage struct {
	Speaker string    `json:"speaker"`
	Content string    `json:"content"`
	Time    time.Time `json:"time"`
}

// StructuredConversation represents the explicit conversation format
type StructuredConversation struct {
	Source       string                `json:"source"`
	People       []string              `json:"people"`
	User         string                `json:"user"`
	Conversation []ConversationMessage `json:"conversation"`
}

// ConversationDocument represents a document containing structured conversation data
type ConversationDocument struct {
	ID           string                 `json:"id"`
	Conversation StructuredConversation `json:"conversation"`
	Tags         []string               `json:"tags,omitempty"`
	Metadata     map[string]string      `json:"metadata,omitempty"`
}

// ToTextDocument converts a ConversationDocument to the legacy TextDocument format
func (cd *ConversationDocument) ToTextDocument() *TextDocument {
	var content strings.Builder

	for _, msg := range cd.Conversation.Conversation {
		content.WriteString(fmt.Sprintf("%s: %s\n", msg.Speaker, msg.Content))
	}

	// Use the timestamp of the first message if available
	var timestamp *time.Time
	if len(cd.Conversation.Conversation) > 0 {
		timestamp = &cd.Conversation.Conversation[0].Time
	}

	// Copy metadata and add conversation-specific metadata
	metadata := make(map[string]string)
	for k, v := range cd.Metadata {
		metadata[k] = v
	}
	metadata["source"] = cd.Conversation.Source
	metadata["user"] = cd.Conversation.User

	return &TextDocument{
		ID:        cd.ID,
		Content:   strings.TrimSpace(content.String()),
		Timestamp: timestamp,
		Tags:      cd.Tags,
		Metadata:  metadata,
	}
}

// TextDocument represents a legacy document format used internally by storage
type TextDocument struct {
	ID        string
	Content   string
	Timestamp *time.Time
	Tags      []string
	Metadata  map[string]string
}

// MemoryFact represents an extracted fact about a person
type MemoryFact struct {
	ID        string            `json:"id"`
	Speaker   string            `json:"speaker"`
	Content   string            `json:"content"`
	Timestamp time.Time         `json:"timestamp"`
	Source    string            `json:"source"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type QueryResult struct {
	Facts     []MemoryFact   `json:"facts"`
	Documents []TextDocument `json:"documents,omitempty"` // For backward compatibility
}

type ProgressUpdate struct {
	Processed int `json:"processed"`
	Total     int `json:"total"`
}

// Storage interface for the memory system
type Storage interface {
	Store(ctx context.Context, documents []ConversationDocument, progressChan chan<- ProgressUpdate) error
	Query(ctx context.Context, query string) (QueryResult, error)
}

// Owner: johan@eternis.ai
package memory

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Document interface that both TextDocument and ConversationDocument implement
type Document interface {
	GetID() string
	GetContent() string
	GetTimestamp() *time.Time
	GetTags() []string
	GetMetadata() map[string]string

	// Type discrimination methods
	IsConversation() bool
	AsConversation() (*ConversationDocument, bool)
	AsText() (*TextDocument, bool)
}

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

// Document interface implementation for ConversationDocument
func (cd *ConversationDocument) GetID() string {
	return cd.ID
}

func (cd *ConversationDocument) GetContent() string {
	var content strings.Builder
	for _, msg := range cd.Conversation.Conversation {
		content.WriteString(fmt.Sprintf("%s: %s\n", msg.Speaker, msg.Content))
	}
	return strings.TrimSpace(content.String())
}

func (cd *ConversationDocument) GetTimestamp() *time.Time {
	if len(cd.Conversation.Conversation) > 0 {
		return &cd.Conversation.Conversation[0].Time
	}
	return nil
}

func (cd *ConversationDocument) GetTags() []string {
	return cd.Tags
}

func (cd *ConversationDocument) GetMetadata() map[string]string {
	metadata := make(map[string]string)
	for k, v := range cd.Metadata {
		metadata[k] = v
	}
	metadata["source"] = cd.Conversation.Source
	metadata["user"] = cd.Conversation.User
	return metadata
}

func (cd *ConversationDocument) IsConversation() bool {
	return true
}

func (cd *ConversationDocument) AsConversation() (*ConversationDocument, bool) {
	return cd, true
}

func (cd *ConversationDocument) AsText() (*TextDocument, bool) {
	return cd.ToTextDocument(), true
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

// Document interface implementation for TextDocument
func (td *TextDocument) GetID() string {
	return td.ID
}

func (td *TextDocument) GetContent() string {
	return td.Content
}

func (td *TextDocument) GetTimestamp() *time.Time {
	return td.Timestamp
}

func (td *TextDocument) GetTags() []string {
	return td.Tags
}

func (td *TextDocument) GetMetadata() map[string]string {
	return td.Metadata
}

func (td *TextDocument) IsConversation() bool {
	return false
}

func (td *TextDocument) AsConversation() (*ConversationDocument, bool) {
	return nil, false
}

func (td *TextDocument) AsText() (*TextDocument, bool) {
	return td, true
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
	Store(ctx context.Context, documents []Document, progressChan chan<- ProgressUpdate) error
	Query(ctx context.Context, query string) (QueryResult, error)
}

// Helper functions to convert slices to Document interface

// TextDocumentsToDocuments converts a slice of TextDocument to a slice of Document
func TextDocumentsToDocuments(textDocs []TextDocument) []Document {
	docs := make([]Document, len(textDocs))
	for i := range textDocs {
		docs[i] = &textDocs[i]
	}
	return docs
}

// ConversationDocumentsToDocuments converts a slice of ConversationDocument to a slice of Document
func ConversationDocumentsToDocuments(convDocs []ConversationDocument) []Document {
	docs := make([]Document, len(convDocs))
	for i := range convDocs {
		docs[i] = &convDocs[i]
	}
	return docs
}

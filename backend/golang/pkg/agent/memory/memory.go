// Owner: dmitry@eternis.ai
package memory

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Document interface that both TextDocument and ConversationDocument implement.
type Document interface {
	ID() string
	Content() string
	Timestamp() *time.Time
	Tags() []string
	Metadata() map[string]string
}

// ConversationMessage represents a single message in a conversation.
type ConversationMessage struct {
	Speaker string    `json:"speaker"`
	Content string    `json:"content"`
	Time    time.Time `json:"time"`
}

// StructuredConversation represents the explicit conversation format.
type StructuredConversation struct {
	Source       string                `json:"source"`
	People       []string              `json:"people"`
	User         string                `json:"user"`
	Conversation []ConversationMessage `json:"conversation"`
}

// ConversationDocument represents a document containing structured conversation data.
type ConversationDocument struct {
	FieldID       string                 `json:"id"`
	Conversation  StructuredConversation `json:"conversation"`
	FieldTags     []string               `json:"tags,omitempty"`
	FieldMetadata map[string]string      `json:"metadata,omitempty"`
}

// Document interface implementation for ConversationDocument.
func (cd *ConversationDocument) ID() string {
	return cd.FieldID
}

func (cd *ConversationDocument) Content() string {
	var content strings.Builder
	for _, msg := range cd.Conversation.Conversation {
		content.WriteString(fmt.Sprintf("%s: %s\n", msg.Speaker, msg.Content))
	}
	return strings.TrimSpace(content.String())
}

func (cd *ConversationDocument) Timestamp() *time.Time {
	if len(cd.Conversation.Conversation) > 0 {
		return &cd.Conversation.Conversation[0].Time
	}
	return nil
}

func (cd *ConversationDocument) Tags() []string {
	return cd.FieldTags
}

func (cd *ConversationDocument) Metadata() map[string]string {
	metadata := make(map[string]string)
	for k, v := range cd.FieldMetadata {
		metadata[k] = v
	}
	metadata["source"] = cd.Conversation.Source
	metadata["user"] = cd.Conversation.User
	return metadata
}

// ToTextDocument converts a ConversationDocument to the legacy TextDocument format.
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
	for k, v := range cd.FieldMetadata {
		metadata[k] = v
	}
	metadata["source"] = cd.Conversation.Source
	metadata["user"] = cd.Conversation.User

	return &TextDocument{
		FieldID:        cd.FieldID,
		FieldContent:   strings.TrimSpace(content.String()),
		FieldTimestamp: timestamp,
		FieldTags:      cd.FieldTags,
		FieldMetadata:  metadata,
	}
}

// TextDocument represents a legacy document format used internally by storage.
type TextDocument struct {
	FieldID        string            `json:"id"`
	FieldContent   string            `json:"content"`
	FieldTimestamp *time.Time        `json:"timestamp"`
	FieldTags      []string          `json:"tags,omitempty"`
	FieldMetadata  map[string]string `json:"metadata,omitempty"`
}

// Document interface implementation for TextDocument.
func (td *TextDocument) ID() string {
	return td.FieldID
}

func (td *TextDocument) Content() string {
	return td.FieldContent
}

func (td *TextDocument) Timestamp() *time.Time {
	return td.FieldTimestamp
}

func (td *TextDocument) Tags() []string {
	return td.FieldTags
}

func (td *TextDocument) Metadata() map[string]string {
	return td.FieldMetadata
}

// MemoryFact represents an extracted fact about a person.
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

// Storage interface for the memory system.
type Storage interface {
	Store(ctx context.Context, documents []Document, progressChan chan<- ProgressUpdate) error
	Query(ctx context.Context, query string) (QueryResult, error)
}

// Helper functions to convert slices to Document interface

// TextDocumentsToDocuments converts a slice of TextDocument to a slice of Document.
func TextDocumentsToDocuments(textDocs []TextDocument) []Document {
	docs := make([]Document, len(textDocs))
	for i := range textDocs {
		docs[i] = &textDocs[i]
	}
	return docs
}

// ConversationDocumentsToDocuments converts a slice of ConversationDocument to a slice of Document.
func ConversationDocumentsToDocuments(convDocs []ConversationDocument) []Document {
	docs := make([]Document, len(convDocs))
	for i := range convDocs {
		docs[i] = &convDocs[i]
	}
	return docs
}

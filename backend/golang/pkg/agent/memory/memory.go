// Owner: dmitry@eternis.ai
package memory

import (
	"context"
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
	Source() string
}

// ConversationMessage represents a single message in a conversation.
type ConversationMessage struct {
	Speaker string    `json:"speaker"`
	Content string    `json:"content"`
	Time    time.Time `json:"time"`
}

// ConversationDocument represents a document containing structured conversation data.
type ConversationDocument struct {
	FieldID       string                `json:"id"`
	FieldSource   string                `json:"source"`       // Merged from StructuredConversation
	People        []string              `json:"people"`       // Merged from StructuredConversation
	User          string                `json:"user"`         // Merged from StructuredConversation
	Conversation  []ConversationMessage `json:"conversation"` // Merged from StructuredConversation
	FieldTags     []string              `json:"tags,omitempty"`
	FieldMetadata map[string]string     `json:"metadata,omitempty"`
}

// Document interface implementation for ConversationDocument.
func (cd *ConversationDocument) ID() string {
	return cd.FieldID
}

func (cd *ConversationDocument) Content() string {
	var content strings.Builder
	content.Grow(len(cd.Conversation) * 50) // rough estimate
	hasContent := false                     // Track if any substantive content is added
	for _, msg := range cd.Conversation {
		trimmedMsgContent := strings.TrimSpace(msg.Content)
		if trimmedMsgContent == "" {
			continue // Skip messages with only whitespace content
		}
		content.WriteString(msg.Speaker)
		content.WriteString(": ")
		content.WriteString(trimmedMsgContent)
		content.WriteString("\n")
		hasContent = true
	}
	if !hasContent {
		return "" // If no messages had real content, return empty string
	}
	return strings.TrimSpace(content.String()) // Final trim for the whole block
}

func (cd *ConversationDocument) Timestamp() *time.Time {
	if len(cd.Conversation) > 0 {
		return &cd.Conversation[0].Time
	}
	return nil
}

func (cd *ConversationDocument) Tags() []string {
	return cd.FieldTags
}

func (cd *ConversationDocument) Metadata() map[string]string {
	metadata := make(map[string]string)
	if cd.FieldMetadata != nil {
		for k, v := range cd.FieldMetadata {
			metadata[k] = v
		}
	}
	metadata["source"] = cd.FieldSource
	metadata["user"] = cd.User
	// Add people to metadata if needed, for now, it's directly accessible
	// metadata["people"] = strings.Join(cd.People, ", ")
	return metadata
}

func (cd *ConversationDocument) Source() string {
	return cd.FieldSource
}

// TextDocument represents a document format used internally by storage.
type TextDocument struct {
	FieldID        string            `json:"id"`
	FieldContent   string            `json:"content"`
	FieldTimestamp *time.Time        `json:"timestamp"`
	FieldSource    string            `json:"source,omitempty"`
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
	// Ensure metadata is not nil
	if td.FieldMetadata == nil {
		return make(map[string]string)
	}
	return td.FieldMetadata // Source is no longer guaranteed to be in metadata; use Source() method
}

func (td *TextDocument) Source() string {
	return td.FieldSource // Now returns the top-level field
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

type ProgressCallback func(processed, total int)

type Storage interface {
	Store(ctx context.Context, documents []Document, progressCallback ProgressCallback) error
	Query(ctx context.Context, query string) (QueryResult, error)
}

// Helper functions to convert slices to Document interface

// TextDocumentsToDocuments converts a slice of TextDocument to a slice of Document.
func TextDocumentsToDocuments(textDocs []TextDocument) []Document {
	docs := make([]Document, len(textDocs))
	for i := range textDocs {
		// Create a new variable for the address operation to avoid capturing loop variable
		doc := textDocs[i]
		docs[i] = &doc
	}
	return docs
}

// ConversationDocumentsToDocuments converts a slice of ConversationDocument to a slice of Document.
func ConversationDocumentsToDocuments(convDocs []ConversationDocument) []Document {
	docs := make([]Document, len(convDocs))
	for i := range convDocs {
		// Create a new variable for the address operation to avoid capturing loop variable
		doc := convDocs[i]
		docs[i] = &doc
	}
	return docs
}

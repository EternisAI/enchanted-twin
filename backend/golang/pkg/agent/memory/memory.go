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
	for _, msg := range cd.Conversation {
		content.WriteString(fmt.Sprintf("%s: %s\n", msg.Speaker, msg.Content))
	}
	return strings.TrimSpace(content.String())
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

// ToTextDocument converts a ConversationDocument to the legacy TextDocument format.
func (cd *ConversationDocument) ToTextDocument() *TextDocument {
	// Use the timestamp of the first message if available
	var timestamp *time.Time
	if len(cd.Conversation) > 0 {
		timestamp = &cd.Conversation[0].Time
	}

	// Copy metadata and add conversation-specific metadata
	metadata := make(map[string]string)
	if cd.FieldMetadata != nil {
		for k, v := range cd.FieldMetadata {
			metadata[k] = v
		}
	}
	metadata["source"] = cd.FieldSource // Use direct field
	metadata["user"] = cd.User          // Use direct field

	return &TextDocument{
		FieldID:        cd.FieldID,
		FieldContent:   cd.Content(), // Simplified to use Content() method
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
	// Ensure metadata is not nil, especially if "source" might be missing
	if td.FieldMetadata == nil {
		return make(map[string]string)
	}
	return td.FieldMetadata
}

func (td *TextDocument) Source() string {
	if td.FieldMetadata != nil {
		if source, ok := td.FieldMetadata["source"]; ok {
			return source
		}
	}
	return "" // Return empty string if source is not found
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
	Store(ctx context.Context, documents []TextDocument, progressCallback ProgressCallback) error
	StoreRawData(ctx context.Context, documents []TextDocument, progressCallback ProgressCallback) error
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

package memory

import (
	"context"
	"time"
)

// TextDocument represents a single piece of text information to be stored or retrieved.
type TextDocument struct {
	ID        string            `json:"id,omitempty"`       // Unique identifier (omitempty for new documents)
	Content   string            `json:"content"`            // The actual text content
	Timestamp *time.Time        `json:"timestamp"`          // When the document was created/recorded (NOW A POINTER)
	Tags      []string          `json:"tags,omitempty"`     // Keywords or categories
	Metadata  map[string]string `json:"metadata,omitempty"` // Other arbitrary metadata
	// Speaker   string            `json:"speaker,omitempty"`   // REMOVED
	// Certainty float64           `json:"certainty,omitempty"` // REMOVED
}

// QueryResult holds the results of a query operation.
type QueryResult struct {
	Text      []string       `json:"text,omitempty"` // Potentially a summary or direct answers
	Documents []TextDocument `json:"documents"`      // The list of relevant documents found
}

// ProgressUpdate signals progress during a Store operation.
type ProgressUpdate struct {
	Processed int    `json:"processed"`
	Total     int    `json:"total"`
	Message   string `json:"message,omitempty"` // Optional message
}

// Storage defines the interface for memory storage and retrieval operations.
type Storage interface {
	// Store processes and stores a batch of documents.
	// It can optionally send progress updates through the provided channel.
	Store(ctx context.Context, documents []TextDocument, progressChan chan<- ProgressUpdate) error

	// Query retrieves documents relevant to a query string.
	Query(ctx context.Context, query string) (QueryResult, error) // k int REMOVED
}

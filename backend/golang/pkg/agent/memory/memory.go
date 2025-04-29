package memory

import (
	"context"
	"time"
)

type TextDocument struct {
	ID        string
	Content   string
	Timestamp *time.Time
	Tags      []string
	Metadata  map[string]string
}

type QueryResult struct {
	Text      []string
	Documents []TextDocument
}

type ProgressUpdate struct {
	Processed int
	Total     int
}

type Storage interface {
	Store(ctx context.Context, documents []TextDocument, progressChan chan<- ProgressUpdate) error
	Query(ctx context.Context, query string) (QueryResult, error)
}

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
}

type QueryResult struct {
	Text      []string
	Documents []TextDocument
}

type Storage interface {
	Store(ctx context.Context, documents []TextDocument) error
	Query(ctx context.Context, query string) (QueryResult, error)
}

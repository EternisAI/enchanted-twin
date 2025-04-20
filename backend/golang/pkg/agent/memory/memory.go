package memory

import "time"

type TextDocument struct {
	ID        string
	Content   string
	Timestamp *time.Time
	Tags      []string
}

type Storage interface {
	Store(documents []TextDocument) error
	Query(query string) ([]TextDocument, error)
}

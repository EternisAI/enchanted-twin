// Owner: johan@eternis.ai
package memory

import (
	"context"
	"fmt"
	"time"
)

type TextDocument struct {
	ID        string
	Content   string
	Timestamp *time.Time
	Tags      []string
	Metadata  map[string]string
}

func (d *TextDocument) String() string {
	metadataString := ""
	for k, v := range d.Metadata {
		metadataString += fmt.Sprintf("%s: %s. ", k, v)
	}
	return fmt.Sprintf("Content: %s, Timestamp: %s, Metadata: %s", d.Content, d.Timestamp, metadataString)
}

type DocumentWithDistance struct {
	Document TextDocument
	Distance float32
}

type QueryResult struct {
	Text      []string
	Documents []TextDocument
}

type QueryWithDistanceResult struct {
	Documents []DocumentWithDistance
}

type ProgressUpdate struct {
	Processed int
	Total     int
}

type Storage interface {
	Store(ctx context.Context, documents []TextDocument, progressChan chan<- ProgressUpdate) error
	Query(ctx context.Context, query string) (QueryResult, error)
	QueryWithDistance(ctx context.Context, query string, metadataFilters ...map[string]string) (QueryWithDistanceResult, error)
}

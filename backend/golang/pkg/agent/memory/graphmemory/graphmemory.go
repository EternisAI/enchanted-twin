package graphmemory

import (
	"context"
	"database/sql"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

type GraphMemory struct {
	db *sql.DB
	ai *ai.Service
}

func NewGraphMemory(pgString string, ai *ai.Service) (*GraphMemory, error) {
	db, err := sql.Open("postgres", pgString)
	if err != nil {
		return nil, err
	}

	return &GraphMemory{db: db, ai: ai}, nil
}

func (g *GraphMemory) Store(ctx context.Context, documents []memory.TextDocument) error {
	// Cosmin will implement
	return nil
}

func (g *GraphMemory) Query(ctx context.Context, query string) ([]memory.TextDocument, error) {

	return nil, nil
}

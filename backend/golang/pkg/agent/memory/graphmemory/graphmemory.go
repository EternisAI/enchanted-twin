package graphmemory

import (
	"database/sql"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
)

type GraphMemory struct {
	db *sql.DB
}

func NewGraphMemory(pgString string) (*GraphMemory, error) {
	db, err := sql.Open("postgres", pgString)
	if err != nil {
		return nil, err
	}

	return &GraphMemory{db: db}, nil
}

func (g *GraphMemory) Store(documents []memory.TextDocument) error {
	// Cosmin will implement
	return nil
}

func (g *GraphMemory) Query(query string) ([]memory.TextDocument, error) {
	// August will implement
	return nil, nil
}

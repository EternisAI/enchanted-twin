package embeddingsmemory

import (
	"fmt"

	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/charmbracelet/log"
	"github.com/jmoiron/sqlx"
)

type EmbeddingsMemory struct {
	db                  *sqlx.DB
	ai                  *ai.Service
	logger              *log.Logger
	embeddingsModelName string
}

func NewEmbeddingsMemory(logger *log.Logger, pgString string, ai *ai.Service, recreate bool, completionsModelName string) (*EmbeddingsMemory, error) {
	db, err := sqlx.Open("postgres", pgString)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	mem := &EmbeddingsMemory{db: db, ai: ai, logger: logger, embeddingsModelName: completionsModelName}

	if err := mem.ensureDbSchema(recreate); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, fmt.Errorf("failed to create database schema: %w (close error: %v)", err, closeErr)
		}
		return nil, fmt.Errorf("failed to create database schema: %w", err)
	}

	return mem, nil
}

func (m *EmbeddingsMemory) ensureDbSchema(recreate bool) error {
	if _, err := m.db.Exec(sqlExtensions); err != nil {
		return err
	}

	if recreate {
		if _, err := m.db.Exec(sqlDropSchema); err != nil {
			return err
		}
	}

	if _, err := m.db.Exec(sqlSchema); err != nil {
		return err
	}

	return nil
}

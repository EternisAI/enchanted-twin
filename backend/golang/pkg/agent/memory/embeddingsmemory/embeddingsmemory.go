package embeddingsmemory

import (
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/jmoiron/sqlx"

	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

type EmbeddingsMemory struct {
	db                  *sqlx.DB
	ai                  *ai.Service
	logger              *log.Logger
	embeddingsModelName string
}

type Config struct {
	AI                  *ai.Service
	Logger              *log.Logger
	PgString            string
	Recreate            bool
	EmbeddingsModelName string
}

const (
	EmbeddingLength = 1536
)

func NewEmbeddingsMemory(config *Config) (*EmbeddingsMemory, error) {
	db, err := sqlx.Open("postgres", config.PgString)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	mem := &EmbeddingsMemory{db: db, ai: config.AI, logger: config.Logger, embeddingsModelName: config.EmbeddingsModelName}

	if err := mem.ensureDbSchema(config.Recreate); err != nil {
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

// OpenAI embeddings are 1536 dim, we want to pad Ollama embeddings to 1536 dim.
func padVector(vec []float64, length int) []float64 {
	if len(vec) >= length {
		return vec[:length]
	}
	return append(vec, make([]float64, length-len(vec))...)
}

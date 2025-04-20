package graphmemory

import (
	"context"
	"database/sql"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/ai"

	_ "github.com/lib/pq" // PostgreSQL driver
)

type GraphMemory struct {
	db *sql.DB
	ai *ai.Service
}

func NewGraphMemory(pgString string, ai *ai.Service, recreate bool) (*GraphMemory, error) {
	db, err := sql.Open("postgres", pgString)
	if err != nil {
		return nil, err
	}

	mem := &GraphMemory{db: db, ai: ai}
	if err := mem.ensureDbSchema(recreate); err != nil {
		return nil, err
	}

	return mem, nil
}

func (g *GraphMemory) Store(ctx context.Context, documents []memory.TextDocument) error {
	// Cosmin will implement
	return nil
}

func (g *GraphMemory) Query(ctx context.Context, query string) ([]memory.TextDocument, error) {

	return nil, nil
}

const sqlExtensions = `
-- Enable the hstore extension for PostgreSQL
CREATE EXTENSION IF NOT EXISTS hstore;
-- CREATE EXTENSION IF NOT EXISTS vector; -- TODO(cosmic): install pgvector
`

const sqlDropSchema = `
-- Drop existing tables if they exist
DROP TABLE IF EXISTS facts;
DROP TABLE IF EXISTS text_entries;
DROP TABLE IF EXISTS embeddings;
`

const sqlSchema = `
-- Raw text entries and metadata
CREATE TABLE IF NOT EXISTS text_entries (
  id BIGSERIAL PRIMARY KEY,
  text TEXT NOT NULL,
  meta HSTORE  -- additional metadata
);

-- Add unique constraint with coalesced meta to handle nulls
CREATE UNIQUE INDEX IF NOT EXISTS text_entries_text_meta_idx ON text_entries (text, COALESCE(meta, ''));

-- Table for storing extracted structured facts
CREATE TABLE facts (
  id BIGSERIAL PRIMARY KEY,
  text_entry_id BIGINT NOT NULL,
  sub TEXT NOT NULL,  -- Subject
  prd TEXT NOT NULL,  -- Predicate
  obj TEXT NOT NULL,  -- Object
  FOREIGN KEY (text_entry_id) REFERENCES text_entries(id) ON DELETE CASCADE
);

-- Create GIN indexes for fulltext search on subject and object
CREATE INDEX facts_sub_fulltext_idx ON facts USING GIN (to_tsvector('english', sub));
CREATE INDEX facts_obj_fulltext_idx ON facts USING GIN (to_tsvector('english', obj));

-- Table for storing embedding vectors
CREATE TABLE embeddings (
  id BIGSERIAL PRIMARY KEY,
  text_entry_id BIGINT NOT NULL,
  -- guts vector(1536),  -- TODO Using pgvector for embedding vectors
  FOREIGN KEY (text_entry_id) REFERENCES text_entries(id) ON DELETE CASCADE
);

-- Create an index on the embedding vectors for faster similarity search
-- CREATE INDEX ON embeddings USING hnsw (guts vector_cosine_ops); -- TODO
`

func (g *GraphMemory) ensureDbSchema(recreate bool) error {
	if _, err := g.db.Exec(sqlExtensions); err != nil {
		return err
	}

	if recreate {
		if _, err := g.db.Exec(sqlDropSchema); err != nil {
			return err
		}
	}

	if _, err := g.db.Exec(sqlSchema); err != nil {
		return err
	}

	return nil
}

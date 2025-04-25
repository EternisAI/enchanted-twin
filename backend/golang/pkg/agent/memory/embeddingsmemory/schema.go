package embeddingsmemory

const sqlExtensions = `
-- Enable necessary extensions
CREATE EXTENSION IF NOT EXISTS vector;
`

const sqlDropSchema = `
-- Drop existing tables if they exist
DROP TABLE IF EXISTS embeddings CASCADE;
DROP TABLE IF EXISTS text_entries CASCADE;
`

const sqlSchema = `
-- Raw text entries and metadata
CREATE TABLE IF NOT EXISTS text_entries (
  id UUID PRIMARY KEY,
  text TEXT NOT NULL,
  meta JSONB DEFAULT '{}'::jsonb,  -- Using JSONB for key-value metadata
  text_hash TEXT GENERATED ALWAYS AS (MD5(text)) STORED,
  created_at TIMESTAMP WITH TIME ZONE NOT NULL  -- Timestamp column
);

-- Create indexes for text_entries
CREATE INDEX IF NOT EXISTS text_entries_meta_gin_idx ON text_entries USING GIN (meta);
CREATE INDEX IF NOT EXISTS text_entries_created_at_idx ON text_entries (created_at);  -- Index for timestamp

-- Table for storing embedding vectors
CREATE TABLE IF NOT EXISTS embeddings (
  id BIGSERIAL PRIMARY KEY,
  text_entry_id UUID NOT NULL,
  embedding vector(1536),  -- Using pgvector for embedding vectors
  FOREIGN KEY (text_entry_id) REFERENCES text_entries(id) ON DELETE CASCADE
);

-- Create index for vector similarity search
CREATE INDEX IF NOT EXISTS embeddings_vector_idx ON embeddings USING hnsw (embedding vector_cosine_ops);
`

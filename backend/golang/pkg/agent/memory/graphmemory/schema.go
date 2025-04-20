package graphmemory

const sqlExtensions = `
-- Enable the hstore extension for PostgreSQL
CREATE EXTENSION IF NOT EXISTS hstore;
-- Enable ltree for hierarchical tag support
CREATE EXTENSION IF NOT EXISTS ltree;
-- CREATE EXTENSION IF NOT EXISTS vector; -- TODO(cosmic): install pgvector
`

const sqlDropSchema = `
-- Drop existing tables if they exist (in correct dependency order)
DROP TABLE IF EXISTS facts CASCADE;
DROP TABLE IF EXISTS embeddings CASCADE;
DROP TABLE IF EXISTS text_entries CASCADE;
`

const sqlSchema = `
-- Raw text entries and metadata
CREATE TABLE IF NOT EXISTS text_entries (
  id BIGSERIAL PRIMARY KEY,
  text TEXT NOT NULL,
  meta HSTORE,  -- additional metadata
  tags ltree[] DEFAULT NULL, -- hierarchical tags stored as ltree array
  created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Add unique constraint with coalesced meta to handle nulls
CREATE UNIQUE INDEX IF NOT EXISTS text_entries_text_meta_idx ON text_entries (text, COALESCE(meta, ''));

-- Create GiST index on ltree array for efficient tag-based queries
CREATE INDEX IF NOT EXISTS text_entries_tags_idx ON text_entries USING GIST (tags);

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

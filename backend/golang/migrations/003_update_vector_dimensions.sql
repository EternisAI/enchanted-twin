-- +goose Up
-- Update vector dimensions from 1536 to 768 for jina-embeddings-v2-base-en

-- Drop existing vector columns and recreate with correct dimensions
ALTER TABLE memory_facts DROP COLUMN IF EXISTS content_vector;
ALTER TABLE memory_facts ADD COLUMN content_vector VECTOR(768); -- jina-embeddings-v2-base-en dimensions

ALTER TABLE document_chunks DROP COLUMN IF EXISTS content_vector;
ALTER TABLE document_chunks ADD COLUMN content_vector VECTOR(768); -- jina-embeddings-v2-base-en dimensions

-- Recreate indexes for the new vector columns
CREATE INDEX IF NOT EXISTS memory_facts_content_vector_idx ON memory_facts USING hnsw (content_vector vector_cosine_ops);
CREATE INDEX IF NOT EXISTS document_chunks_content_vector_idx ON document_chunks USING hnsw (content_vector vector_cosine_ops);

-- +goose Down
-- Revert vector dimensions back to 1536 for OpenAI embeddings

ALTER TABLE memory_facts DROP COLUMN IF EXISTS content_vector;
ALTER TABLE memory_facts ADD COLUMN content_vector VECTOR(1536); -- OpenAI embedding dimensions

ALTER TABLE document_chunks DROP COLUMN IF EXISTS content_vector;
ALTER TABLE document_chunks ADD COLUMN content_vector VECTOR(1536); -- OpenAI embedding dimensions

-- Recreate indexes for the reverted vector columns
CREATE INDEX IF NOT EXISTS memory_facts_content_vector_idx ON memory_facts USING hnsw (content_vector vector_cosine_ops);
CREATE INDEX IF NOT EXISTS document_chunks_content_vector_idx ON document_chunks USING hnsw (content_vector vector_cosine_ops);
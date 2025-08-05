-- +goose Up
-- Enable pgvector extension
CREATE EXTENSION IF NOT EXISTS vector;

-- Memory facts table (equivalent to MemoryFact class)
CREATE TABLE memory_facts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    content TEXT NOT NULL,
    content_vector VECTOR(1536) NOT NULL, -- OpenAI embedding dimensions
    timestamp TIMESTAMPTZ NOT NULL,
    source TEXT NOT NULL,
    tags TEXT[] DEFAULT '{}',
    document_references TEXT[] DEFAULT '{}',
    metadata_json JSONB DEFAULT '{}',
    -- Structured fact fields
    fact_category TEXT NOT NULL,
    fact_subject TEXT NOT NULL, 
    fact_attribute TEXT,
    fact_value TEXT,
    fact_temporal_context TEXT,
    fact_sensitivity TEXT,
    fact_importance INTEGER,
    fact_file_path TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Documents table (equivalent to SourceDocument class)
CREATE TABLE source_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    content TEXT NOT NULL,
    content_hash TEXT NOT NULL UNIQUE,
    document_type TEXT NOT NULL,
    original_id TEXT NOT NULL,
    metadata_json JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Document chunks table (equivalent to DocumentChunk class)  
CREATE TABLE document_chunks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    content TEXT NOT NULL,
    content_vector VECTOR(1536),
    chunk_index INTEGER NOT NULL,
    original_document_id TEXT NOT NULL,
    source TEXT NOT NULL,
    file_path TEXT,
    tags TEXT[] DEFAULT '{}',
    metadata_json JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS document_chunks;
DROP TABLE IF EXISTS source_documents;
DROP TABLE IF EXISTS memory_facts;
DROP EXTENSION IF EXISTS vector;
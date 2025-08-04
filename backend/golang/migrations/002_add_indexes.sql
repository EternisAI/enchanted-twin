-- +goose Up
-- Indexes for memory_facts table
CREATE INDEX idx_memory_facts_vector ON memory_facts USING ivfflat (content_vector vector_cosine_ops);
CREATE INDEX idx_memory_facts_source ON memory_facts(source);
CREATE INDEX idx_memory_facts_timestamp ON memory_facts(timestamp);
CREATE INDEX idx_memory_facts_category ON memory_facts(fact_category);
CREATE INDEX idx_memory_facts_subject ON memory_facts(fact_subject);
CREATE INDEX idx_memory_facts_importance ON memory_facts(fact_importance);
CREATE INDEX idx_memory_facts_file_path ON memory_facts(fact_file_path);
CREATE INDEX idx_memory_facts_tags ON memory_facts USING GIN(tags);
CREATE INDEX idx_memory_facts_document_references ON memory_facts USING GIN(document_references);

-- Indexes for document_chunks table
CREATE INDEX idx_document_chunks_vector ON document_chunks USING ivfflat (content_vector vector_cosine_ops);
CREATE INDEX idx_document_chunks_source ON document_chunks(source);
CREATE INDEX idx_document_chunks_file_path ON document_chunks(file_path);
CREATE INDEX idx_document_chunks_original_document_id ON document_chunks(original_document_id);

-- Indexes for source_documents table
CREATE INDEX idx_source_documents_hash ON source_documents(content_hash);
CREATE INDEX idx_source_documents_type ON source_documents(document_type);
CREATE INDEX idx_source_documents_original_id ON source_documents(original_id);

-- +goose Down
DROP INDEX IF EXISTS idx_source_documents_original_id;
DROP INDEX IF EXISTS idx_source_documents_type;
DROP INDEX IF EXISTS idx_source_documents_hash;
DROP INDEX IF EXISTS idx_document_chunks_original_document_id;
DROP INDEX IF EXISTS idx_document_chunks_file_path;
DROP INDEX IF EXISTS idx_document_chunks_source;
DROP INDEX IF EXISTS idx_document_chunks_vector;
DROP INDEX IF EXISTS idx_memory_facts_document_references;
DROP INDEX IF EXISTS idx_memory_facts_tags;
DROP INDEX IF EXISTS idx_memory_facts_file_path;
DROP INDEX IF EXISTS idx_memory_facts_importance;
DROP INDEX IF EXISTS idx_memory_facts_subject;
DROP INDEX IF EXISTS idx_memory_facts_category;
DROP INDEX IF EXISTS idx_memory_facts_timestamp;
DROP INDEX IF EXISTS idx_memory_facts_source;
DROP INDEX IF EXISTS idx_memory_facts_vector;
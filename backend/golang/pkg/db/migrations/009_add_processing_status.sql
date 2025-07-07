-- Migration: Add processing status fields to data_sources table
-- +goose Up

-- Add processing status fields to data_sources table
ALTER TABLE data_sources ADD COLUMN processing_status TEXT DEFAULT 'idle' CHECK (processing_status IN ('idle', 'processing', 'indexing'));
ALTER TABLE data_sources ADD COLUMN processing_started_at DATETIME;
ALTER TABLE data_sources ADD COLUMN processing_workflow_id TEXT;

-- Create indexes on processing status fields
CREATE INDEX IF NOT EXISTS idx_data_sources_processing_status ON data_sources(processing_status);
CREATE INDEX IF NOT EXISTS idx_data_sources_processing_workflow ON data_sources(processing_workflow_id);

-- +goose Down

-- Drop the indexes
DROP INDEX IF EXISTS idx_data_sources_processing_workflow;
DROP INDEX IF EXISTS idx_data_sources_processing_status;

-- Drop the columns (Note: SQLite doesn't support DROP COLUMN, so this is a limitation)
-- ALTER TABLE data_sources DROP COLUMN processing_workflow_id;
-- ALTER TABLE data_sources DROP COLUMN processing_started_at;
-- ALTER TABLE data_sources DROP COLUMN processing_status; 
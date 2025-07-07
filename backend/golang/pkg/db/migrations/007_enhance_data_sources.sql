-- Migration: Enhance data_sources table with additional columns
-- +goose Up

-- Add created_at column
ALTER TABLE data_sources ADD COLUMN created_at DATETIME;

-- Add state column
ALTER TABLE data_sources ADD COLUMN state TEXT;

-- Add processing status fields
ALTER TABLE data_sources ADD COLUMN processing_status TEXT DEFAULT 'idle' CHECK (processing_status IN ('idle', 'processing', 'indexing'));
ALTER TABLE data_sources ADD COLUMN processing_started_at DATETIME;
ALTER TABLE data_sources ADD COLUMN processing_workflow_id TEXT;

-- Update existing rows
UPDATE data_sources SET created_at = CURRENT_TIMESTAMP WHERE created_at IS NULL;
UPDATE data_sources SET state = 'active' WHERE state IS NULL;

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_data_sources_created_at ON data_sources(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_data_sources_state ON data_sources(state);
CREATE INDEX IF NOT EXISTS idx_data_sources_path_state ON data_sources(path, state);
CREATE INDEX IF NOT EXISTS idx_data_sources_processing_status ON data_sources(processing_status);
CREATE INDEX IF NOT EXISTS idx_data_sources_processing_workflow ON data_sources(processing_workflow_id);

-- +goose Down

-- Drop the indexes
DROP INDEX IF EXISTS idx_data_sources_processing_workflow;
DROP INDEX IF EXISTS idx_data_sources_processing_status;
DROP INDEX IF EXISTS idx_data_sources_path_state;
DROP INDEX IF EXISTS idx_data_sources_state;
DROP INDEX IF EXISTS idx_data_sources_created_at;

-- Drop the columns (Note: SQLite doesn't support DROP COLUMN in older versions)
-- ALTER TABLE data_sources DROP COLUMN processing_workflow_id;
-- ALTER TABLE data_sources DROP COLUMN processing_started_at;
-- ALTER TABLE data_sources DROP COLUMN processing_status;
-- ALTER TABLE data_sources DROP COLUMN state;
-- ALTER TABLE data_sources DROP COLUMN created_at; 
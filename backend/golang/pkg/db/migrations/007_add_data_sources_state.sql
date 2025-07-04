-- Migration: Add state column to data_sources table
-- +goose Up

-- Add state column to data_sources table without default value first
ALTER TABLE data_sources ADD COLUMN state TEXT;

-- Update existing rows to set state to 'active'
UPDATE data_sources SET state = 'active' WHERE state IS NULL;

-- Create index on state column
CREATE INDEX IF NOT EXISTS idx_data_sources_state ON data_sources(state);

-- Create composite index on path and state
CREATE INDEX IF NOT EXISTS idx_data_sources_path_state ON data_sources(path, state);

-- +goose Down

-- Drop the indexes
DROP INDEX IF EXISTS idx_data_sources_path_state;
DROP INDEX IF EXISTS idx_data_sources_state;

-- Drop the state column
ALTER TABLE data_sources DROP COLUMN state; 
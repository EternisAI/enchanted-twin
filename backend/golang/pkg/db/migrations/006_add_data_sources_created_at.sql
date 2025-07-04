-- Migration: Add created_at column to data_sources table
-- +goose Up

-- Add created_at column to data_sources table without default value first
ALTER TABLE data_sources ADD COLUMN created_at DATETIME;

-- Update existing rows to set created_at to the current timestamp
UPDATE data_sources SET created_at = CURRENT_TIMESTAMP WHERE created_at IS NULL;

-- Create index on created_at column
CREATE INDEX IF NOT EXISTS idx_data_sources_created_at ON data_sources(created_at DESC);

-- +goose Down

-- Drop the index
DROP INDEX IF EXISTS idx_data_sources_created_at;

-- Drop the created_at column
ALTER TABLE data_sources DROP COLUMN created_at; 
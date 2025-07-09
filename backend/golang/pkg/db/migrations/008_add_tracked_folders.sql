-- Migration: Add tracked_folders table
-- +goose Up

-- Create tracked_folders table
CREATE TABLE IF NOT EXISTS tracked_folders (
    id TEXT PRIMARY KEY,
    path TEXT NOT NULL UNIQUE,
    name TEXT,
    is_enabled BOOLEAN DEFAULT TRUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Create index on path for faster lookups
CREATE INDEX IF NOT EXISTS idx_tracked_folders_path ON tracked_folders(path);

-- Create index on enabled status
CREATE INDEX IF NOT EXISTS idx_tracked_folders_enabled ON tracked_folders(is_enabled);

-- +goose Down

-- Drop the indexes
DROP INDEX IF EXISTS idx_tracked_folders_enabled;
DROP INDEX IF EXISTS idx_tracked_folders_path;

-- Drop the table
DROP TABLE IF EXISTS tracked_folders; 
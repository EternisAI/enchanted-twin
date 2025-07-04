-- Migration: Add privacy_dict_json column to chats table
-- +goose Up

-- Add column to store privacy dictionary as JSON
ALTER TABLE chats ADD COLUMN privacy_dict_json TEXT;

-- Create index on privacy_dict_json for potential queries
CREATE INDEX IF NOT EXISTS idx_chats_privacy_dict ON chats(privacy_dict_json) WHERE privacy_dict_json IS NOT NULL;

-- +goose Down

-- Remove index first
DROP INDEX IF EXISTS idx_chats_privacy_dict;

-- Note: SQLite doesn't support DROP COLUMN, so we can't easily revert column additions
-- In a production environment, you might need to recreate tables without these columns
-- For now, we'll leave the columns but document the limitation 
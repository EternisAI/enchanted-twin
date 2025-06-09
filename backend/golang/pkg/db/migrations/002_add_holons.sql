-- Migration: Add holon-related enhancements and additional columns
-- +goose Up

-- Add state column to threads table
ALTER TABLE threads ADD COLUMN state TEXT NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'broadcasted', 'received', 'hidden', 'visible'));

-- Add state column to thread_messages table  
ALTER TABLE thread_messages ADD COLUMN state TEXT NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'broadcasted', 'received', 'hidden', 'visible'));

-- Add remote_thread_id column to threads table for API synchronization
ALTER TABLE threads ADD COLUMN remote_thread_id INTEGER;

-- Create index on remote_thread_id for faster lookups
CREATE INDEX IF NOT EXISTS idx_threads_remote_id ON threads(remote_thread_id);

-- Insert config values for telegram integration
INSERT OR IGNORE INTO config (key, value) VALUES ('telegram_chat_uuid', '');
INSERT OR IGNORE INTO config (key, value) VALUES ('telegram_chat_id', NULL);

-- +goose Down

-- Remove indexes and columns in reverse order
DROP INDEX IF EXISTS idx_threads_remote_id;

-- Note: SQLite doesn't support DROP COLUMN, so we can't easily revert column additions
-- In a production environment, you might need to recreate tables without these columns
-- For now, we'll leave the columns but document the limitation

-- Remove config entries
DELETE FROM config WHERE key IN ('telegram_chat_uuid', 'telegram_chat_id');
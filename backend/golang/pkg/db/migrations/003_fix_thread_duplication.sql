-- Migration: Fix thread duplication by adding unique constraint on remote_thread_id
-- +goose Up

-- Create unique index on remote_thread_id to prevent duplicate threads pointing to the same remote thread
CREATE UNIQUE INDEX IF NOT EXISTS idx_threads_remote_id_unique ON threads(remote_thread_id) WHERE remote_thread_id IS NOT NULL;

-- +goose Down

-- Remove the unique constraint
DROP INDEX IF EXISTS idx_threads_remote_id_unique;
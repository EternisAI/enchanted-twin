-- Migration: Fix thread duplication by adding unique constraint on remote_thread_id
-- +goose Up

-- First, identify and remove duplicate threads with the same remote_thread_id
-- Keep the thread that was created first (oldest created_at) for each remote_thread_id
DELETE FROM threads 
WHERE rowid NOT IN (
    SELECT MIN(rowid) 
    FROM threads 
    WHERE remote_thread_id IS NOT NULL 
    GROUP BY remote_thread_id
);

-- Also clean up any orphaned thread_messages that may reference deleted threads
DELETE FROM thread_messages 
WHERE thread_id NOT IN (SELECT id FROM threads);

-- Now create unique index on remote_thread_id to prevent future duplicates
CREATE UNIQUE INDEX IF NOT EXISTS idx_threads_remote_id_unique ON threads(remote_thread_id) WHERE remote_thread_id IS NOT NULL;

-- +goose Down

-- Remove the unique constraint
DROP INDEX IF EXISTS idx_threads_remote_id_unique;

-- Note: We cannot restore the deleted duplicate threads in the rollback
-- This is a destructive migration that cleans up data inconsistencies
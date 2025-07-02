-- Migration: Add LLM evaluation fields for thread processing decisions
-- +goose Up

-- Add columns to store LLM evaluation results for threads
ALTER TABLE threads ADD COLUMN evaluation_reason TEXT;
ALTER TABLE threads ADD COLUMN evaluation_confidence REAL;
ALTER TABLE threads ADD COLUMN evaluated_at TIMESTAMP;
ALTER TABLE threads ADD COLUMN evaluated_by TEXT DEFAULT 'llm-processor';

-- Create index on evaluation fields for analytics and debugging
CREATE INDEX IF NOT EXISTS idx_threads_evaluation ON threads(evaluation_confidence DESC, evaluated_at DESC);
CREATE INDEX IF NOT EXISTS idx_threads_state_evaluation ON threads(state, evaluation_confidence DESC);

-- +goose Down

-- Remove indexes first
DROP INDEX IF EXISTS idx_threads_state_evaluation;
DROP INDEX IF EXISTS idx_threads_evaluation;

-- Note: SQLite doesn't support DROP COLUMN, so we can't easily revert column additions
-- In a production environment, you might need to recreate tables without these columns
-- For now, we'll leave the columns but document the limitation
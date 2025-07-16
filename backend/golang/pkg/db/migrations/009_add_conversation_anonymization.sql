-- +goose Up
-- +goose StatementBegin

-- Table to store conversation anonymization dictionaries
CREATE TABLE conversation_dicts (
    conversation_id TEXT PRIMARY KEY NOT NULL,
    dict_data TEXT NOT NULL,  -- JSON containing {"TOKEN": "original_value"} mappings
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Table to track which messages have been anonymized in each conversation
CREATE TABLE anonymized_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id TEXT NOT NULL,
    message_hash TEXT NOT NULL,  -- SHA256 hash of normalized message content
    anonymized_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(conversation_id, message_hash),
    FOREIGN KEY (conversation_id) REFERENCES conversation_dicts(conversation_id) ON DELETE CASCADE
);

-- Indexes for performance
CREATE INDEX idx_conversation_messages ON anonymized_messages(conversation_id);
CREATE INDEX idx_message_hash ON anonymized_messages(message_hash);
CREATE INDEX idx_conversation_updated ON conversation_dicts(updated_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Drop indexes first
DROP INDEX IF EXISTS idx_conversation_updated;
DROP INDEX IF EXISTS idx_message_hash;
DROP INDEX IF EXISTS idx_conversation_messages;

-- Drop tables
DROP TABLE IF EXISTS anonymized_messages;
DROP TABLE IF EXISTS conversation_dicts;

-- +goose StatementEnd
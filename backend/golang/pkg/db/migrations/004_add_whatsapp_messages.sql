-- Migration: Add WhatsApp messages table for buffering
-- +goose Up

-- Create WhatsApp messages table for buffering messages before memory storage
CREATE TABLE IF NOT EXISTS whatsapp_messages (
    id TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL,
    sender_jid TEXT NOT NULL,
    sender_name TEXT NOT NULL,
    content TEXT NOT NULL,
    message_type TEXT NOT NULL DEFAULT 'text',
    sent_at TIMESTAMP NOT NULL,
    from_me BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_whatsapp_messages_conversation ON whatsapp_messages(conversation_id);
CREATE INDEX IF NOT EXISTS idx_whatsapp_messages_sent_at ON whatsapp_messages(conversation_id, sent_at ASC);
CREATE INDEX IF NOT EXISTS idx_whatsapp_messages_created ON whatsapp_messages(created_at DESC);

-- +goose Down

-- Remove indexes and table
DROP INDEX IF EXISTS idx_whatsapp_messages_created;
DROP INDEX IF EXISTS idx_whatsapp_messages_sent_at;
DROP INDEX IF EXISTS idx_whatsapp_messages_conversation;
DROP TABLE IF EXISTS whatsapp_messages; 
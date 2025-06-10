-- Migration: Initial schema with user profiles, chats, messages, threads, and data sources
-- +goose Up

-- Create user profiles table
CREATE TABLE IF NOT EXISTS user_profiles (
    id TEXT PRIMARY KEY,
    name TEXT,
    bio TEXT
);

-- Create chats table
CREATE TABLE IF NOT EXISTS chats (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    category TEXT NOT NULL DEFAULT 'TEXT',
    holon_thread_id TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_chats_id ON chats(id);
CREATE INDEX IF NOT EXISTS idx_chats_category ON chats(category);

-- Create messages table
CREATE TABLE IF NOT EXISTS messages (
    id TEXT PRIMARY KEY,
    chat_id TEXT NOT NULL,
    text TEXT,
    tool_calls JSON,
    tool_results JSON,
    image_urls JSON,
    role TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (chat_id) REFERENCES chats(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_messages_chat_id ON messages(chat_id);
CREATE INDEX IF NOT EXISTS idx_messages_chat_created ON messages(chat_id, created_at DESC);

-- Create friend activity tracking table
CREATE TABLE IF NOT EXISTS friend_activity_tracking (
    id TEXT PRIMARY KEY,
    chat_id TEXT NOT NULL,
    activity_type TEXT NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_friend_activity_chat_id ON friend_activity_tracking(chat_id);
CREATE INDEX IF NOT EXISTS idx_friend_activity_timestamp ON friend_activity_tracking(timestamp DESC);

-- Create authors table
CREATE TABLE IF NOT EXISTS authors (
    identity   TEXT PRIMARY KEY,
    alias      TEXT
);

-- Create threads table
CREATE TABLE IF NOT EXISTS threads (
    id               TEXT PRIMARY KEY,
    title            TEXT NOT NULL,
    content          TEXT NOT NULL,
    author_identity  TEXT NOT NULL,
    created_at       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at       TIMESTAMP,
    image_urls       JSON    NOT NULL DEFAULT '[]',
    actions          JSON    NOT NULL DEFAULT '[]',
    views            INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY(author_identity) REFERENCES authors(identity) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_threads_author ON threads(author_identity);

-- Create thread messages table
CREATE TABLE IF NOT EXISTS thread_messages (
    id               TEXT PRIMARY KEY,
    thread_id        TEXT NOT NULL,
    author_identity  TEXT NOT NULL,
    content          TEXT NOT NULL,
    created_at       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    is_delivered     BOOLEAN   NOT NULL DEFAULT FALSE,
    actions          JSON      NOT NULL DEFAULT '[]',
    FOREIGN KEY(thread_id)       REFERENCES threads(id) ON DELETE CASCADE,
    FOREIGN KEY(author_identity) REFERENCES authors(identity) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_thread_messages_thread ON thread_messages(thread_id);
CREATE INDEX IF NOT EXISTS idx_thread_messages_author ON thread_messages(author_identity);
CREATE INDEX IF NOT EXISTS idx_thread_messages_created ON thread_messages(thread_id, created_at DESC);

-- Create holons table
CREATE TABLE IF NOT EXISTS holons (
    id       TEXT PRIMARY KEY, 
    name     TEXT NOT NULL
);

-- Create holon participants table
CREATE TABLE IF NOT EXISTS holon_participants (
    holon_id        TEXT NOT NULL,
    author_identity TEXT NOT NULL,
    joined_at       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (holon_id, author_identity),
    FOREIGN KEY (holon_id)        REFERENCES holons(id)     ON DELETE CASCADE,
    FOREIGN KEY (author_identity) REFERENCES authors(identity) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_holon_participants_holon  ON holon_participants(holon_id);
CREATE INDEX IF NOT EXISTS idx_holon_participants_author ON holon_participants(author_identity);

-- Create data sources table
CREATE TABLE IF NOT EXISTS data_sources (
    id TEXT PRIMARY KEY,
    name TEXT,
    path TEXT,
    processed_path TEXT,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    is_indexed BOOLEAN DEFAULT FALSE,
    has_error BOOLEAN DEFAULT FALSE
);

-- Create config table
CREATE TABLE IF NOT EXISTS config (
    key TEXT PRIMARY KEY,
    value TEXT
);

-- Create source usernames table
CREATE TABLE IF NOT EXISTS source_usernames (
    id TEXT PRIMARY KEY,
    source TEXT NOT NULL,
    username TEXT NOT NULL,
    user_id TEXT,
    first_name TEXT,
    last_name TEXT,
    phone_number TEXT,
    bio TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_source_usernames_source ON source_usernames(source);

-- Create MCP servers table
CREATE TABLE IF NOT EXISTS mcp_servers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    command TEXT NOT NULL,
    args JSON,
    envs JSON,
    type TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    enabled BOOLEAN DEFAULT FALSE
);
CREATE INDEX IF NOT EXISTS idx_mcp_servers_id ON mcp_servers(id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_mcp_servers_name ON mcp_servers(name);

-- Seed data
-- Insert default user profile
INSERT INTO user_profiles (id, name, bio) 
VALUES ('default', '(missing name)', '');

-- Insert default holon network
INSERT INTO holons (id, name) 
VALUES ('holon-default-network', 'HolonNetwork');

-- +goose Down

-- Drop tables in reverse order to handle foreign key constraints
DROP INDEX IF EXISTS idx_mcp_servers_name;
DROP INDEX IF EXISTS idx_mcp_servers_id;
DROP TABLE IF EXISTS mcp_servers;

DROP INDEX IF EXISTS idx_source_usernames_source;
DROP TABLE IF EXISTS source_usernames;

DROP TABLE IF EXISTS config;

DROP TABLE IF EXISTS data_sources;

DROP INDEX IF EXISTS idx_holon_participants_author;
DROP INDEX IF EXISTS idx_holon_participants_holon;
DROP TABLE IF EXISTS holon_participants;

DROP TABLE IF EXISTS holons;

DROP INDEX IF EXISTS idx_thread_messages_created;
DROP INDEX IF EXISTS idx_thread_messages_author;
DROP INDEX IF EXISTS idx_thread_messages_thread;
DROP TABLE IF EXISTS thread_messages;

DROP INDEX IF EXISTS idx_threads_author;
DROP TABLE IF EXISTS threads;

DROP TABLE IF EXISTS authors;

DROP INDEX IF EXISTS idx_friend_activity_timestamp;
DROP INDEX IF EXISTS idx_friend_activity_chat_id;
DROP TABLE IF EXISTS friend_activity_tracking;

DROP INDEX IF EXISTS idx_messages_chat_created;
DROP INDEX IF EXISTS idx_messages_chat_id;
DROP TABLE IF EXISTS messages;

DROP INDEX IF EXISTS idx_chats_category;
DROP INDEX IF EXISTS idx_chats_id;
DROP TABLE IF EXISTS chats;

DROP TABLE IF EXISTS user_profiles;
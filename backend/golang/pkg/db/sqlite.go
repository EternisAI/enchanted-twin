// Owner: johan@eternis.ai
package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

// Wrapper around a SQLite database connection that provides
// functionality specific to Twins.
//
// 1. The creation method creates the tables if they do not exist.
// 2. Convenience methods for querying data.
// 3. Convenience method for inserting and updating data.
type Store struct {
	db *sqlx.DB
}

// NewStore creates a new SQLite-backed store.
func NewStore(ctx context.Context, dbPath string) (*Store, error) {
	// Create the parent directory if it doesn't exist
	dir := filepath.Dir(dbPath)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}
	}

	db, err := sqlx.ConnectContext(ctx, "sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SQLite: %w", err)
	}

	// Enable WAL mode for better concurrency and performance
	_, err = db.ExecContext(ctx, "PRAGMA journal_mode=WAL;")
	if err != nil {
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Enable enforcement of foreign key constraints
	_, err = db.ExecContext(ctx, "PRAGMA foreign_keys=ON;")
	if err != nil {
		return nil, fmt.Errorf("failed to enable foreign key constraints: %w", err)
	}

	// Create user_profiles table if it doesn't exist
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS user_profiles (
			id TEXT PRIMARY KEY,
			name TEXT,
			bio TEXT
		);

		CREATE TABLE IF NOT EXISTS chats (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_chats_id ON chats(id);

		CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			chat_id TEXT NOT NULL,
			text TEXT,
			tool_calls JSON,
			tool_results JSON,
			image_urls JSON,
			role TEXT,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			feedback TEXT,
			FOREIGN KEY (chat_id) REFERENCES chats(id) ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS idx_messages_chat_id ON messages(chat_id);
		CREATE INDEX IF NOT EXISTS idx_messages_chat_created ON messages(chat_id, created_at DESC);

		-- Add feedback column if it doesn't exist
		ALTER TABLE messages ADD COLUMN feedback TEXT;
	`)
	if err != nil {
		return nil, err
	}

	// Create mcp_servers table if it doesn't exist
	_, err = db.Exec(`
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
	`)
	if err != nil {
		return nil, err
	}

	// Insert default profile if it doesn't exist
	_, err = db.ExecContext(ctx, `
		INSERT OR IGNORE INTO user_profiles (id, name, bio) VALUES ('default', '(missing name)', '')
	`)
	if err != nil {
		return nil, err
	}

	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS data_sources (
			id TEXT PRIMARY KEY,
			name TEXT,
			path TEXT,
			processed_path TEXT,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			is_indexed BOOLEAN DEFAULT FALSE,
			has_error BOOLEAN DEFAULT FALSE
		)
	`)
	if err != nil {
		return nil, err
	}

	// Create config table if it doesn't exist
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS config (
			key TEXT PRIMARY KEY,
			value TEXT
		)
	`)
	if err != nil {
		return nil, err
	}

	uuid := uuid.New().String()
	_, err = db.ExecContext(ctx, `
		INSERT OR IGNORE INTO config (key, value) 
		VALUES ('telegram_chat_uuid', ?)
	`, uuid)
	if err != nil {
		return nil, err
	}

	_, err = db.ExecContext(ctx, `
		INSERT OR IGNORE INTO config (key, value) 
		VALUES ('telegram_chat_id', NULL)
	`)
	if err != nil {
		return nil, err
	}

	store := &Store{db: db}

	if err = store.InitOAuth(ctx); err != nil {
		return nil, err
	}

	return store, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// DB returns the underlying sqlx.DB instance.
func (s *Store) DB() *sqlx.DB {
	db := s.db
	return db
}

func (s *Store) GetValue(ctx context.Context, key string) (string, error) {
	var value sql.NullString

	err := s.db.GetContext(ctx, &value, "SELECT value FROM config WHERE key = ?", key)
	if err != nil {
		return "", err
	}
	if !value.Valid {
		return "", nil // Return empty string for NULL values
	}

	return value.String, nil
}

func (s *Store) SetValue(ctx context.Context, key string, value string) error {
	_, err := s.db.ExecContext(
		ctx,
		"INSERT OR REPLACE INTO config (key, value) VALUES (?, ?)",
		key,
		value,
	)
	if err != nil {
		return err
	}
	return nil
}

func (s *Store) GetAllKeys(ctx context.Context) ([]string, error) {
	var keys []string
	err := s.db.SelectContext(ctx, &keys, "SELECT key FROM config")
	if err != nil {
		return nil, err
	}
	return keys, nil
}

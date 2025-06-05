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
			voice BOOLEAN NOT NULL DEFAULT FALSE,
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
			FOREIGN KEY (chat_id) REFERENCES chats(id) ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS idx_messages_chat_id ON messages(chat_id);
		CREATE INDEX IF NOT EXISTS idx_messages_chat_created ON messages(chat_id, created_at DESC);

		CREATE TABLE IF NOT EXISTS friend_activity_tracking (
			id TEXT PRIMARY KEY,
			chat_id TEXT NOT NULL,
			activity_type TEXT NOT NULL,
			timestamp TIMESTAMP NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_friend_activity_chat_id ON friend_activity_tracking(chat_id);
		CREATE INDEX IF NOT EXISTS idx_friend_activity_timestamp ON friend_activity_tracking(timestamp DESC);

		CREATE TABLE IF NOT EXISTS authors (
			identity   TEXT PRIMARY KEY,
			alias      TEXT
      	);

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

		CREATE TABLE IF NOT EXISTS holons (
        	id           TEXT PRIMARY KEY, 
        	name     TEXT NOT NULL
      	);

		INSERT OR IGNORE INTO holons (id, name) VALUES ('holon-default-network', 'HolonNetwork');

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
		CREATE UNIQUE INDEX IF NOT EXISTS idx_mcp_servers_name ON mcp_servers(name);
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

	// Create source_usernames table if it doesn't exist
	_, err = db.ExecContext(ctx, `
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

type SourceUsername struct {
	ID          string  `db:"id" json:"id"`
	Source      string  `db:"source" json:"source"`
	Username    string  `db:"username" json:"username"`
	UserID      *string `db:"user_id" json:"user_id,omitempty"`
	FirstName   *string `db:"first_name" json:"first_name,omitempty"`
	LastName    *string `db:"last_name" json:"last_name,omitempty"`
	PhoneNumber *string `db:"phone_number" json:"phone_number,omitempty"`
	Bio         *string `db:"bio" json:"bio,omitempty"`
	CreatedAt   string  `db:"created_at" json:"created_at"`
	UpdatedAt   string  `db:"updated_at" json:"updated_at"`
}

func (s *Store) SetSourceUsername(ctx context.Context, sourceUsername SourceUsername) error {
	if sourceUsername.ID == "" {
		sourceUsername.ID = uuid.New().String()
	}

	_, err := s.db.ExecContext(
		ctx,
		`INSERT OR REPLACE INTO source_usernames 
		(id, source, username, user_id, first_name, last_name, phone_number, bio, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		sourceUsername.ID,
		sourceUsername.Source,
		sourceUsername.Username,
		sourceUsername.UserID,
		sourceUsername.FirstName,
		sourceUsername.LastName,
		sourceUsername.PhoneNumber,
		sourceUsername.Bio,
	)
	return err
}

func (s *Store) GetSourceUsername(ctx context.Context, source string) (*SourceUsername, error) {
	var sourceUsername SourceUsername
	err := s.db.GetContext(ctx, &sourceUsername, "SELECT * FROM source_usernames WHERE source = ?", source)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &sourceUsername, nil
}

func (s *Store) GetAllSourceUsernames(ctx context.Context) ([]SourceUsername, error) {
	var sourceUsernames []SourceUsername
	err := s.db.SelectContext(ctx, &sourceUsernames, "SELECT * FROM source_usernames ORDER BY source")
	if err != nil {
		return nil, err
	}
	return sourceUsernames, nil
}

func (s *Store) DeleteSourceUsername(ctx context.Context, source string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM source_usernames WHERE source = ?", source)
	return err
}

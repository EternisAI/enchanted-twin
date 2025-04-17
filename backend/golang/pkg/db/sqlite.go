package db

import (
	"context"
	"fmt"

	"github.com/EternisAI/enchanted-twin/graph/model"
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

// NewStore creates a new SQLite-backed store
func NewStore(dbPath string) (*Store, error) {
	db, err := sqlx.Connect("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SQLite: %w", err)
	}

	// Enable WAL mode for better concurrency and performance
	_, err = db.Exec("PRAGMA journal_mode=WAL;")
	if err != nil {
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Create user_profiles table if it doesn't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS user_profiles (
			id TEXT PRIMARY KEY,
			name TEXT
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
			FOREIGN KEY (chat_id) REFERENCES chats(id) ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS idx_messages_chat_id ON messages(chat_id);
	`)
	if err != nil {
		return nil, err
	}

	// Insert default profile if it doesn't exist
	_, err = db.Exec(`
		INSERT OR IGNORE INTO user_profiles (id, name) VALUES ('default', '(missing name)')
	`)
	if err != nil {
		return nil, err
	}

	return &Store{db: db}, nil
}

// GetUserProfile retrieves the user profile
func (s *Store) GetUserProfile(ctx context.Context) (*model.UserProfile, error) {
	var name string
	err := s.db.GetContext(ctx, &name, `SELECT name FROM user_profiles WHERE id = 'default'`)
	if err != nil {
		return nil, err
	}
	return &model.UserProfile{
		Name: &name,
	}, nil
}

// UpdateUserProfile updates the user profile
func (s *Store) UpdateUserProfile(ctx context.Context, input model.UpdateProfileInput) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE user_profiles SET name = ? WHERE id = 'default'
	`, input.Name)
	if err != nil {
		return false, err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return rows > 0, nil
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}

// DB returns the underlying sqlx.DB instance
func (s *Store) DB() *sqlx.DB {
	db := s.db
	return db
}

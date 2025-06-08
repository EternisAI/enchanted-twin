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

	// Schema creation is now handled by the migration system
	// Only database connection setup is done here

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

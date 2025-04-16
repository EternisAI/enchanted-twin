package db

import (
	"context"
	"fmt"

	"github.com/EternisAI/enchanted-twin/graph/model"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sqlx.DB
}

// NewStore creates a new SQLite-backed store
func NewStore(dbPath string) (*Store, error) {
	db, err := sqlx.Connect("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SQLite: %w", err)
	}

	// Create user_profiles table if it doesn't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS user_profiles (
			id TEXT PRIMARY KEY,
			name TEXT
		)
	`)
	if err != nil {
		return nil, err
	}

	// Insert default profile if it doesn't exist
	_, err = db.Exec(`
		INSERT OR IGNORE INTO user_profiles (id, name) VALUES ('default', 'John Doe')
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

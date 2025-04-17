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
		INSERT OR IGNORE INTO user_profiles (id, name) VALUES ('default', '(missing name)')
	`)

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS data_sources (
			id TEXT PRIMARY KEY,
			name TEXT,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			is_indexed BOOLEAN DEFAULT FALSE
		)
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

type DataSource struct {
	ID        string `db:"id"`
	Name      string `db:"name"`
	UpdatedAt string `db:"updated_at"`
	IsIndexed *bool  `db:"is_indexed"`
}

// GetDataSources retrieves all data sources
func (s *Store) GetDataSources(ctx context.Context) ([]*DataSource, error) {
	var dataSources []*DataSource
	err := s.db.SelectContext(ctx, &dataSources, `SELECT id, name, updated_at, is_indexed FROM data_sources`)
	if err != nil {
		return nil, err
	}
	return dataSources, nil
}

func (s *Store) CreateDataSource(ctx context.Context, id string, name string) (*DataSource, error) {
	_, err := s.db.ExecContext(ctx, `INSERT INTO data_sources (id, name) VALUES (?, ?)`, id, name)
	if err != nil {
		return nil, err
	}
	return &DataSource{ID: id, Name: name}, nil
}

func (s *Store) UpdateDataSource(ctx context.Context, id string, isIndexed bool) (*DataSource, error) {
	_, err := s.db.ExecContext(ctx, `UPDATE data_sources SET updated_at = CURRENT_TIMESTAMP, is_indexed = ? WHERE id = ?`, isIndexed, id)
	if err != nil {
		return nil, err
	}
	return &DataSource{ID: id, IsIndexed: &isIndexed}, nil
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}

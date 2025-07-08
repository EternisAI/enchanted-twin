package db

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

type TrackedFolder struct {
	ID        string  `db:"id" json:"id"`
	Path      string  `db:"path" json:"path"`
	Name      *string `db:"name" json:"name,omitempty"`
	IsEnabled bool    `db:"is_enabled" json:"is_enabled"`
	CreatedAt string  `db:"created_at" json:"created_at"`
	UpdatedAt string  `db:"updated_at" json:"updated_at"`
}

type CreateTrackedFolderInput struct {
	Path string
	Name *string
}

// GetTrackedFolders retrieves all tracked folders.
func (s *Store) GetTrackedFolders(ctx context.Context) ([]*TrackedFolder, error) {
	var folders []*TrackedFolder
	err := s.db.SelectContext(ctx, &folders, `
		SELECT id, path, name, is_enabled, created_at, updated_at 
		FROM tracked_folders 
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	return folders, nil
}

// GetEnabledTrackedFolders retrieves all enabled tracked folders.
func (s *Store) GetEnabledTrackedFolders(ctx context.Context) ([]*TrackedFolder, error) {
	var folders []*TrackedFolder
	err := s.db.SelectContext(ctx, &folders, `
		SELECT id, path, name, is_enabled, created_at, updated_at 
		FROM tracked_folders 
		WHERE is_enabled = TRUE
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	return folders, nil
}

// AddTrackedFolder adds a new tracked folder.
func (s *Store) AddTrackedFolder(ctx context.Context, input *CreateTrackedFolderInput) (*TrackedFolder, error) {
	id := uuid.New().String()

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO tracked_folders (id, path, name, is_enabled, created_at, updated_at) 
		VALUES (?, ?, ?, TRUE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, id, input.Path, input.Name)
	if err != nil {
		return nil, err
	}

	return &TrackedFolder{
		ID:        id,
		Path:      input.Path,
		Name:      input.Name,
		IsEnabled: true,
	}, nil
}

// DeleteTrackedFolder removes a tracked folder.
func (s *Store) DeleteTrackedFolder(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM tracked_folders WHERE id = ?`, id)
	return err
}

// UpdateTrackedFolder updates a tracked folder.
func (s *Store) UpdateTrackedFolder(ctx context.Context, id string, name *string, isEnabled bool) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE tracked_folders 
		SET name = ?, is_enabled = ?, updated_at = CURRENT_TIMESTAMP 
		WHERE id = ?
	`, name, isEnabled, id)
	return err
}

// TrackedFolderExistsByPath checks if a tracked folder already exists for the given path.
func (s *Store) TrackedFolderExistsByPath(ctx context.Context, path string) (bool, error) {
	var count int
	err := s.db.GetContext(ctx, &count, `SELECT COUNT(*) FROM tracked_folders WHERE path = ?`, path)
	return count > 0, err
}

// GetTrackedFolderByPath retrieves a tracked folder by path.
func (s *Store) GetTrackedFolderByPath(ctx context.Context, path string) (*TrackedFolder, error) {
	var folder TrackedFolder
	err := s.db.GetContext(ctx, &folder, `
		SELECT id, path, name, is_enabled, created_at, updated_at 
		FROM tracked_folders 
		WHERE path = ?
	`, path)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &folder, nil
}

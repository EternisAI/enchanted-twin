// Owner: johan@eternis.ai
package db

import (
	"context"

	"github.com/google/uuid"
)

type DataSource struct {
	ID            string  `db:"id"`
	Name          string  `db:"name"`
	UpdatedAt     string  `db:"updated_at"`
	CreatedAt     string  `db:"created_at"`
	Path          string  `db:"path"`
	ProcessedPath *string `db:"processed_path"`
	IsIndexed     *bool   `db:"is_indexed"`
	HasError      *bool   `db:"has_error"`
	State         string  `db:"state"`
}

type CreateDataSourceFromFileInput struct {
	Name string
	Path string
}

// GetDataSources retrieves all data sources.
func (s *Store) GetDataSources(ctx context.Context) ([]*DataSource, error) {
	var dataSources []*DataSource
	err := s.db.SelectContext(ctx, &dataSources, `SELECT id, name, updated_at, created_at, path, processed_path, is_indexed, has_error, state FROM data_sources ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	return dataSources, nil
}

// GetUnindexedDataSources retrieves all active data sources that are not indexed.
func (s *Store) GetUnindexedDataSources(ctx context.Context) ([]*DataSource, error) {
	var dataSources []*DataSource
	query := `SELECT id, name, updated_at, created_at, path, processed_path, is_indexed, has_error, state FROM data_sources WHERE has_error = FALSE AND is_indexed = FALSE AND state = 'active' ORDER BY created_at DESC`

	var allDataSources []*DataSource
	err := s.db.SelectContext(ctx, &allDataSources, `SELECT id, name, updated_at, created_at, path, processed_path, is_indexed, has_error, state FROM data_sources ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}

	err = s.db.SelectContext(ctx, &dataSources, query)
	if err != nil {
		return nil, err
	}

	return dataSources, nil
}

// CreateDataSource creates a new data source.
func (s *Store) CreateDataSource(ctx context.Context, id string, name string, path string) (*DataSource, error) {
	_, err := s.db.ExecContext(ctx, `INSERT INTO data_sources (id, name, path) VALUES (?, ?, ?)`, id, name, path)
	if err != nil {
		return nil, err
	}
	return &DataSource{ID: id, Name: name, Path: path}, nil
}

// UpdateDataSource updates a data source.
func (s *Store) UpdateDataSourceState(ctx context.Context, id string, isIndexed bool, hasError bool) (*DataSource, error) {
	_, err := s.db.ExecContext(ctx, `UPDATE data_sources SET updated_at = CURRENT_TIMESTAMP, is_indexed = ?, has_error = ? WHERE id = ?`, isIndexed, hasError, id)
	if err != nil {
		return nil, err
	}
	return &DataSource{ID: id, IsIndexed: &isIndexed, HasError: &hasError}, nil
}

func (s *Store) UpdateDataSourceProcessedPath(ctx context.Context, id string, processedPath string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE data_sources SET updated_at = CURRENT_TIMESTAMP, processed_path = ? WHERE id = ?`, processedPath, id)
	if err != nil {
		return err
	}
	return nil
}

func (s *Store) DeleteDataSourceError(ctx context.Context, id string) (*DataSource, error) {
	_, err := s.db.ExecContext(ctx, `UPDATE data_sources SET has_error = FALSE WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	return &DataSource{ID: id}, nil
}

func (s *Store) DeleteDataSource(ctx context.Context, id string) (*DataSource, error) {
	_, err := s.db.ExecContext(ctx, `DELETE FROM data_sources WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	return &DataSource{ID: id}, nil
}

// DataSourceExistsByPath checks if a data source already exists for the given file path.
func (s *Store) DataSourceExistsByPath(ctx context.Context, path string) (bool, error) {
	var count int
	err := s.db.GetContext(ctx, &count, `SELECT COUNT(*) FROM data_sources WHERE path = ?`, path)
	return count > 0, err
}

// ActiveDataSourceExistsByPath checks if an active data source exists for the given file path.
func (s *Store) ActiveDataSourceExistsByPath(ctx context.Context, path string) (bool, error) {
	var count int
	err := s.db.GetContext(ctx, &count, `SELECT COUNT(*) FROM data_sources WHERE path = ? AND state = 'active'`, path)
	return count > 0, err
}

// MarkDataSourceAsDeleted marks an active data source as deleted.
func (s *Store) MarkDataSourceAsDeleted(ctx context.Context, path string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE data_sources 
		SET state = 'deleted', updated_at = CURRENT_TIMESTAMP 
		WHERE path = ? AND state = 'active'
	`, path)
	return err
}

// MarkDataSourceAsReplaced marks an active data source as replaced.
func (s *Store) MarkDataSourceAsReplaced(ctx context.Context, path string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE data_sources 
		SET state = 'replaced', updated_at = CURRENT_TIMESTAMP 
		WHERE path = ? AND state = 'active'
	`, path)
	return err
}

// UpdateDataSourcePath updates the path of an existing data source.
func (s *Store) UpdateDataSourcePath(ctx context.Context, id string, newPath string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE data_sources 
		SET path = ?, updated_at = CURRENT_TIMESTAMP 
		WHERE id = ?
	`, newPath, id)
	return err
}

// FindOrphanedDataSources finds active data sources whose files no longer exist at their recorded paths.
func (s *Store) FindOrphanedDataSources(ctx context.Context) ([]*DataSource, error) {
	var dataSources []*DataSource
	err := s.db.SelectContext(ctx, &dataSources, `
		SELECT id, name, path, state, created_at, updated_at, is_indexed, has_error, processed_path
		FROM data_sources 
		WHERE state = 'active'
		ORDER BY updated_at DESC
	`)
	if err != nil {
		return nil, err
	}
	return dataSources, nil
}

// GetDataSourceHistory retrieves the complete history of all data sources for a given path.
func (s *Store) GetDataSourceHistory(ctx context.Context, path string) ([]*DataSource, error) {
	var dataSources []*DataSource
	err := s.db.SelectContext(ctx, &dataSources, `
		SELECT id, name, path, state, created_at, updated_at, is_indexed, has_error, processed_path
		FROM data_sources 
		WHERE path = ? 
		ORDER BY created_at DESC
	`, path)
	return dataSources, err
}

// CreateDataSourceFromFile creates a new data source from a file.
func (s *Store) CreateDataSourceFromFile(ctx context.Context, input *CreateDataSourceFromFileInput) (string, error) {
	id := uuid.New().String()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO data_sources (id, name, path, state, created_at, is_indexed, has_error) 
		VALUES (?, ?, ?, 'active', CURRENT_TIMESTAMP, FALSE, FALSE)
	`, id, input.Name, input.Path)
	if err != nil {
		return "", err
	}

	return id, nil
}

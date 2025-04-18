package db

import (
	"context"
)

type DataSource struct {
	ID        string `db:"id"`
	Name      string `db:"name"`
	UpdatedAt string `db:"updated_at"`
	Path      string `db:"path"`
	IsIndexed *bool  `db:"is_indexed"`
}

// GetDataSources retrieves all data sources
func (s *Store) GetDataSources(ctx context.Context) ([]*DataSource, error) {
	var dataSources []*DataSource
	err := s.db.SelectContext(ctx, &dataSources, `SELECT id, name, updated_at, path, is_indexed FROM data_sources`)
	if err != nil {
		return nil, err
	}
	return dataSources, nil
}

// GetUnindexedDataSources retrieves all data sources that are not indexed
func (s *Store) GetUnindexedDataSources(ctx context.Context) ([]*DataSource, error) {
	var dataSources []*DataSource
	err := s.db.SelectContext(ctx, &dataSources, `SELECT id, name, updated_at, path, is_indexed FROM data_sources WHERE is_indexed = FALSE`)
	if err != nil {
		return nil, err
	}
	return dataSources, nil
}

// CreateDataSource creates a new data source
func (s *Store) CreateDataSource(ctx context.Context, id string, name string, path string) (*DataSource, error) {
	_, err := s.db.ExecContext(ctx, `INSERT INTO data_sources (id, name, path) VALUES (?, ?, ?)`, id, name, path)
	if err != nil {
		return nil, err
	}
	return &DataSource{ID: id, Name: name, Path: path}, nil
}

// UpdateDataSource updates a data source
func (s *Store) UpdateDataSource(ctx context.Context, id string, isIndexed bool) (*DataSource, error) {
	_, err := s.db.ExecContext(ctx, `UPDATE data_sources SET updated_at = CURRENT_TIMESTAMP, is_indexed = ? WHERE id = ?`, isIndexed, id)
	if err != nil {
		return nil, err
	}
	return &DataSource{ID: id, IsIndexed: &isIndexed}, nil
}

package repository

import (
	"log/slog"

	"github.com/jmoiron/sqlx"
)

type Repository struct {
	logger *slog.Logger
	db     *sqlx.DB
}

func NewRepository(logger *slog.Logger, db *sqlx.DB) *Repository {
	return &Repository{
		logger: logger,
		db:     db,
	}
}

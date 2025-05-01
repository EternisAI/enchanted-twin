package repository

import (
	"github.com/charmbracelet/log"
	"github.com/jmoiron/sqlx"
)

type Repository struct {
	logger *log.Logger
	db     *sqlx.DB
}

func NewRepository(logger *log.Logger, db *sqlx.DB) *Repository {
	return &Repository{
		logger: logger,
		db:     db,
	}
}

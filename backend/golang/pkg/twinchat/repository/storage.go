package repository

import (
	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/charmbracelet/log"
	"github.com/jmoiron/sqlx"
)

type Repository struct {
	logger *log.Logger
	db     *sqlx.DB
}

func NewRepository(logger *log.Logger, store *db.Store) *Repository {
	return &Repository{
		logger: logger,
		db:     store.DB(),
	}
}

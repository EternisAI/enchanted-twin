package db

import (
	"database/sql"
	"fmt"

	"github.com/charmbracelet/log"
	_ "github.com/mattn/go-sqlite3"

	"github.com/EternisAI/enchanted-twin/pkg/db/sqlc/config"
	"github.com/EternisAI/enchanted-twin/pkg/db/sqlc/holons"
	"github.com/EternisAI/enchanted-twin/pkg/db/sqlc/whatsapp"
)

// DB wraps the database connection and provides additional functionality.
type DB struct {
	*sql.DB
	ConfigQueries   *config.Queries
	HolonsQueries   *holons.Queries
	WhatsappQueries *whatsapp.Queries
	logger          *log.Logger
}

// New creates a new database connection.
func New(sqlDB *sql.DB, logger *log.Logger) (*DB, error) {
	// Test the connection
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("Successfully connected to database")

	// Run migrations automatically
	if err := RunMigrations(sqlDB, logger); err != nil {
		return nil, fmt.Errorf("failed to run database migrations: %w", err)
	}

	// Create queries instances
	configQueries := config.New(sqlDB)
	holonsQueries := holons.New(sqlDB)
	whatsappQueries := whatsapp.New(sqlDB)

	return &DB{
		DB:              sqlDB,
		ConfigQueries:   configQueries,
		HolonsQueries:   holonsQueries,
		WhatsappQueries: whatsappQueries,
		logger:          logger,
	}, nil
}

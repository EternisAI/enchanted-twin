package db

import (
	"database/sql"
	"fmt"

	"github.com/EternisAI/enchanted-twin/pkg/db/sqlc/config"
	"github.com/EternisAI/enchanted-twin/pkg/db/sqlc/holons"
	"github.com/charmbracelet/log"

	_ "github.com/lib/pq"

	"go.uber.org/zap"
)

// DB wraps the database connection and provides additional functionality
type DB struct {
	*sql.DB
	ConfigQueries *config.Queries
	HolonsQueries *holons.Queries
	logger        *log.Logger
}

// New creates a new database connection
func New(sqlDB *sql.DB, logger *log.Logger) (*DB, error) {
	// Test the connection
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("Successfully connected to database")

	// Run migrations automatically
	logger.Info("Running database migrations...")
	if err := RunMigrations(sqlDB); err != nil {
		logger.Error("Failed to run database migrations", zap.Error(err))
		return nil, fmt.Errorf("failed to run database migrations: %w", err)
	}
	logger.Info("Database migrations completed successfully")

	// Create queries instances
	configQueries := config.New(sqlDB)
	holonsQueries := holons.New(sqlDB)

	return &DB{
		DB:            sqlDB,
		ConfigQueries: configQueries,
		HolonsQueries: holonsQueries,
		logger:        logger,
	}, nil
}

// Health checks if the database connection is healthy
func (db *DB) Health() error {
	return db.Ping()
}

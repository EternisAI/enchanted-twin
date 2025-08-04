package db

import (
	"database/sql"
	"embed"
	"fmt"
	logstd "log"
	"os"

	"github.com/charmbracelet/log"
	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

// RunMigrations runs all pending migrations automatically.
func RunMigrations(db *sql.DB, logger *log.Logger) error {
	// Set up goose to use a logger that writes to a discarded output
	// to suppress default goose output, since we'll handle logging ourselves
	goose.SetLogger(logstd.New(os.Stderr, "", 0))
	goose.SetBaseFS(embedMigrations)

	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	logger.Info("Running database migrations...")
	err := goose.Up(db, "migrations")
	if err != nil {
		logger.Error("Database migrations failed", "error", err)
		return err
	}
	logger.Info("Database migrations completed successfully")
	return nil
}

package db

import (
	"database/sql"
	"embed"
	"fmt"
	logstd "log"
	"strings"

	"github.com/charmbracelet/log"
	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

// gooseLogWriter wraps goose output with our structured logger
type gooseLogWriter struct {
	logger *log.Logger
}

func (w *gooseLogWriter) Write(p []byte) (n int, err error) {
	msg := strings.TrimSpace(string(p))
	if msg != "" {
		w.logger.Debug("goose", "message", msg)
	}
	return len(p), nil
}

// RunMigrations runs all pending migrations automatically.
func RunMigrations(db *sql.DB, logger *log.Logger) error {
	// Wrap goose logging with our structured logger
	goose.SetLogger(logstd.New(&gooseLogWriter{logger: logger}, "", 0))
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

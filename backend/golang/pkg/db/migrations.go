package db

import (
	"database/sql"
	"embed"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/log"
	"go.uber.org/zap"

	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

// Migration represents a database migration
type Migration struct {
	Version int
	Name    string
	SQL     string
}

// Migrator handles database migrations
type Migrator struct {
	db     *sql.DB
	logger *log.Logger
}

// NewMigrator creates a new migrator instance
func NewMigrator(db *sql.DB, logger *log.Logger) *Migrator {
	return &Migrator{
		db:     db,
		logger: logger,
	}
}

// RunMigrations runs all pending migrations
func (m *Migrator) RunMigrations() error {
	m.logger.Info("Running database migrations...")

	// Create migrations table if it doesn't exist
	if err := m.createMigrationsTable(); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Load migration files
	migrations, err := m.loadMigrations()
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	// Get current version
	currentVersion, err := m.getCurrentVersion()
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	// Run pending migrations
	for _, migration := range migrations {
		if migration.Version > currentVersion {
			if err := m.runMigration(migration); err != nil {
				return fmt.Errorf("failed to run migration %d: %w", migration.Version, err)
			}
		}
	}

	m.logger.Info("Migrations completed successfully!")
	return nil
}

// RunMigrations runs all pending migrations automatically
func RunMigrations(db *sql.DB) error {
	goose.SetBaseFS(embedMigrations)

	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	return goose.Up(db, "migrations")
}

// GetMigrationStatus returns the current migration status
func GetMigrationStatus(db *sql.DB) error {
	goose.SetBaseFS(embedMigrations)

	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	return goose.Status(db, "migrations")
}

// Rollback rolls back the last migration
func Rollback(db *sql.DB) error {
	goose.SetBaseFS(embedMigrations)

	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	return goose.Down(db, "migrations")
}

// GetVersion returns the current migration version
func GetVersion(db *sql.DB) (int64, error) {
	goose.SetBaseFS(embedMigrations)

	if err := goose.SetDialect("sqlite3"); err != nil {
		return 0, fmt.Errorf("failed to set goose dialect: %w", err)
	}

	return goose.GetDBVersion(db)
}

// createMigrationsTable creates the migrations table if it doesn't exist
func (m *Migrator) createMigrationsTable() error {
	_, err := m.db.Exec(`
		CREATE TABLE IF NOT EXISTS migrations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			version INTEGER NOT NULL,
			name TEXT NOT NULL,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`)
	return err
}

// loadMigrations loads migration files from the embedded filesystem
func (m *Migrator) loadMigrations() ([]Migration, error) {
	files, err := embedMigrations.ReadDir("migrations")
	if err != nil {
		return nil, err
	}

	var migrations []Migration
	for _, file := range files {
		if !file.IsDir() {
			version, name, err := parseMigrationFileName(file.Name())
			if err != nil {
				return nil, err
			}

			sqlBytes, err := embedMigrations.ReadFile(filepath.Join("migrations", file.Name()))
			if err != nil {
				return nil, err
			}

			migrations = append(migrations, Migration{
				Version: version,
				Name:    name,
				SQL:     string(sqlBytes),
			})
		}
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

// parseMigrationFileName parses the migration file name and extracts the version and name
func parseMigrationFileName(fileName string) (version int, name string, err error) {
	parts := strings.SplitN(fileName, "_", 2)
	if len(parts) != 2 {
		return 0, "", fmt.Errorf("invalid migration file name format: %s", fileName)
	}

	versionStr := parts[0]
	name = parts[1]

	version, err = strconv.Atoi(versionStr)
	if err != nil {
		return 0, "", fmt.Errorf("failed to parse migration version: %w", err)
	}

	return version, name, nil
}

// getCurrentVersion retrieves the current migration version from the database
func (m *Migrator) getCurrentVersion() (int, error) {
	var version int
	err := m.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM migrations").Scan(&version)
	return version, err
}

// runMigration executes a single migration
func (m *Migrator) runMigration(migration Migration) error {
	m.logger.Info("Applying migration", zap.Int("version", migration.Version), zap.String("name", migration.Name))

	_, err := m.db.Exec(migration.SQL)
	if err != nil {
		return fmt.Errorf("failed to execute migration SQL: %w", err)
	}

	_, err = m.db.Exec("INSERT INTO migrations (version, name) VALUES ($1, $2)", migration.Version, migration.Name)
	if err != nil {
		return fmt.Errorf("failed to record migration in the database: %w", err)
	}

	m.logger.Info("Migration applied successfully", zap.Int("version", migration.Version), zap.String("name", migration.Name))
	return nil
}

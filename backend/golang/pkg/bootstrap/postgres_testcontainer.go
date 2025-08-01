package bootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

// PostgresTestContainer manages a PostgreSQL testcontainer with pgvector support.
type PostgresTestContainer struct {
	container testcontainers.Container
	logger    *log.Logger
	db        *sql.DB
	port      string
}

// SetupPostgresTestContainer creates and starts a PostgreSQL container with pgvector extension.
func SetupPostgresTestContainer(ctx context.Context, logger *log.Logger) (*PostgresTestContainer, error) {
	// Use pgvector/pgvector image which includes PostgreSQL with pgvector extension
	req := testcontainers.ContainerRequest{
		Image:        "pgvector/pgvector:pg16",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_DB":       "postgres",
			"POSTGRES_USER":     "postgres",
			"POSTGRES_PASSWORD": "password",
		},
		WaitingFor: wait.ForAll(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2). // PostgreSQL logs this twice during startup
				WithStartupTimeout(2*time.Minute),
			wait.ForListeningPort("5432/tcp").WithStartupTimeout(2*time.Minute),
		),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start PostgreSQL container: %w", err)
	}

	// Get the mapped port
	host, err := container.Host(ctx)
	if err != nil {
		container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get container host: %w", err)
	}

	mappedPort, err := container.MappedPort(ctx, "5432")
	if err != nil {
		container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get mapped port: %w", err)
	}

	port := mappedPort.Port()
	connStr := fmt.Sprintf("postgres://postgres:password@%s:%s/postgres?sslmode=disable", host, port)

	// Wait a bit more for PostgreSQL to be fully ready
	time.Sleep(2 * time.Second)

	// Create database connection
	config, err := pgx.ParseConfig(connStr)
	if err != nil {
		container.Terminate(ctx)
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	db := stdlib.OpenDB(*config)
	if err := db.Ping(); err != nil {
		container.Terminate(ctx)
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Enable pgvector extension
	if _, err := db.Exec("CREATE EXTENSION IF NOT EXISTS vector;"); err != nil {
		logger.Warn("Failed to create pgvector extension, continuing without it", "error", err)
	} else {
		logger.Info("pgvector extension enabled successfully")
	}

	ptc := &PostgresTestContainer{
		container: container,
		logger:    logger,
		db:        db,
		port:      port,
	}

	logger.Info("PostgreSQL testcontainer ready", 
		"host", host, 
		"port", port, 
		"database", "postgres")

	return ptc, nil
}

// GetDB returns the database connection.
func (ptc *PostgresTestContainer) GetDB() *sql.DB {
	return ptc.db
}

// GetPort returns the mapped port.
func (ptc *PostgresTestContainer) GetPort() string {
	return ptc.port
}

// GetConnectionString returns the full connection string.
func (ptc *PostgresTestContainer) GetConnectionString(ctx context.Context) (string, error) {
	host, err := ptc.container.Host(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get container host: %w", err)
	}
	return fmt.Sprintf("postgres://postgres:password@%s:%s/postgres?sslmode=disable", host, ptc.port), nil
}

// HasPgvector checks if the pgvector extension is available.
func (ptc *PostgresTestContainer) HasPgvector() bool {
	var exists bool
	err := ptc.db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'vector');").Scan(&exists)
	return err == nil && exists
}

// Cleanup terminates the container and closes the database connection.
func (ptc *PostgresTestContainer) Cleanup(ctx context.Context) error {
	if ptc.db != nil {
		ptc.db.Close()
	}
	
	if ptc.container != nil {
		ptc.logger.Info("Terminating PostgreSQL container")
		if err := ptc.container.Terminate(ctx); err != nil {
			return fmt.Errorf("failed to terminate container: %w", err)
		}
	}
	
	ptc.logger.Info("PostgreSQL testcontainer cleaned up successfully")
	return nil
}

// IsHealthy checks if the container and database are healthy.
func (ptc *PostgresTestContainer) IsHealthy(ctx context.Context) error {
	if ptc.container == nil || ptc.db == nil {
		return fmt.Errorf("container or database not initialized")
	}

	// Check container state
	state, err := ptc.container.State(ctx)
	if err != nil {
		return fmt.Errorf("failed to get container state: %w", err)
	}
	if !state.Running {
		return fmt.Errorf("container is not running")
	}

	// Check database connection
	if err := ptc.db.Ping(); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	return nil
}
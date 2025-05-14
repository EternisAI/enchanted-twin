package container

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	charmlog "github.com/charmbracelet/log"
	"github.com/pkg/errors"
)

const (
	// DefaultPostgresImage is the default PostgreSQL image URL.
	DefaultPostgresImage = "pgvector/pgvector:pg17"

	// DefaultPostgresContainerName is the default name for the PostgreSQL container.
	DefaultPostgresContainerName = "enchanted-twin-postgres-podman"

	// DefaultPostgresPort is the default port PostgreSQL listens on.
	DefaultPostgresPort = "5432"
)

// PostgresOptions represents configuration options for a PostgreSQL container.
type PostgresOptions struct {
	ImageURL      string // Image URL (default: "pgvector/pgvector:pg17")
	ContainerName string // Container name (default: "enchanted-twin-postgres-podman")
	Port          string // Host port to bind to (default: "5432")
	DataPath      string // Host path to store PostgreSQL data
	User          string // PostgreSQL user (default: "postgres")
	Password      string // Password for the PostgreSQL user (default: "postgres")
	Database      string // Default database to create (default: "postgres")
}

// DefaultPostgresOptions returns a PostgresOptions struct with default values.
func DefaultPostgresOptions() PostgresOptions {
	// Use standard user config directory for data path
	var baseDataDir string
	configDir, err := os.UserConfigDir()
	if err != nil {
		// Fallback if config directory cannot be determined: use home directory
		homeDir, err := os.UserHomeDir()
		if err != nil {
			// Final fallback: use relative path
			baseDataDir = "."
		} else {
			// Use hidden directory in home as fallback
			baseDataDir = filepath.Join(homeDir, ".enchanted")
		}
	} else {
		baseDataDir = configDir
	}

	dataPath := filepath.Join(baseDataDir, "enchanted", "db", "postgres-podman-data")

	return PostgresOptions{
		ImageURL:      DefaultPostgresImage,
		ContainerName: DefaultPostgresContainerName,
		Port:          DefaultPostgresPort,
		DataPath:      dataPath,
		User:          "postgres",
		Password:      "postgres",
		Database:      "postgres",
	}
}

// PostgresManager provides specialized functions for handling PostgreSQL containers.
type PostgresManager struct {
	manager ContainerManager
	options PostgresOptions
	logger  *charmlog.Logger
}

// NewPostgresManager creates a new PostgresManager with the given options.
func NewPostgresManager(logger *charmlog.Logger, options PostgresOptions, containerRuntime string) *PostgresManager {
	// Merge with defaults for any unset fields
	defaults := DefaultPostgresOptions()

	if options.ImageURL == "" {
		options.ImageURL = defaults.ImageURL
	}
	if options.ContainerName == "" {
		options.ContainerName = defaults.ContainerName
	}
	if options.Port == "" {
		options.Port = defaults.Port
	}
	if options.DataPath == "" {
		options.DataPath = defaults.DataPath
	}
	if options.User == "" {
		options.User = defaults.User
	}
	if options.Password == "" {
		options.Password = defaults.Password
	}
	if options.Database == "" {
		options.Database = defaults.Database
	}

	return &PostgresManager{
		manager: NewManager(containerRuntime),
		options: options,
		logger:  logger,
	}
}

// Returns: containerID string and error.
func (p *PostgresManager) StartPostgresContainer(ctx context.Context) (string, error) {
	running, err := p.manager.IsMachineRunning(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to verify if Podman is running")
	}

	if !running {
		return "", errors.New("podman machine is not running")
	}

	if err := os.MkdirAll(p.options.DataPath, 0o755); err != nil {
		return "", errors.Wrap(err, "failed to create data directory")
	}

	containerExists, containerID, err := p.manager.CheckContainerExists(ctx, p.options.ContainerName)
	if err != nil {
		p.logger.Warn("Failed to check if container exists: ", err)
	}

	if containerExists {
		p.logger.Info("PostgreSQL container already exists with ID: ", containerID)

		// Force remove the existing container to ensure a clean state
		p.logger.Info("Removing existing PostgreSQL container to ensure clean state")
		err = p.manager.RemoveContainer(ctx, containerID)
		if err != nil {
			p.logger.Warn("Failed to remove existing container: ", err)
		}
	}

	initScriptPath := filepath.Join(p.options.DataPath, "init.sql")
	err = CreateInitScript(initScriptPath, p.options.User, p.options.Database)
	if err != nil {
		p.logger.Warn("Failed to create init script: ", err)
	}

	containerConfig := ContainerConfig{
		ImageURL: p.options.ImageURL,
		Name:     p.options.ContainerName,
		Ports:    map[string]string{p.options.Port: "5432"},
		Volumes: map[string]string{
			p.options.DataPath: "/var/lib/postgresql/data",
			initScriptPath:     "/docker-entrypoint-initdb.d/init.sql",
		},
		Environment: map[string]string{
			"POSTGRES_PASSWORD":         p.options.Password,
			"POSTGRES_USER":             p.options.User,
			"POSTGRES_DB":               "postgres",
			"POSTGRES_HOST_AUTH_METHOD": "trust",
		},
		PullIfNeeded: true,
	}

	p.logger.Info("Starting PostgreSQL container with standard configuration")
	containerID, err = p.manager.RunContainer(ctx, containerConfig)
	if err != nil {
		return "", errors.Wrap(err, "failed to run PostgreSQL container")
	}

	p.logger.Info("Waiting for PostgreSQL to initialize...")
	time.Sleep(10 * time.Second)

	p.logger.Info("Setting up PostgreSQL for external access")
	if err := p.SetupPostgreSQL(ctx); err != nil {
		p.logger.Error("Failed to set up PostgreSQL", "error", err)
		return containerID, nil
	}

	p.logger.Info("PostgreSQL container started successfully", "containerId", containerID, "port", p.options.Port)
	return containerID, nil
}

// createInitScript creates an SQL init script that will run when the container starts.
func CreateInitScript(path string, user string, dbName string) error {
	script := fmt.Sprintf(`
-- Make the user a superuser to avoid permission issues
ALTER USER %s WITH SUPERUSER;

-- Create the database if it doesn't exist
CREATE DATABASE %s;

-- Connect to the new database
\\c %s;

-- Create extension if available
CREATE EXTENSION IF NOT EXISTS vector;

-- Grant all privileges
GRANT ALL PRIVILEGES ON DATABASE %s TO %s;
`, user, dbName, dbName, dbName, user)

	return os.WriteFile(path, []byte(script), 0o644)
}

// setupPostgreSQL performs the necessary setup for PostgreSQL.
func (p *PostgresManager) SetupPostgreSQL(ctx context.Context) error {
	// Wait for PostgreSQL to be ready first
	if err := p.WaitForReady(ctx, 30*time.Second); err != nil {
		return errors.Wrap(err, "PostgreSQL failed to become ready")
	}

	// Configure PostgreSQL for external access
	p.configurePostgres(ctx)

	// Verify database exists
	p.logger.Info("Verifying database exists", "database", "enchanted_twin")
	output, err := p.ExecuteSQLCommand(ctx, fmt.Sprintf("SELECT 1 FROM pg_database WHERE datname = '%s'", "enchanted_twin"))
	if err != nil || !strings.Contains(output, "1 row") {
		p.logger.Warn("Database doesn't appear to exist, attempting to create it manually", "error", err)

		// If init script didn't create database, create it manually
		createDbCmd := fmt.Sprintf("CREATE DATABASE %s WITH OWNER = %s ENCODING = 'UTF8';",
			"enchanted_twin", p.options.User)
		_, createErr := p.ExecuteSQLCommand(ctx, createDbCmd)
		if createErr != nil {
			p.logger.Error("Failed to create database manually", "error", createErr)
			// Try to make the user a superuser
			_, _ = p.ExecuteSQLCommand(ctx, fmt.Sprintf("ALTER USER %s WITH SUPERUSER;", p.options.User))
			// Try again
			_, createErr = p.ExecuteSQLCommand(ctx, createDbCmd)
			if createErr != nil {
				p.logger.Error("Failed to create database even after superuser grant", "error", createErr)
				return errors.Wrap(createErr, "failed to create database after multiple attempts")
			}
		}
	}

	// Verify the database was created successfully
	p.logger.Info("Verifying database is accessible")
	testArgs := []string{
		"exec",
		p.options.ContainerName,
		"psql",
		"-U", p.options.User,
		"-d", "enchanted_twin",
		"-c", "SELECT 1",
	}

	testCtx, cancel := context.WithTimeout(ctx, p.manager.DefaultTimeout())
	defer cancel()

	output, err = p.manager.ExecCommand(testCtx, p.manager.Executable(), testArgs)
	if err != nil {
		p.logger.Error("Failed to verify database access", "error", err, "output", output)
		return errors.Wrap(err, "failed to verify database access")
	}

	p.logger.Info("Database verified and ready for connections")
	return nil
}

// createDatabase creates the specified database and ensures it has proper permissions.
func (p *PostgresManager) createDatabase(ctx context.Context, dbName string) error {
	p.logger.Info("Creating database", "database", dbName)

	// Drop database if it exists
	dropCmd := fmt.Sprintf("DROP DATABASE IF EXISTS %s;", dbName)
	_, err := p.ExecuteSQLCommand(ctx, dropCmd)
	if err != nil {
		p.logger.Warn("Failed to drop database if it exists", "error", err)
		// Continue anyway
	}

	// Create the specified database
	createCmd := fmt.Sprintf("CREATE DATABASE %s WITH OWNER = %s ENCODING = 'UTF8';",
		dbName, p.options.User)
	_, err = p.ExecuteSQLCommand(ctx, createCmd)
	if err != nil {
		return errors.Wrap(err, "failed to create database")
	}

	// Grant all privileges
	grantCmd := fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE %s TO %s;",
		dbName, p.options.User)
	_, err = p.ExecuteSQLCommand(ctx, grantCmd)
	if err != nil {
		p.logger.Warn("Failed to grant privileges", "error", err)
	}

	// Create pg_vector extension if available
	extCmd := fmt.Sprintf("\\c %s; CREATE EXTENSION IF NOT EXISTS vector;", dbName)
	_, err = p.ExecuteSQLCommand(ctx, extCmd)
	if err != nil {
		p.logger.Warn("Failed to create vector extension - this is expected if using standard PostgreSQL", "error", err)
	}

	p.logger.Info("Database created successfully", "database", dbName)
	return nil
}

// EnsureDatabase ensures a database exists in PostgreSQL, creating it if it doesn't.
func (p *PostgresManager) EnsureDatabase(ctx context.Context, dbName string) error {
	p.logger.Info("Database should already be set up - verifying existence")

	output, err := p.ExecuteSQLCommand(ctx, fmt.Sprintf("SELECT 1 FROM pg_database WHERE datname = '%s'", dbName))

	if err == nil && strings.Contains(output, "1 row") {
		p.logger.Info("Database exists as expected", "database", dbName)

		testArgs := []string{
			"exec",
			p.options.ContainerName,
			"psql",
			"-U", p.options.User,
			"-d", dbName,
			"-c", "SELECT 1",
		}

		testCtx, cancel := context.WithTimeout(ctx, p.manager.DefaultTimeout())
		defer cancel()

		output, err := p.manager.ExecCommand(testCtx, p.manager.Executable(), testArgs)
		if err != nil {
			p.logger.Error("Database exists but cannot connect to it", "error", err)
			// Attempt to recreate
			return p.createDatabase(ctx, dbName)
		}

		p.logger.Info("Database connection verified", "database", dbName, "output", output)
		return nil
	}

	// If we get here, the database doesn't exist - recreate it
	p.logger.Warn("Database doesn't exist as expected - recreating", "database", dbName)
	return p.createDatabase(ctx, dbName)
}

// WaitForReady waits for PostgreSQL to be ready to accept connections.
func (p *PostgresManager) WaitForReady(ctx context.Context, maxWaitTime time.Duration) error {
	p.logger.Info("Waiting for PostgreSQL to be ready")

	// Set up timeout
	timeout := time.Now().Add(maxWaitTime)
	attemptCount := 0

	for time.Now().Before(timeout) {
		attemptCount++
		// Check PostgreSQL logs to diagnose startup issues
		if attemptCount%5 == 0 {
			p.logger.Info("Checking PostgreSQL logs to diagnose startup issues")
			logsCmd := []string{
				"logs",
				p.options.ContainerName,
			}

			logOutput, _ := p.manager.ExecCommand(ctx, p.manager.Executable(), logsCmd)
			p.logger.Debug("PostgreSQL logs", "logs", logOutput)
		}

		// Try to execute a simple query
		p.logger.Debug("Executing connection test", "attempt", attemptCount)
		output, err := p.ExecuteSQLCommand(ctx, "SELECT 1")

		if err == nil && strings.Contains(output, "1 row") {
			p.logger.Info("PostgreSQL is ready")
			return nil
		}

		if err != nil {
			p.logger.Debug("Connection test failed", "error", err, "output", output)
		}

		// Sleep before retrying
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}

		// Break after reasonable number of attempts
		if attemptCount > 30 {
			p.logger.Warn("Maximum number of connection attempts reached", "attempts", attemptCount)
			// Check container status
			statusCmd := []string{
				"container", "inspect",
				"--format", "{{.State.Status}}",
				p.options.ContainerName,
			}
			statusOutput, _ := p.manager.ExecCommand(ctx, p.manager.Executable(), statusCmd)
			p.logger.Error("Container status", "status", statusOutput)

			return fmt.Errorf("PostgreSQL not ready after %d attempts: latest container status: %s",
				attemptCount, statusOutput)
		}
	}

	return fmt.Errorf("timeout waiting for PostgreSQL to be ready after %d attempts", attemptCount)
}

// ExecuteSQLCommand executes a SQL command in the PostgreSQL container.
func (p *PostgresManager) ExecuteSQLCommand(ctx context.Context, command string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, p.manager.DefaultTimeout())
	defer cancel()

	cmd := p.manager.Executable()
	args := []string{
		"exec",
		p.options.ContainerName,
		"psql",
		"-U", p.options.User,
		"-d", p.options.Database,
		"-c", command,
	}

	execCmd := cmd + " " + strings.Join(args, " ")
	p.logger.Debug("Executing command", "command", execCmd)

	output, err := p.manager.ExecCommand(ctx, cmd, args)
	if err != nil {
		return "", errors.Wrap(err, "failed to execute SQL command")
	}

	return output, nil
}

// configurePostgres configures PostgreSQL to accept external connections.
func (p *PostgresManager) configurePostgres(ctx context.Context) {
	p.logger.Info("Configuring PostgreSQL for external connections")

	// Configure PostgreSQL to accept connections from all addresses
	cmds := []string{
		"ALTER SYSTEM SET listen_addresses = '*';",
		"ALTER SYSTEM SET log_connections = 'on';",
		"ALTER SYSTEM SET log_disconnections = 'on';",
	}

	for _, cmd := range cmds {
		_, err := p.ExecuteSQLCommand(ctx, cmd)
		if err != nil {
			p.logger.Warn("PostgreSQL configuration failed", "command", cmd, "error", err)
		}
	}

	// Reload configuration
	_, err := p.ExecuteSQLCommand(ctx, "SELECT pg_reload_conf();")
	if err != nil {
		p.logger.Warn("Failed to reload PostgreSQL configuration", "error", err)
	}

	// Verify configuration
	checks := []string{
		"SHOW listen_addresses;",
		"SHOW log_connections;",
		"SHOW log_disconnections;",
	}

	for _, check := range checks {
		output, err := p.ExecuteSQLCommand(ctx, check)
		if err != nil {
			p.logger.Warn("Failed to verify PostgreSQL configuration", "command", check, "error", err)
		} else {
			p.logger.Debug("PostgreSQL configuration", "command", check, "output", output)
		}
	}
}

// GetConnectionString returns the connection string for PostgreSQL.
func (p *PostgresManager) GetConnectionString(dbName string) string {
	// If dbName is empty, use the default database
	if dbName == "" {
		dbName = p.options.Database
	}

	return fmt.Sprintf("postgres://%s:%s@localhost:%s/%s?sslmode=disable",
		p.options.User, p.options.Password, p.options.Port, dbName)
}

// TestDbConnection tests connectivity to the given PostgreSQL connection string.
// It lists tables in the connected database to verify read access and provides
// diagnostic logging. It retries a few times with exponential back-off before
// returning an error.
func TestDbConnection(ctx context.Context, connString string, logger *charmlog.Logger) error {
	logger.Info("Attempting direct database connection", "connectionString", connString)

	const maxAttempts = 5
	for i := 0; i < maxAttempts; i++ {
		db, err := sql.Open("postgres", connString)
		if err == nil {
			defer db.Close() //nolint:errcheck

			pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err = db.PingContext(pingCtx)
			cancel()
			if err == nil {
				logger.Info("Direct database connection successful")

				var result int
				if err := db.QueryRow("SELECT 1").Scan(&result); err == nil && result == 1 {
					logger.Info("Database query successful", "result", result)

					listQuery := `SELECT table_name FROM information_schema.tables WHERE table_schema='public' ORDER BY table_name`
					rows, qErr := db.Query(listQuery)
					if qErr != nil {
						logger.Warn("Failed to list tables", "error", qErr)
					} else {
						defer rows.Close() //nolint:errcheck
						var tables []string
						for rows.Next() {
							var tbl string
							if rows.Scan(&tbl) == nil {
								tables = append(tables, tbl)
							}
						}
						if len(tables) == 0 {
							logger.Info("No tables found in database (empty database)")
						} else {
							logger.Info("Tables in database", "count", len(tables), "tables", tables)
						}
					}
					return nil
				}
			} else {
				logger.Warn("Database ping failed", "error", err)
				if strings.Contains(err.Error(), "does not exist") {
					logger.Error("Database in connection string does not exist")
				}
			}
		} else {
			logger.Warn("Failed to open database connection", "error", err)
		}

		wait := time.Duration(1<<uint(i)) * time.Second
		logger.Warn("Database connection attempt failed, retrying", "attempt", i+1, "max", maxAttempts, "wait", wait)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}

	return fmt.Errorf("unable to connect to database after %d attempts", maxAttempts)
}

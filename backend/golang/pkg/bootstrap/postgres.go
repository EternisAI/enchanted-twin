package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/EternisAI/enchanted-twin/internal/service/docker"
)

// PostgresOptions represents configuration options for a PostgreSQL container
type PostgresOptions struct {
	Version       string // PostgreSQL version tag (default: "17")
	Port          string // Host port to bind to (default: "5432")
	DataPath      string // Host path to store PostgreSQL data (default: "./postgres-data")
	Password      string // Password for the PostgreSQL user (default: "postgres")
	User          string // PostgreSQL user (default: "postgres")
	Database      string // Default database to create (default: "postgres")
	ContainerName string // Container name (default: "enchanted-twin-postgres")
}

// PostgresService manages a PostgreSQL Docker container
type PostgresService struct {
	dockerService *docker.Service
	options       PostgresOptions
	logger        *slog.Logger
}

// DefaultPostgresOptions returns a PostgresOptions struct with default values
func DefaultPostgresOptions() PostgresOptions {
	// Get current working directory for default data path
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	return PostgresOptions{
		Version:       "17",
		Port:          "",
		DataPath:      filepath.Join(cwd, "data", "postgres"),
		User:          "postgres",
		Password:      "postgres",
		Database:      "postgres",
		ContainerName: "enchanted-twin-postgres",
	}
}

// NewPostgresService creates a new PostgreSQL service with sensible defaults
func NewPostgresService(logger *slog.Logger, options PostgresOptions) (*PostgresService, error) {
	// Merge provided options with defaults
	defaults := DefaultPostgresOptions()

	// Apply defaults for any unset fields
	if options.Version == "" {
		options.Version = defaults.Version
	}
	if options.Port == "" {
		options.Port = findRandomAvailablePort()
	}
	if options.DataPath == "" {
		options.DataPath = defaults.DataPath
	}
	if options.Password == "" {
		options.Password = defaults.Password
	}
	if options.User == "" {
		options.User = defaults.User
	}
	if options.Database == "" {
		options.Database = defaults.Database
	}
	if options.ContainerName == "" {
		options.ContainerName = defaults.ContainerName
	}

	// Ensure data directory exists
	if err := os.MkdirAll(options.DataPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create PostgreSQL data directory: %w", err)
	}

	// Configure Docker container options
	containerOptions := docker.ContainerOptions{
		ImageName:     "postgres",
		ImageTag:      options.Version,
		ContainerName: options.ContainerName,
		EnvVars: map[string]string{
			"POSTGRES_PASSWORD": options.Password,
			"POSTGRES_USER":     options.User,
			"POSTGRES_DB":       options.Database,
		},
		Ports: map[string]string{
			options.Port: "5432",
		},
		Volumes: map[string]string{
			options.DataPath: "/var/lib/postgresql/data",
		},
		Detached:      true,
		RestartPolicy: "unless-stopped",
	}

	// Create Docker service
	dockerService, err := docker.NewService(containerOptions, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker service: %w", err)
	}

	return &PostgresService{
		dockerService: dockerService,
		options:       options,
		logger:        logger,
	}, nil
}

// Start starts the PostgreSQL container and optionally waits for it to be ready
func (s *PostgresService) Start(ctx context.Context, waitForReady bool) error {
	// Start the PostgreSQL container
	err := s.dockerService.RunContainer(ctx)
	if err != nil {
		return fmt.Errorf("failed to start PostgreSQL container: %w", err)
	}

	if waitForReady {
		// Wait for PostgreSQL to be ready (up to 60 seconds)
		err = s.WaitForReady(ctx, 60*time.Second)
		if err != nil {
			return fmt.Errorf("PostgreSQL container failed to become ready: %w", err)
		}
	}

	return nil
}

// WaitForReady waits for PostgreSQL to be ready to accept connections
func (s *PostgresService) WaitForReady(ctx context.Context, maxWaitTime time.Duration) error {
	s.logger.Info("Waiting for PostgreSQL to be ready")

	// Set up timeout
	timeout := time.Now().Add(maxWaitTime)

	for time.Now().Before(timeout) {
		// Try to execute a simple query
		output, err := s.dockerService.ExecuteCommand(ctx, []string{
			"psql",
			"-U", s.options.User,
			"-d", s.options.Database,
			"-c", "SELECT 1",
		})

		if err == nil && strings.Contains(output, "1 row") {
			s.logger.Info("PostgreSQL is ready")
			return nil
		}

		// Sleep before retrying
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}

	return fmt.Errorf("timeout waiting for PostgreSQL to be ready")
}

// isPortAvailable checks if a port is available to use
func isPortAvailable(port string) bool {
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return false
	}
	if err := ln.Close(); err != nil {
		// Just log this error but still return true as we were able to bind
		fmt.Printf("Error closing listener: %v\n", err)
	}
	return true
}

// findRandomAvailablePort finds a random available port
func findRandomAvailablePort() string {
	// Try to get a random port by asking the OS for one
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		return "" // Unable to find a port
	}
	defer func() {
		if err := ln.Close(); err != nil {
			fmt.Printf("Error closing listener: %v\n", err)
		}
	}()

	// Get the port from the listener
	addr := ln.Addr().(*net.TCPAddr)
	return fmt.Sprintf("%d", addr.Port)
}

// Stop stops the PostgreSQL container
func (s *PostgresService) Stop(ctx context.Context) error {
	return s.dockerService.StopContainer(ctx)
}

// Remove removes the PostgreSQL container
func (s *PostgresService) Remove(ctx context.Context) error {
	return s.dockerService.RemoveContainer(ctx)
}

// CreateDatabase creates a new database in PostgreSQL
func (s *PostgresService) CreateDatabase(ctx context.Context, dbName string) error {
	s.logger.Info("Creating PostgreSQL database", slog.String("database", dbName))

	// Execute the create database command
	_, err := s.dockerService.ExecuteCommand(ctx, []string{
		"psql",
		"-U", s.options.User,
		"-c", fmt.Sprintf("CREATE DATABASE %s;", dbName),
	})

	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}

	s.logger.Info("Created PostgreSQL database", slog.String("database", dbName))
	return nil
}

// GetConnectionString returns the connection string for PostgreSQL
func (s *PostgresService) GetConnectionString(dbName string) string {
	// If dbName is empty, use the default database
	if dbName == "" {
		dbName = s.options.Database
	}

	return fmt.Sprintf("postgres://%s:%s@localhost:%s/%s?sslmode=disable",
		s.options.User, s.options.Password, s.options.Port, dbName)
}

// GetLogs gets logs from the PostgreSQL container
func (s *PostgresService) GetLogs(ctx context.Context) (string, error) {
	return s.dockerService.GetContainerLogs(ctx)
}

// ExecuteSQL executes a SQL command in PostgreSQL
func (s *PostgresService) ExecuteSQL(ctx context.Context, database, sql string) (string, error) {
	return s.dockerService.ExecuteCommand(ctx, []string{
		"psql",
		"-U", s.options.User,
		"-d", database,
		"-c", sql,
	})
}

// DockerService returns the underlying Docker service
func (s *PostgresService) DockerService() *docker.Service {
	return s.dockerService
}

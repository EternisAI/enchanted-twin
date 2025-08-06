package bootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/EternisAI/enchanted-twin/migrations"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory/storage"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

// Custom PostgreSQL version to match our 17.5 binaries.
const PostgreSQL17_5 embeddedpostgres.PostgresVersion = "17.5"

// Test-specific PostgreSQL version to match pgvector binaries.
const PostgreSQL16_4 embeddedpostgres.PostgresVersion = "16.4"

type PostgresServer struct {
	postgres    *embeddedpostgres.EmbeddedPostgres
	db          *sql.DB
	port        uint32
	dataPath    string
	logger      *log.Logger
	hasPgvector bool
}

func BootstrapPostgresServer(ctx context.Context, logger *log.Logger, port string, dataPath string) (*PostgresServer, error) {
	appDataPath := os.Getenv("APP_DATA_PATH")
	if appDataPath == "" {
		appDataPath = "./output"
	}

	runtimePath := filepath.Join(appDataPath, "postgres-runtime")

	absDataPath, err := filepath.Abs(dataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve dataPath: %w", err)
	}
	absRuntimePath, err := filepath.Abs(runtimePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve runtimePath: %w", err)
	}

	relPath, err := filepath.Rel(absRuntimePath, absDataPath)
	if err == nil && !strings.HasPrefix(relPath, "..") {
		return nil, fmt.Errorf("CRITICAL: dataPath (%s) is inside runtimePath (%s) - this would cause data loss as runtime is recreated on each start", absDataPath, absRuntimePath)
	}

	logger.Info("PostgreSQL path validation passed", "dataPath", absDataPath, "runtimePath", absRuntimePath)
	return BootstrapPostgresServerWithPaths(ctx, logger, port, dataPath, runtimePath)
}

func BootstrapPostgresServerWithPaths(ctx context.Context, logger *log.Logger, port string, dataPath string, runtimePath string) (*PostgresServer, error) {
	return BootstrapPostgresServerWithOptions(ctx, logger, port, dataPath, runtimePath, true)
}

func BootstrapPostgresServerWithOptions(ctx context.Context, logger *log.Logger, port string, dataPath string, runtimePath string, enablePgvector bool) (*PostgresServer, error) {
	// Use PostgreSQL 16 when pgvector binaries aren't available (fallback to standard embedded-postgres)
	return BootstrapPostgresServerWithVersion(ctx, logger, port, dataPath, runtimePath, enablePgvector, embeddedpostgres.V16)
}

func BootstrapPostgresServerWithVersion(ctx context.Context, logger *log.Logger, port string, dataPath string, runtimePath string, enablePgvector bool, version embeddedpostgres.PostgresVersion) (*PostgresServer, error) {
	startTime := time.Now()
	logger.Info("Starting PostgreSQL server bootstrap", "port", port, "dataPath", dataPath)

	// Convert port to uint32
	portInt, err := strconv.ParseUint(port, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid port: %w", err)
	}

	// Find available port if specified port is in use
	actualPort := findAvailablePostgresPort(uint32(portInt), logger)

	// Ensure data directory and runtime directory exist
	if err := os.MkdirAll(dataPath, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}
	if err := os.MkdirAll(runtimePath, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create runtime directory: %w", err)
	}

	// Check for local binaries in APP_DATA_PATH/postgres first
	appDataPath := os.Getenv("APP_DATA_PATH")
	if appDataPath == "" {
		appDataPath = "./output"
	}
	localBinariesPath := filepath.Join(appDataPath, "postgres")
	var hasPgvector bool
	var pgvectorBinariesPath string

	if _, err := os.Stat(filepath.Join(localBinariesPath, "bin", "postgres")); err == nil {
		// Check if pgvector extension exists in local binaries
		vectorExtPath := filepath.Join(localBinariesPath, "share", "postgresql", "extension", "vector.control")
		if _, err := os.Stat(vectorExtPath); err == nil {
			pgvectorBinariesPath = localBinariesPath
			hasPgvector = true
			logger.Info("Using local PostgreSQL binaries with pgvector support", "path", localBinariesPath)
		} else {
			logger.Info("Using local PostgreSQL binaries without pgvector support", "path", localBinariesPath)
			pgvectorBinariesPath = localBinariesPath
		}
	} else if enablePgvector {
		logger.Warn("pgvector requested but no local binaries found - using standard PostgreSQL without pgvector")
	}

	// Get password from environment variable or use default for embedded server
	password := os.Getenv("POSTGRES_PASSWORD")
	if password == "" {
		password = "testpassword" // Default for embedded server only
	}

	// Get timezone from environment variable or use UTC as default
	timezone := os.Getenv("POSTGRES_TIMEZONE")
	if timezone == "" {
		timezone = "UTC" // Safe default timezone
	}

	// Create embedded postgres configuration
	config := embeddedpostgres.DefaultConfig().
		Port(actualPort).
		DataPath(dataPath).
		RuntimePath(runtimePath).
		Username("postgres").
		Password(password).
		Database("postgres").
		Version(version). // Use specified version
		StartTimeout(60 * time.Second).
		StartParameters(map[string]string{
			"timezone":     timezone,
			"log_timezone": timezone,
		})

	// Use binaries if available
	if pgvectorBinariesPath != "" {
		config = config.BinariesPath(pgvectorBinariesPath)
		logger.Debug("Configured embedded PostgreSQL to use binaries", "path", pgvectorBinariesPath)
	}

	postgres := embeddedpostgres.NewDatabase(config)

	// Check if PostgreSQL cluster already exists in data directory
	pgVersionFile := filepath.Join(dataPath, "PG_VERSION")
	if _, err := os.Stat(pgVersionFile); err == nil {
		logger.Debug("Found existing PostgreSQL cluster", "dataPath", dataPath)
	} else {
		logger.Debug("No existing PostgreSQL cluster found, will initialize new cluster", "dataPath", dataPath)
	}

	logger.Info("Starting embedded PostgreSQL server", "port", actualPort, "dataPath", dataPath)

	// Kill any existing PostgreSQL processes to prevent lock file conflicts
	if err := killExistingPostgresProcesses(logger, dataPath); err != nil {
		logger.Warn("Failed to kill existing PostgreSQL processes", "error", err)
	}

	if err := postgres.Start(); err != nil {
		return nil, fmt.Errorf("failed to start PostgreSQL: %w", err)
	}

	// Connect to the database
	connString := fmt.Sprintf("host=localhost port=%d user=postgres password=testpassword dbname=postgres sslmode=disable", actualPort)

	pgxConfig, err := pgx.ParseConfig(connString)
	if err != nil {
		if stopErr := postgres.Stop(); stopErr != nil {
			logger.Error("Failed to stop PostgreSQL after config parse error", "error", stopErr)
		}
		return nil, fmt.Errorf("failed to parse connection config: %w", err)
	}

	db := stdlib.OpenDB(*pgxConfig)

	// Test connection with retry
	maxRetries := 30
	for i := 0; i < maxRetries; i++ {
		if err := db.Ping(); err == nil {
			break
		}
		if i == maxRetries-1 {
			if stopErr := postgres.Stop(); stopErr != nil {
				logger.Error("Failed to stop PostgreSQL after connection timeout", "error", stopErr)
			}
			return nil, fmt.Errorf("failed to connect to PostgreSQL after %d attempts", maxRetries)
		}
		time.Sleep(100 * time.Millisecond)
	}

	server := &PostgresServer{
		postgres:    postgres,
		db:          db,
		port:        actualPort,
		dataPath:    dataPath,
		logger:      logger,
		hasPgvector: hasPgvector,
	}

	// Enable pgvector extension if we have pgvector binaries
	if hasPgvector {
		if err := server.enablePgvectorExtension(ctx); err != nil {
			// If pgvector extension fails, warn but continue without it
			logger.Warn("Failed to enable pgvector extension, continuing without vector support", "error", err)
			server.hasPgvector = false
		}
	}

	// Run migrations only if pgvector is enabled (production mode)
	if server.hasPgvector {
		if err := server.runMigrations(ctx); err != nil {
			if stopErr := server.Stop(); stopErr != nil {
				logger.Error("Failed to stop PostgreSQL after migration error", "error", stopErr)
			}
			return nil, fmt.Errorf("failed to run migrations: %w", err)
		}
	}

	logger.Info("PostgreSQL server is ready",
		"elapsed", time.Since(startTime),
		"port", actualPort,
		"dataPath", dataPath,
		"pgvector_enabled", server.hasPgvector,
		"timezone", timezone)

	return server, nil
}

func (s *PostgresServer) enablePgvectorExtension(ctx context.Context) error {
	s.logger.Debug("Enabling pgvector extension")

	_, err := s.db.ExecContext(ctx, "CREATE EXTENSION IF NOT EXISTS vector")
	if err != nil {
		return fmt.Errorf("failed to create vector extension: %w", err)
	}

	s.logger.Debug("pgvector extension enabled successfully")
	return nil
}

func (s *PostgresServer) runMigrations(ctx context.Context) error {
	s.logger.Debug("Running database migrations")

	// Set up goose with embedded filesystem
	goose.SetBaseFS(migrations.EmbedPostgresMigrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	// Run migrations from embedded filesystem
	if err := goose.UpContext(ctx, s.db, "."); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	s.logger.Debug("Database migrations completed successfully")
	return nil
}

func (s *PostgresServer) GetDB() *sql.DB {
	return s.db
}

func (s *PostgresServer) GetPort() uint32 {
	return s.port
}

func (s *PostgresServer) Stop() error {
	s.logger.Info("Stopping PostgreSQL server")

	if s.db != nil {
		if err := s.db.Close(); err != nil {
			s.logger.Error("Failed to close database connection", "error", err)
		}
	}

	if s.postgres != nil {
		if err := s.postgres.Stop(); err != nil {
			s.logger.Error("Failed to stop PostgreSQL server", "error", err)
			return err
		}
	}

	s.logger.Info("PostgreSQL server stopped successfully")
	return nil
}

func killExistingPostgresProcesses(logger *log.Logger, dataPath string) error {
	pidFilePath := filepath.Join(dataPath, "postmaster.pid")

	// Check if postmaster.pid file exists
	if _, err := os.Stat(pidFilePath); os.IsNotExist(err) {
		logger.Debug("No postmaster.pid file found", "path", pidFilePath)
		return nil
	}

	logger.Info("Found existing postmaster.pid file, attempting to clean up", "path", pidFilePath)

	// Read the PID from the file
	pidData, err := os.ReadFile(pidFilePath)
	if err != nil {
		logger.Warn("Failed to read postmaster.pid file", "error", err)
		// Still try to remove the file
		if removeErr := os.Remove(pidFilePath); removeErr != nil {
			logger.Warn("Failed to remove stale postmaster.pid file", "error", removeErr)
		} else {
			logger.Info("Removed stale postmaster.pid file")
		}
		return nil
	}

	// Parse PID (first line of the file)
	lines := strings.Split(strings.TrimSpace(string(pidData)), "\n")
	if len(lines) == 0 {
		logger.Warn("Empty postmaster.pid file")
		if err := os.Remove(pidFilePath); err != nil {
			logger.Warn("Failed to remove empty postmaster.pid file", "error", err)
		}
		return nil
	}

	pid := strings.TrimSpace(lines[0])
	if pid == "" {
		logger.Warn("Invalid PID in postmaster.pid file")
		if err := os.Remove(pidFilePath); err != nil {
			logger.Warn("Failed to remove invalid postmaster.pid file", "error", err)
		}
		return nil
	}

	logger.Info("Found PostgreSQL process", "pid", pid)

	// Check if process is still running (Unix-like systems only)
	if runtime.GOOS == "windows" {
		logger.Debug("Skipping process check on Windows, removing pid file")
		if err := os.Remove(pidFilePath); err != nil {
			logger.Warn("Failed to remove postmaster.pid file", "error", err)
		}
		return nil
	}

	// Check if kill command is available
	if _, err := exec.LookPath("kill"); err != nil {
		logger.Warn("kill command not found, removing pid file without process check", "error", err)
		if err := os.Remove(pidFilePath); err != nil {
			logger.Warn("Failed to remove postmaster.pid file", "error", err)
		}
		return nil
	}

	if err := exec.Command("kill", "-0", pid).Run(); err != nil {
		// Process is not running, remove stale pid file
		logger.Info("PostgreSQL process is not running, removing stale pid file", "pid", pid)
		if err := os.Remove(pidFilePath); err != nil {
			logger.Warn("Failed to remove stale postmaster.pid file", "error", err)
		} else {
			logger.Info("Removed stale postmaster.pid file")
		}
		return nil
	}

	// Process is running, try to use pg_ctl for graceful shutdown first
	logger.Info("PostgreSQL process is running, attempting graceful shutdown", "pid", pid)

	// Try to find pg_ctl and use it for proper shutdown
	gracefulShutdown := false
	pgCtlPath := findPgCtlPath(logger)
	if pgCtlPath != "" {
		logger.Info("Attempting graceful shutdown using pg_ctl", "pg_ctl_path", pgCtlPath)
		if err := exec.Command(pgCtlPath, "-D", dataPath, "stop", "-m", "fast").Run(); err != nil {
			logger.Warn("pg_ctl stop failed, falling back to signal-based shutdown", "error", err)
		} else {
			logger.Info("Successfully stopped PostgreSQL using pg_ctl")
			gracefulShutdown = true
		}
	} else {
		logger.Debug("pg_ctl not found, using signal-based shutdown")
	}

	// If pg_ctl didn't work, fall back to signal-based shutdown
	if !gracefulShutdown {
		if err := exec.Command("kill", "-TERM", pid).Run(); err != nil {
			logger.Warn("Failed to send SIGTERM to PostgreSQL process", "pid", pid, "error", err)
			// Try SIGKILL as last resort (PostgreSQL docs warn against this)
			logger.Warn("Using SIGKILL as last resort - this may leave shared memory segments")
			if err := exec.Command("kill", "-KILL", pid).Run(); err != nil {
				logger.Warn("Failed to send SIGKILL to PostgreSQL process", "pid", pid, "error", err)
			} else {
				logger.Info("Killed PostgreSQL process with SIGKILL", "pid", pid)
			}
		} else {
			logger.Info("Sent SIGTERM to PostgreSQL process", "pid", pid)
		}
	}

	// Wait a bit for process to terminate
	time.Sleep(1 * time.Second)

	// Clean up shared memory segments
	if err := cleanupPostgresSharedMemory(logger, dataPath); err != nil {
		logger.Warn("Failed to cleanup shared memory segments", "error", err)
	}

	// Remove the pid file if it still exists
	if _, err := os.Stat(pidFilePath); err == nil {
		if err := os.Remove(pidFilePath); err != nil {
			logger.Warn("Failed to remove postmaster.pid file after process termination", "error", err)
		} else {
			logger.Info("Removed postmaster.pid file after process termination")
		}
	}

	return nil
}

func cleanupPostgresSharedMemory(logger *log.Logger, dataPath string) error {
	// Only attempt cleanup on Unix-like systems (macOS, Linux)
	if runtime.GOOS == "windows" {
		logger.Debug("Skipping shared memory cleanup on Windows")
		return nil
	}

	// Check if required commands are available
	if _, err := exec.LookPath("ipcs"); err != nil {
		logger.Warn("ipcs command not found, skipping shared memory cleanup", "error", err)
		return nil
	}
	if _, err := exec.LookPath("ipcrm"); err != nil {
		logger.Warn("ipcrm command not found, skipping shared memory cleanup", "error", err)
		return nil
	}

	// Look for postgresql.conf to get the port number for shared memory key calculation
	// PostgreSQL uses port number as part of the shared memory key
	postgresqlConfPath := filepath.Join(dataPath, "postgresql.conf")

	// Read port from postgresql.conf if it exists
	var port uint32 = 5432 // default port
	if confData, err := os.ReadFile(postgresqlConfPath); err == nil {
		lines := strings.Split(string(confData), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "port") && strings.Contains(line, "=") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					portStr := strings.TrimSpace(parts[1])
					portStr = strings.Trim(portStr, "'\"")
					// Handle comments in the line
					if commentIdx := strings.Index(portStr, "#"); commentIdx >= 0 {
						portStr = strings.TrimSpace(portStr[:commentIdx])
					}
					if portInt, err := strconv.ParseUint(portStr, 10, 32); err == nil {
						port = uint32(portInt)
						break
					}
				}
			}
		}
	}

	logger.Debug("Cleaning up PostgreSQL shared memory segments", "port", port, "dataPath", dataPath, "os", runtime.GOOS)

	// PostgreSQL calculates shared memory key as: 0x2000000 + port * 0x10000
	// This is based on PostgreSQL's source code in src/backend/port/sysv_shmem.c
	shmKey := 0x2000000 + int(port)*0x10000

	// Find shared memory segments using ipcs
	cmd := exec.Command("ipcs", "-m")
	output, err := cmd.Output()
	if err != nil {
		// On some systems, ipcs might require different flags or might not be available
		logger.Debug("Failed to list shared memory segments, this is usually not critical", "error", err)
		return nil // Don't treat this as a fatal error
	}

	lines := strings.Split(string(output), "\n")
	segmentsFound := 0

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			// Check if this line contains a shared memory key that matches PostgreSQL pattern
			if keyStr := fields[1]; strings.HasPrefix(keyStr, "0x") {
				if key, err := strconv.ParseInt(keyStr, 0, 64); err == nil {
					// Check if this key matches PostgreSQL's pattern (within reasonable range)
					if key >= int64(shmKey) && key < int64(shmKey+0x10000) {
						shmid := fields[0]
						segmentsFound++
						logger.Info("Removing PostgreSQL shared memory segment", "key", keyStr, "shmid", shmid)

						if err := exec.Command("ipcrm", "-m", shmid).Run(); err != nil {
							logger.Warn("Failed to remove shared memory segment", "shmid", shmid, "error", err)
						} else {
							logger.Info("Successfully removed shared memory segment", "shmid", shmid)
						}
					}
				}
			}
		}
	}

	if segmentsFound == 0 {
		logger.Debug("No PostgreSQL shared memory segments found to clean up")
	}

	return nil
}

func findPgCtlPath(logger *log.Logger) string {
	// First, try to find pg_ctl in the same binary directory as the PostgreSQL binaries
	// Check if we're using bundled binaries by looking for a bin directory near dataPath
	appDataPath := os.Getenv("APP_DATA_PATH")
	if appDataPath == "" {
		appDataPath = "./output"
	}

	// Check local binaries path first
	localBinariesPath := filepath.Join(appDataPath, "postgres", "bin", "pg_ctl")
	if _, err := os.Stat(localBinariesPath); err == nil {
		logger.Debug("Found pg_ctl in local binaries", "path", localBinariesPath)
		return localBinariesPath
	}

	// Fall back to system PATH
	if pgCtlPath, err := exec.LookPath("pg_ctl"); err == nil {
		logger.Debug("Found pg_ctl in system PATH", "path", pgCtlPath)
		return pgCtlPath
	}

	logger.Debug("pg_ctl not found in local binaries or system PATH")
	return ""
}

// isPortInUse checks if a port is already in use by attempting to bind to it.
func isPortInUse(port int) bool {
	conn, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return true
	}
	_ = conn.Close()
	return false
}

func findAvailablePostgresPort(preferredPort uint32, logger *log.Logger) uint32 {
	// Try the preferred port first
	if !isPortInUse(int(preferredPort)) {
		return preferredPort
	}

	logger.Info("Preferred port is in use, finding alternative", "preferred_port", preferredPort)

	// Try a range of ports starting from the preferred port
	for port := preferredPort + 1; port < preferredPort+100; port++ {
		if !isPortInUse(int(port)) {
			logger.Info("Using alternative port", "port", port)
			return port
		}
	}

	// If no port found in range, try random port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		logger.Warn("Failed to get random port, using preferred port anyway", "error", err)
		return preferredPort
	}
	defer func() {
		if err := listener.Close(); err != nil {
			logger.Error("Failed to close listener", "error", err)
		}
	}()

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		logger.Warn("Failed to get TCP address from listener, using preferred port")
		return preferredPort
	}
	randomPort := uint32(addr.Port)
	logger.Info("Using random available port", "port", randomPort)
	return randomPort
}

func InitPostgresSchema(db *sql.DB, port uint32, logger *log.Logger, embedding ai.Embedding, embeddingsModel string) error {
	logger.Debug("Starting PostgreSQL schema initialization")
	start := time.Now()

	embeddingsWrapper, err := storage.NewEmbeddingWrapper(embedding, embeddingsModel)
	if err != nil {
		return fmt.Errorf("creating embedding wrapper: %w", err)
	}

	// Connect with pgx for storage validation
	connString := fmt.Sprintf("host=localhost port=%d user=postgres password=testpassword dbname=postgres sslmode=disable", port)
	conn, err := pgx.Connect(context.Background(), connString)
	if err != nil {
		return fmt.Errorf("connecting to database for validation: %w", err)
	}
	defer func() {
		if err := conn.Close(context.Background()); err != nil {
			logger.Error("Failed to close database connection", "error", err)
		}
	}()

	// Create storage instance for schema validation
	storageInstance, err := storage.NewPostgresStorage(storage.NewPostgresStorageInput{
		DB:                conn,
		Logger:            logger,
		EmbeddingsWrapper: embeddingsWrapper,
		ConnString:        connString,
	})
	if err != nil {
		return fmt.Errorf("creating storage instance: %w", err)
	}

	// Validate schema exists (migrations should have created it)
	if postgresStorageInstance, ok := storageInstance.(*storage.PostgresStorage); ok {
		if err := postgresStorageInstance.ValidateSchema(context.Background()); err != nil {
			return fmt.Errorf("schema validation failed: %w", err)
		}
	}

	logger.Debug("PostgreSQL schema initialization completed", "elapsed", time.Since(start))
	return nil
}

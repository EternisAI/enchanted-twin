package bootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/charmbracelet/log"
	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory/storage"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/bootstrap/pgvector"
)

type PostgresServer struct {
	postgres      *embeddedpostgres.EmbeddedPostgres
	db            *sql.DB
	port          uint32
	dataPath      string
	logger        *log.Logger
	binaryManager *pgvector.BinaryManager
	hasPgvector   bool
}

func BootstrapPostgresServer(ctx context.Context, logger *log.Logger, port string, dataPath string) (*PostgresServer, error) {
	return BootstrapPostgresServerWithPaths(ctx, logger, port, dataPath, filepath.Join(dataPath, "runtime"))
}

func BootstrapPostgresServerWithPaths(ctx context.Context, logger *log.Logger, port string, dataPath string, runtimePath string) (*PostgresServer, error) {
	return BootstrapPostgresServerWithOptions(ctx, logger, port, dataPath, runtimePath, true)
}

func BootstrapPostgresServerWithOptions(ctx context.Context, logger *log.Logger, port string, dataPath string, runtimePath string, enablePgvector bool) (*PostgresServer, error) {
	startTime := time.Now()
	logger.Info("Starting PostgreSQL server bootstrap", "port", port, "dataPath", dataPath)

	// Convert port to uint32
	portInt, err := strconv.ParseUint(port, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid port: %w", err)
	}

	// Find available port if specified port is in use
	actualPort := findAvailablePostgresPort(uint32(portInt), logger)

	// Ensure data directory exists
	if err := os.MkdirAll(dataPath, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Initialize pgvector binary manager
	var binaryManager *pgvector.BinaryManager
	var hasPgvector bool
	var pgvectorBinariesPath string

	if enablePgvector {
		binaryManager = pgvector.NewBinaryManager(logger, "")

		// Try to get pgvector-enabled binaries
		binariesPath, hasVector, err := binaryManager.GetBinariesPath(ctx)
		if err != nil {
			logger.Warn("Failed to get pgvector binaries, falling back to standard PostgreSQL", "error", err)
		} else if hasVector {
			logger.Info("Using pgvector-enabled PostgreSQL binaries", "path", binariesPath)
			pgvectorBinariesPath = binariesPath
			hasPgvector = true
		} else {
			logger.Info("pgvector binaries not available, using standard PostgreSQL")
		}
	}

	// Create embedded postgres configuration
	config := embeddedpostgres.DefaultConfig().
		Port(actualPort).
		DataPath(dataPath).
		RuntimePath(runtimePath).
		Username("postgres").
		Password("testpassword").
		Database("postgres").
		StartTimeout(60 * time.Second)

	// Use pgvector binaries if available
	if hasPgvector && pgvectorBinariesPath != "" {
		config = config.BinariesPath(pgvectorBinariesPath)
		logger.Debug("Configured embedded PostgreSQL to use pgvector binaries", "path", pgvectorBinariesPath)
	}

	postgres := embeddedpostgres.NewDatabase(config)

	logger.Info("Starting embedded PostgreSQL server", "port", actualPort, "dataPath", dataPath)

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
		postgres:      postgres,
		db:            db,
		port:          actualPort,
		dataPath:      dataPath,
		logger:        logger,
		binaryManager: binaryManager,
		hasPgvector:   hasPgvector,
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
		"pgvector_enabled", server.hasPgvector)

	return server, nil
}

// HasPgvector returns true if the server has pgvector extension enabled.
func (s *PostgresServer) HasPgvector() bool {
	return s.hasPgvector
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

	// Set up goose
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	// Run migrations from the migrations directory
	migrationsDir := "./migrations"
	if err := goose.UpContext(ctx, s.db, migrationsDir); err != nil {
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

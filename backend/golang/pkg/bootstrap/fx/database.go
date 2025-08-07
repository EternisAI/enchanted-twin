package fx

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/charmbracelet/log"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory/storage"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/bootstrap"
	"github.com/EternisAI/enchanted-twin/pkg/config"
)

// DatabaseModule provides database and storage services.
var DatabaseModule = fx.Module("database",
	fx.Provide(
		ProvideEmbeddingWrapper,
		ProvideMemoryStorage,
	),
	fx.Invoke(func() {}), // Trigger module initialization
)

// EmbeddingWrapperResult provides embedding wrapper for memory storage.
type EmbeddingWrapperResult struct {
	fx.Out
	EmbeddingWrapper *storage.EmbeddingWrapper
}

// ProvideEmbeddingWrapper creates embedding wrapper for memory operations.
func ProvideEmbeddingWrapper(logger *log.Logger, aiEmbeddingsService ai.Embedding, envs *config.Config) (EmbeddingWrapperResult, error) {
	logger.Info("Creating embedding wrapper", "model", envs.EmbeddingsModel)

	embeddingsWrapper, err := storage.NewEmbeddingWrapper(aiEmbeddingsService, envs.EmbeddingsModel)
	if err != nil {
		logger.Error("Failed to create embedding wrapper", "error", err)
		return EmbeddingWrapperResult{}, err
	}

	return EmbeddingWrapperResult{EmbeddingWrapper: embeddingsWrapper}, nil
}

// MemoryStorageResult provides memory storage interface.
type MemoryStorageResult struct {
	fx.Out
	Storage storage.Interface
}

// ProvideMemoryStorage creates memory storage based on configured backend.
func ProvideMemoryStorage(
	lc fx.Lifecycle,
	logger *log.Logger,
	envs *config.Config,
	aiEmbeddingsService ai.Embedding,
	embeddingWrapper *storage.EmbeddingWrapper,
) (MemoryStorageResult, error) {
	logger.Info("Initializing memory storage", "backend", envs.MemoryBackend)
	memoryCreateStart := time.Now()

	var storageInterface storage.Interface
	var err error

	switch envs.MemoryBackend {
	case "postgresql", "": // treat empty as default
		storageInterface, err = createPostgreSQLStorage(
			context.Background(),
			logger,
			envs,
			aiEmbeddingsService,
			embeddingWrapper,
		)
	default:
		err = fmt.Errorf("unsupported memory backend: %s (only 'postgresql' is supported)", envs.MemoryBackend)
		logger.Error("Unsupported memory backend", "backend", envs.MemoryBackend, "supported", []string{"postgresql"})
	}

	if err != nil {
		logger.Error("Failed to create storage interface", "backend", envs.MemoryBackend, "error", err)
		return MemoryStorageResult{}, err
	}

	logger.Info("Memory storage initialized", "backend", envs.MemoryBackend, "elapsed", time.Since(memoryCreateStart))

	// Register cleanup hooks if storage implements cleanup interface
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			logger.Info("Cleaning up memory storage", "backend", envs.MemoryBackend)
			if storageInterface != nil {
				if err := storageInterface.Close(ctx); err != nil {
					logger.Error("Failed to close memory storage", "error", err)
					return err
				}
			}
			return nil
		},
	})

	return MemoryStorageResult{Storage: storageInterface}, nil
}

// postgresCleanupResources defines resources that need cleanup for PostgreSQL storage.
type postgresCleanupResources struct {
	server interface{ Stop() error } // PostgreSQL server
	pool   *pgxpool.Pool             // Connection pool
	logger *log.Logger
}

// cleanup performs context-aware cleanup of PostgreSQL resources with timeout.
func (r *postgresCleanupResources) cleanup(ctx context.Context) {
	// Create cleanup context with timeout
	cleanupCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Channel to signal cleanup completion
	done := make(chan struct{})

	go func() {
		defer close(done)

		// Close connection pool first
		if r.pool != nil {
			r.logger.Info("Closing PostgreSQL connection pool")
			r.pool.Close()
		}

		// Stop PostgreSQL server
		if r.server != nil {
			r.logger.Info("Stopping PostgreSQL server")
			if err := r.server.Stop(); err != nil {
				r.logger.Error("Failed to stop PostgreSQL server during cleanup", "error", err)
			} else {
				r.logger.Info("PostgreSQL server stopped successfully")
			}
		}
	}()

	// Wait for cleanup completion or timeout
	select {
	case <-done:
		r.logger.Info("PostgreSQL cleanup completed successfully")
	case <-cleanupCtx.Done():
		r.logger.Warn("PostgreSQL cleanup timed out after 10 seconds")
	}
}

// createPostgreSQLStorage initializes PostgreSQL storage backend.
func createPostgreSQLStorage(
	ctx context.Context,
	logger *log.Logger,
	envs *config.Config,
	aiEmbeddingsService ai.Embedding,
	embeddingsWrapper *storage.EmbeddingWrapper,
) (storage.Interface, error) {
	logger.Info("Setting up PostgreSQL memory backend")

	// Initialize cleanup resources tracker
	cleanup := &postgresCleanupResources{
		logger: logger,
	}

	// Always use AppData path for PostgreSQL data to ensure persistence
	postgresPath := filepath.Join(envs.AppDataPath, "postgres-data")
	logger.Info("Starting PostgreSQL bootstrap process",
		"path", postgresPath,
		"port", envs.PostgresPort,
		"app_data_path", envs.AppDataPath,
		"forced_appdata_path", true)
	postgresBootstrapStart := time.Now()

	postgresServer, err := bootstrap.BootstrapPostgresServer(ctx, logger, envs.PostgresPort, postgresPath)
	if err != nil {
		return nil, fmt.Errorf("failed to bootstrap PostgreSQL server: %w", err)
	}
	cleanup.server = postgresServer
	logger.Info("PostgreSQL server bootstrap completed", "elapsed", time.Since(postgresBootstrapStart))

	logger.Info("Initializing PostgreSQL schema")
	schemaInitStart := time.Now()
	if err := bootstrap.InitPostgresSchema(postgresServer.GetDB(), postgresServer.GetPort(), logger, aiEmbeddingsService, envs.EmbeddingsModel); err != nil {
		cleanup.cleanup(ctx)
		return nil, fmt.Errorf("failed to initialize PostgreSQL schema: %w", err)
	}
	logger.Info("PostgreSQL schema initialization completed", "elapsed", time.Since(schemaInitStart))

	// Create PostgreSQL connection pool for storage
	connString := fmt.Sprintf("host=localhost port=%d user=postgres password=testpassword dbname=postgres sslmode=disable", postgresServer.GetPort())

	// Configure connection pool with proper parameters
	poolConfig, err := pgxpool.ParseConfig(connString + " pool_max_conns=20 pool_min_conns=5")
	if err != nil {
		cleanup.cleanup(ctx)
		return nil, fmt.Errorf("failed to parse PostgreSQL pool config: %w", err)
	}

	pgPool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		cleanup.cleanup(ctx)
		return nil, fmt.Errorf("failed to create PostgreSQL connection pool for storage: %w", err)
	}
	cleanup.pool = pgPool

	storageInterface, err := storage.NewPostgresStorage(storage.NewPostgresStorageInput{
		DB:                pgPool,
		Logger:            logger,
		EmbeddingsWrapper: embeddingsWrapper,
		ConnString:        connString,
	})
	if err != nil {
		cleanup.cleanup(ctx)
		return nil, fmt.Errorf("failed to create PostgreSQL storage interface: %w", err)
	}

	logger.Info("PostgreSQL memory backend initialized successfully")
	return storageInterface, nil
}

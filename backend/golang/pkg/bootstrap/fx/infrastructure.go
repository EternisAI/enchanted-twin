package fx

import (
	"context"

	"github.com/charmbracelet/log"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"go.uber.org/fx"

	"github.com/EternisAI/enchanted-twin/pkg/bootstrap"
	"github.com/EternisAI/enchanted-twin/pkg/config"
	"github.com/EternisAI/enchanted-twin/pkg/db"
)

// InfrastructureModule provides all infrastructure services.
var InfrastructureModule = fx.Module("infrastructure",
	fx.Provide(
		ProvideLogger,
		ProvideLoggerFactory,
		ProvideConfig,
		ProvideNATSServer,
		ProvideNATSClient,
		ProvideStore,
		ProvideSQLCDatabase,
	),
)

func ProvideLogger() (*log.Logger, error) {
	// Load config first to determine logging settings
	cfg, err := config.LoadConfigWithAutoDetection()
	if err != nil {
		// If config loading fails, use bootstrap logger to report error
		bootstrapLogger := bootstrap.NewBootstrapLogger()
		bootstrapLogger.Error("Failed to load config", "error", err)
		return nil, err
	}

	logger := bootstrap.NewLogger(cfg)
	// Create a factory temporarily to get a component-aware logger for infrastructure
	factory := bootstrap.NewLoggerFactoryWithConfig(logger, cfg.ComponentLogLevels)
	infraLogger := factory.ForComponent("infrastructure.main")
	infraLogger.Info("Using database path", "path", cfg.DBPath)

	return logger, nil
}

func ProvideLoggerFactory(logger *log.Logger) (*bootstrap.LoggerFactory, error) {
	// We need config again for component log levels, but can't create circular dependency
	// So we'll load it again here - this is acceptable since it's cached
	cfg, err := config.LoadConfigWithAutoDetection()
	if err != nil {
		return nil, err
	}
	return bootstrap.NewLoggerFactoryWithConfig(logger, cfg.ComponentLogLevels), nil
}

func ProvideConfig(loggerFactory *bootstrap.LoggerFactory) (*config.Config, error) {
	logger := loggerFactory.ForComponent("infrastructure.config")
	// Config is already loaded in ProvideLogger, load it again
	// This avoids circular dependencies and is acceptable since config loading is fast
	envs, err := config.LoadConfigWithAutoDetection()
	if err != nil {
		logger.Error("Failed to load config", "error", err)
		return nil, err
	}
	return envs, nil
}

// NATSServerResult wraps NATS server for lifecycle management.
type NATSServerResult struct {
	fx.Out
	Server *server.Server
}

// ProvideNATSServer creates and starts embedded NATS server.
func ProvideNATSServer(lc fx.Lifecycle, loggerFactory *bootstrap.LoggerFactory) (NATSServerResult, error) {
	logger := loggerFactory.ForNATS("nats.server")
	natsServer, err := bootstrap.StartEmbeddedNATSServer(logger)
	if err != nil {
		logger.Error("Unable to start nats server", "error", err)
		return NATSServerResult{}, err
	}
	logger.Info("NATS server started")

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			logger.Info("Shutting down NATS server")
			natsServer.Shutdown()
			return nil
		},
	})

	return NATSServerResult{Server: natsServer}, nil
}

// ProvideNATSClient creates NATS client connection.
func ProvideNATSClient(lc fx.Lifecycle, loggerFactory *bootstrap.LoggerFactory, natsServer *server.Server) (*nats.Conn, error) {
	logger := loggerFactory.ForNATS("nats.client")
	nc, err := bootstrap.NewNatsClient()
	if err != nil {
		logger.Error("Unable to create nats client", "error", err)
		return nil, err
	}
	logger.Info("NATS client started")

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			logger.Info("Closing NATS client")
			nc.Close()
			return nil
		},
	})

	return nc, nil
}

// StoreResult wraps database store for lifecycle management.
type StoreResult struct {
	fx.Out
	Store *db.Store
}

// ProvideStore creates and initializes database store.
func ProvideStore(lc fx.Lifecycle, loggerFactory *bootstrap.LoggerFactory, envs *config.Config) (StoreResult, error) {
	logger := loggerFactory.ForDatabase("sqlite.store")
	store, err := db.NewStore(context.Background(), envs.DBPath, logger)
	if err != nil {
		logger.Error("Unable to create or initialize database", "error", err)
		return StoreResult{}, err
	}

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			logger.Info("Closing database store")
			if err := store.Close(); err != nil {
				logger.Error("Error closing store", "error", err)
				return err
			}
			return nil
		},
	})

	return StoreResult{Store: store}, nil
}

// SQLCDatabaseResult provides SQLC database queries.
type SQLCDatabaseResult struct {
	fx.Out
	Database *db.DB
}

// ProvideSQLCDatabase creates type-safe database queries.
func ProvideSQLCDatabase(loggerFactory *bootstrap.LoggerFactory, store *db.Store) (SQLCDatabaseResult, error) {
	logger := loggerFactory.ForDatabase("sqlite.queries")
	dbsqlc, err := db.New(store.DB().DB, logger)
	if err != nil {
		logger.Error("Error creating database", "error", err)
		return SQLCDatabaseResult{}, err
	}
	return SQLCDatabaseResult{Database: dbsqlc}, nil
}

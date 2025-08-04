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
		ProvideConfig,
		ProvideNATSServer,
		ProvideNATSClient,
		ProvideStore,
		ProvideSQLCDatabase,
	),
)

// ProvideLogger creates a shared logger instance.
func ProvideLogger() *log.Logger {
	return bootstrap.NewLogger()
}

// ProvideConfig loads application configuration.
func ProvideConfig(logger *log.Logger) (*config.Config, error) {
	envs, err := config.LoadConfig(true)
	if err != nil {
		logger.Error("Failed to load config", "error", err)
		return nil, err
	}
	logger.Debug("Config loaded", "envs", envs)
	logger.Info("Using database path", "path", envs.DBPath)
	return envs, nil
}

// NATSServerResult wraps NATS server for lifecycle management.
type NATSServerResult struct {
	fx.Out
	Server *server.Server
}

// ProvideNATSServer creates and starts embedded NATS server.
func ProvideNATSServer(lc fx.Lifecycle, logger *log.Logger) (NATSServerResult, error) {
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
func ProvideNATSClient(lc fx.Lifecycle, logger *log.Logger, natsServer *server.Server) (*nats.Conn, error) {
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
func ProvideStore(lc fx.Lifecycle, logger *log.Logger, envs *config.Config) (StoreResult, error) {
	store, err := db.NewStore(context.Background(), envs.DBPath)
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
func ProvideSQLCDatabase(logger *log.Logger, store *db.Store) (SQLCDatabaseResult, error) {
	dbsqlc, err := db.New(store.DB().DB, logger)
	if err != nil {
		logger.Error("Error creating database", "error", err)
		return SQLCDatabaseResult{}, err
	}
	return SQLCDatabaseResult{Database: dbsqlc}, nil
}

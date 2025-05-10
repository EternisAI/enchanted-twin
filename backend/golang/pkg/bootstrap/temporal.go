// Owner: august@eternis.ai
package bootstrap

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	uiserver "github.com/temporalio/ui-server/v2/server"
	uiconfig "github.com/temporalio/ui-server/v2/server/config"
	uiserveroptions "github.com/temporalio/ui-server/v2/server/server_options"
	"go.temporal.io/sdk/client"
	"go.temporal.io/server/common/authorization"
	"go.temporal.io/server/common/cluster"
	"go.temporal.io/server/common/config"
	"go.temporal.io/server/common/dynamicconfig"
	temporallog "go.temporal.io/server/common/log"
	"go.temporal.io/server/common/membership/static"
	sqliteplugin "go.temporal.io/server/common/persistence/sql/sqlplugin/sqlite"
	"go.temporal.io/server/common/primitives"
	sqliteschema "go.temporal.io/server/schema/sqlite"
	"go.temporal.io/server/temporal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	TemporalNamespace  = "default"
	TemporalServerIP   = "127.0.0.1"
	TemporalServerPort = 7233
	TemporalTaskQueue  = "default"
)

func NewTemporalClient(dbPath string) (client.Client, error) {
	return CreateTemporalClient(
		fmt.Sprintf("%s:%d", TemporalServerIP, TemporalServerPort),
		TemporalNamespace,
		"",
	)
}

func CreateTemporalClient(address string, namespace string, apiKey string) (client.Client, error) {
	clientOptions := client.Options{
		HostPort:  address,
		Namespace: namespace,
		ConnectionOptions: client.ConnectionOptions{
			TLS: func() *tls.Config {
				if apiKey != "" {
					return &tls.Config{}
				}
				return nil
			}(),
			DialOptions: []grpc.DialOption{
				grpc.WithUnaryInterceptor(
					func(ctx context.Context, method string, req any, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
						return invoker(
							metadata.AppendToOutgoingContext(ctx, namespace, namespace),
							method,
							req,
							reply,
							cc,
							opts...,
						)
					},
				),
			},
		},
		Credentials: client.NewAPIKeyStaticCredentials(apiKey),
	}

	return client.Dial(clientOptions)
}

// CreateTemporalServer starts a Temporal server and signals readiness on the ready channel.
func CreateTemporalServer(logger *log.Logger, ready chan<- struct{}, dbPath string) {
	ip := TemporalServerIP
	port := TemporalServerPort
	historyPort := port + 1
	matchingPort := port + 2
	workerPort := port + 3
	uiPort := port + 1000
	clusterName := "active"

	// Check if ports are available
	if err := checkPortsAvailable(ip, []int{port, historyPort, matchingPort, workerPort, uiPort}); err != nil {
		logger.Error("Port conflict detected", "error", err)
		close(ready)
		return
	}

	ui := uiserver.NewServer(uiserveroptions.WithConfigProvider(&uiconfig.Config{
		TemporalGRPCAddress: fmt.Sprintf("%s:%d", TemporalServerIP, TemporalServerPort),
		Host:                TemporalServerIP,
		Port:                uiPort,
		EnableUI:            true,
		CORS:                uiconfig.CORS{CookieInsecure: true},
		HideLogs:            true,
	}))
	go func() {
		if err := ui.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("UI server error: %s", err)
		}
	}()

	conf := &config.Config{
		Global: config.Global{},
		Persistence: config.Persistence{
			DefaultStore:     "sqlite-default",
			VisibilityStore:  "sqlite-default",
			NumHistoryShards: 1,
			DataStores: map[string]config.DataStore{
				"sqlite-default": {
					SQL: &config.SQL{
						PluginName:   sqliteplugin.PluginName,
						DatabaseName: dbPath,
					},
				},
			},
		},
		ClusterMetadata: &cluster.Config{
			EnableGlobalNamespace:    false,
			FailoverVersionIncrement: 10,
			MasterClusterName:        clusterName,
			CurrentClusterName:       clusterName,
			ClusterInformation: map[string]cluster.ClusterInformation{
				clusterName: {
					Enabled:                true,
					InitialFailoverVersion: int64(1),
					RPCAddress:             fmt.Sprintf("%s:%d", ip, port),
					ClusterID:              uuid.NewString(),
				},
			},
		},
		DCRedirectionPolicy: config.DCRedirectionPolicy{
			Policy: "noop",
		},
		Services: map[string]config.Service{
			"frontend": {
				RPC: config.RPC{
					GRPCPort: port,
					BindOnIP: ip,
				},
			},
			"history": {
				RPC: config.RPC{
					GRPCPort: historyPort,
					BindOnIP: ip,
				},
			},
			"matching": {
				RPC: config.RPC{
					GRPCPort: matchingPort,
					BindOnIP: ip,
				},
			},
			"worker": {
				RPC: config.RPC{
					GRPCPort: workerPort,
					BindOnIP: ip,
				},
			},
		},
		Archival: config.Archival{
			History: config.HistoryArchival{
				State: "disabled",
			},
			Visibility: config.VisibilityArchival{
				State: "disabled",
			},
		},
		NamespaceDefaults: config.NamespaceDefaults{
			Archival: config.ArchivalNamespaceDefaults{
				History: config.HistoryArchivalNamespaceDefaults{
					State: "disabled",
				},
				Visibility: config.VisibilityArchivalNamespaceDefaults{
					State: "disabled",
				},
			},
		},
		PublicClient: config.PublicClient{
			HostPort: fmt.Sprintf("%s:%d", ip, port),
		},
	}

	sqlConfig := conf.Persistence.DataStores["sqlite-default"].SQL

	err := sqliteschema.SetupSchema(sqlConfig)
	if err != nil && !strings.Contains(err.Error(), "table namespaces already exists") {
		log.Fatalf("failed to setup SQLite schema: %v", err)
	}

	namespaceConfig, err := sqliteschema.NewNamespaceConfig(
		clusterName,
		TemporalNamespace,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("unable to create namespace config: %s", err)
	}
	if err := sqliteschema.CreateNamespaces(sqlConfig, namespaceConfig); err != nil {
		log.Fatalf("unable to create namespace: %s", err)
	}

	authorizer, err := authorization.GetAuthorizerFromConfig(&conf.Global.Authorization)
	if err != nil {
		log.Fatalf("unable to create authorizer: %s", err)
	}
	temporalLogger := temporallog.NewNoopLogger().With()
	claimMapper, err := authorization.GetClaimMapperFromConfig(
		&conf.Global.Authorization,
		temporalLogger,
	)
	if err != nil {
		log.Fatalf("unable to create claim mapper: %s", err)
	}

	dynConf := make(dynamicconfig.StaticClient)
	dynConf[dynamicconfig.ForceSearchAttributesCacheRefreshOnRead.Key()] = true

	server, err := temporal.NewServer(
		temporal.WithConfig(conf),
		temporal.ForServices(temporal.DefaultServices),
		temporal.WithStaticHosts(map[primitives.ServiceName]static.Hosts{
			primitives.FrontendService: static.SingleLocalHost(fmt.Sprintf("%s:%d", ip, port)),
			primitives.HistoryService: static.SingleLocalHost(
				fmt.Sprintf("%s:%d", ip, historyPort),
			),
			primitives.MatchingService: static.SingleLocalHost(
				fmt.Sprintf("%s:%d", ip, matchingPort),
			),
			primitives.WorkerService: static.SingleLocalHost(
				fmt.Sprintf("%s:%d", ip, workerPort),
			),
		}),
		temporal.WithLogger(temporalLogger),
		temporal.WithAuthorizer(authorizer),
		temporal.WithClaimMapper(
			func(*config.Config) authorization.ClaimMapper { return claimMapper },
		),
		temporal.WithDynamicConfigClient(dynConf),
	)
	if err != nil {
		log.Fatalf("unable to start server: %s", err)
	}

	if err := server.Start(); err != nil {
		log.Fatalf("unable to start server: %s", err)
	}

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// signal that the server is ready
	close(ready)
	logger.Info("Temporal server", "ip", ip, "port", port)
	logger.Info("Temporal UI", "address", fmt.Sprintf("http://%s:%d", ip, uiPort))

	// Wait for shutdown signal
	<-sigChan
	logger.Info("Shutting down Temporal server...")

	// Stop the server gracefully
	if err := server.Stop(); err != nil {
		logger.Error("Error stopping server", "error", err)
	}

	// Stop the UI server
	ui.Stop()
}

func checkPortsAvailable(ip string, ports []int) error {
	for _, port := range ports {
		addr := fmt.Sprintf("%s:%d", ip, port)
		conn, err := net.Dial("tcp", addr)
		if err == nil {
			if closeErr := conn.Close(); closeErr != nil {
				return fmt.Errorf("error closing connection: %w", closeErr)
			}
			return fmt.Errorf("port %d is already in use", port)
		}
	}
	return nil
}

// TemporalService represents a Temporal service that can be started and stopped.
type TemporalService struct {
	logger       *log.Logger
	dbPath       string
	readyChan    chan struct{}
	serverCancel context.CancelFunc
	port         int
}

// NewTemporalService creates a new Temporal service.
func NewTemporalService(logger *log.Logger) (*TemporalService, error) {
	// Create a unique SQLite DB path for this instance
	dbPath := fmt.Sprintf("/tmp/temporaldb-%s.db", uuid.New().String())

	return &TemporalService{
		logger:    logger,
		dbPath:    dbPath,
		readyChan: make(chan struct{}),
		port:      TemporalServerPort,
	}, nil
}

// Start starts the Temporal service.
func (s *TemporalService) Start(ctx context.Context) error {
	_, cancel := context.WithCancel(ctx)
	s.serverCancel = cancel

	// Start the server in a goroutine
	go CreateTemporalServer(s.logger, s.readyChan, s.dbPath)

	return nil
}

// Stop stops the Temporal service.
func (s *TemporalService) Stop(ctx context.Context) error {
	if s.serverCancel != nil {
		s.serverCancel()
	}

	// Additional cleanup if necessary
	return nil
}

// WaitForReady waits for the Temporal service to be ready.
func (s *TemporalService) WaitForReady(ctx context.Context, timeout time.Duration) error {
	select {
	case <-s.readyChan:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(timeout):
		return fmt.Errorf("timeout waiting for Temporal to be ready")
	}
}

// GetHostPort returns the host:port address of the Temporal service.
func (s *TemporalService) GetHostPort() string {
	return fmt.Sprintf("%s:%d", TemporalServerIP, s.port)
}

package bootstrap

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

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
)

const (
	TemporalNamespace  = "default"
	TemporalServerIP   = "127.0.0.1"
	TemporalServerPort = 7233
	TemporalTaskQueue  = "default"
)

func NewTemporalClient(dbPath string) (client.Client, error) {
	return CreateTemporalClient(fmt.Sprintf("%s:%d", TemporalServerIP, TemporalServerPort), TemporalNamespace, "")
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
func CreateTemporalServer(logger *slog.Logger, ready chan<- struct{}, dbPath string) {
	ip := TemporalServerIP
	port := TemporalServerPort
	historyPort := port + 1
	matchingPort := port + 2
	workerPort := port + 3
	uiPort := port + 1000
	clusterName := "active"

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

	namespaceConfig, err := sqliteschema.NewNamespaceConfig(clusterName, TemporalNamespace, false, nil)
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
	claimMapper, err := authorization.GetClaimMapperFromConfig(&conf.Global.Authorization, temporalLogger)
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
			primitives.HistoryService:  static.SingleLocalHost(fmt.Sprintf("%s:%d", ip, historyPort)),
			primitives.MatchingService: static.SingleLocalHost(fmt.Sprintf("%s:%d", ip, matchingPort)),
			primitives.WorkerService:   static.SingleLocalHost(fmt.Sprintf("%s:%d", ip, workerPort)),
		}),
		temporal.WithLogger(temporalLogger),
		temporal.WithAuthorizer(authorizer),
		temporal.WithClaimMapper(func(*config.Config) authorization.ClaimMapper { return claimMapper }),
		temporal.WithDynamicConfigClient(dynConf))
	if err != nil {
		log.Fatalf("unable to start server: %s", err)
	}

	if err := server.Start(); err != nil {
		log.Fatalf("unable to start server: %s", err)
	}
	defer func() {
		if err := server.Stop(); err != nil {
			log.Printf("error stopping server: %s", err)
		}
	}()
	// signal that the server is ready
	close(ready)
	logger.Info("Temporal server", "ip", ip, "port", port)
	logger.Info("Temporal UI", "address", fmt.Sprintf("http://%s:%d", ip, uiPort))

	select {}
}

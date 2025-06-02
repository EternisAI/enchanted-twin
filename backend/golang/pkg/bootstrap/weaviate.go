package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/charmbracelet/log"
	"github.com/go-openapi/loads"
	"github.com/jessevdk/go-flags"
	"github.com/pkg/errors"
	"github.com/weaviate/weaviate-go-client/v5/weaviate"
	"github.com/weaviate/weaviate/adapters/handlers/rest"
	"github.com/weaviate/weaviate/adapters/handlers/rest/operations"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory"
	"github.com/EternisAI/enchanted-twin/pkg/contacts/storage"
)

func BootstrapWeaviateServer(ctx context.Context, logger *log.Logger, port string, dataPath string) (*rest.Server, error) {
	startTime := time.Now()
	logger.Info("Starting Weaviate server bootstrap", "port", port, "dataPath", dataPath)

	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		logger.Info("Creating Weaviate data directory", "path", dataPath)
		if err := os.MkdirAll(dataPath, 0o755); err != nil {
			return nil, errors.Wrap(err, "Failed to create Weaviate data directory")
		}
		logger.Info("Weaviate data directory created", "elapsed", time.Since(startTime))
	} else {
		logger.Info("Weaviate data directory exists", "path", dataPath, "elapsed", time.Since(startTime))
	}

	logger.Debug("Setting PERSISTENCE_DATA_PATH environment variable", "path", dataPath)
	err := os.Setenv("PERSISTENCE_DATA_PATH", dataPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to set PERSISTENCE_DATA_PATH")
	}

	logger.Debug("Loading Weaviate swagger specification")
	swaggerSpec, err := loads.Embedded(rest.SwaggerJSON, rest.FlatSwaggerJSON)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to load swagger spec")
	}
	logger.Debug("Swagger specification loaded", "elapsed", time.Since(startTime))

	logger.Debug("Creating Weaviate API instance")
	api := operations.NewWeaviateAPI(swaggerSpec)
	api.Logger = func(s string, i ...any) {
		logger.Debug(s, i...)
	}
	server := rest.NewServer(api)
	logger.Debug("Weaviate API and server created", "elapsed", time.Since(startTime))

	logger.Debug("Configuring Weaviate server", "port", port)
	server.EnabledListeners = []string{"http"}
	p, err := strconv.Atoi(port)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to convert port to int")
	}
	server.Port = p
	logger.Debug("Server port configured", "port", p, "elapsed", time.Since(startTime))

	logger.Debug("Setting up command line parser")
	parser := flags.NewParser(server, flags.Default)
	parser.ShortDescription = "Weaviate"
	server.ConfigureFlags()
	logger.Debug("Command line flags configured", "elapsed", time.Since(startTime))

	logger.Debug("Adding command line option groups")
	for i, optsGroup := range api.CommandLineOptionsGroups {
		logger.Debug("Adding option group", "index", i, "description", optsGroup.ShortDescription)
		_, err := parser.AddGroup(optsGroup.ShortDescription, optsGroup.LongDescription, optsGroup.Options)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to add flag group")
		}
	}
	logger.Debug("All option groups added", "elapsed", time.Since(startTime))

	logger.Debug("Parsing command line arguments")
	if _, err := parser.Parse(); err != nil {
		if fe, ok := err.(*flags.Error); ok && fe.Type == flags.ErrHelp {
			return nil, nil
		}
		return nil, err
	}
	logger.Debug("Command line arguments parsed", "elapsed", time.Since(startTime))

	logger.Debug("Configuring Weaviate API")
	server.ConfigureAPI()
	logger.Info("Weaviate API configured", "elapsed", time.Since(startTime))

	logger.Info("Starting Weaviate server goroutine")
	go func() {
		logger.Debug("Weaviate server.Serve() starting")
		if err := server.Serve(); err != nil && err != http.ErrServerClosed {
			logger.Error("Weaviate serve error", "error", err)
		}
	}()

	go func() {
		<-ctx.Done()
		logger.Debug("Context canceled, shutting down Weaviate server")
		_ = server.Shutdown()
	}()

	// Give the server a moment to start listening before beginning readiness checks
	time.Sleep(100 * time.Millisecond)

	readyURL := fmt.Sprintf("http://localhost:%d/v1/.well-known/ready", p)
	deadline := time.Now().Add(15 * time.Second)
	logger.Info("Waiting for Weaviate to become ready", "url", readyURL, "timeout", "15s")

	checkCount := 0
	for {
		checkCount++
		if time.Now().After(deadline) {
			logger.Error("Weaviate readiness timeout",
				"url", readyURL,
				"elapsed", time.Since(startTime),
				"checks_performed", checkCount)
			return nil, fmt.Errorf("weaviate did not become ready in time on %s", readyURL)
		}

		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, readyURL, nil)
		resp, err := http.DefaultClient.Do(req)

		if err != nil {
			// Log connection errors more frequently for better debugging
			if checkCount <= 5 || checkCount%5 == 0 {
				logger.Debug("Weaviate readiness check failed",
					"error", err,
					"attempt", checkCount,
					"elapsed", time.Since(startTime))
			}
		} else {
			// Always close the response body to prevent resource leaks
			defer func() {
				if resp != nil && resp.Body != nil {
					resp.Body.Close() //nolint:errcheck
				}
			}()

			if resp.StatusCode == http.StatusOK {
				logger.Info("Weaviate server is ready",
					"elapsed", time.Since(startTime),
					"checks_performed", checkCount)
				return server, nil
			} else {
				// Log non-OK status responses more frequently for better debugging
				if checkCount <= 5 || checkCount%5 == 0 {
					logger.Debug("Weaviate not ready yet",
						"status_code", resp.StatusCode,
						"attempt", checkCount,
						"elapsed", time.Since(startTime))
				}
			}
		}

		time.Sleep(200 * time.Millisecond)
	}
}

func InitSchema(client *weaviate.Client, logger *log.Logger) error {
	logger.Debug("Starting schema initialization")
	start := time.Now()

	if err := evolvingmemory.EnsureSchemaExistsInternal(client, logger); err != nil {
		return err
	}

	if err := storage.EnsureContactSchemaExists(client, logger); err != nil {
		return err
	}

	logger.Debug("Schema initialization completed", "elapsed", time.Since(start))
	return nil
}

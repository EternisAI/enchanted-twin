package graphrag

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/EternisAI/enchanted-twin/internal/service/docker"
)

// GraphRAGService manages the GraphRAG Docker container lifecycle
type GraphRAGService struct {
	dockerService *docker.Service
	dataDir       string
	runMode       string
}

// NewGraphRAGService creates a new GraphRAG service instance
func NewGraphRAGService(dataDir string, runMode string, logger *slog.Logger) (*GraphRAGService, error) {
	// Use default logger if none is provided
	if logger == nil {
		logger = slog.Default()
	}

	// Set defaults
	if dataDir == "" {
		dataDir = "./data/graphrag"
	}
	if runMode == "" {
		runMode = "daemon" // Keep container running by default
	}

	// Find project root to locate context path
	pwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	// Assuming we're somewhere in the backend/golang directory, find the project root
	projectRoot := pwd
	// If we're in the golang directory, go up once to backend, then up again to project root
	if filepath.Base(projectRoot) == "golang" {
		projectRoot = filepath.Dir(filepath.Dir(projectRoot))
	} else {
		// Otherwise find the project root by looking for go.mod then going up two levels
		for {
			if _, err := os.Stat(filepath.Join(projectRoot, "go.mod")); err == nil {
				// Found go.mod, now go up two directories to get to project root
				projectRoot = filepath.Dir(filepath.Dir(projectRoot))
				break
			}
			parentDir := filepath.Dir(projectRoot)
			if parentDir == projectRoot {
				return nil, fmt.Errorf("could not find project root")
			}
			projectRoot = parentDir
		}
	}

	// Create the full path to the Dockerfile directory
	dockerfilePath := filepath.Join("backend", "service", "graphrag")
	contextPath := filepath.Join(projectRoot, dockerfilePath)

	// Setup volumes
	volumes := make(map[string]string)
	volumes[filepath.Join(dataDir, "input_data")] = "/app/input_data"
	volumes[filepath.Join(dataDir, "graphrag_root")] = "/app/graphrag_root"

	// Setup environment variables
	envVars := make(map[string]string)
	envVars["RUN_MODE"] = runMode

	// Optional environment variables to control container behavior
	envVars["DO_INIT"] = "auto"
	envVars["DO_INDEX"] = "true"

	// Pass through the OpenAI API key from the environment
	envVars["OPENAI_API_KEY"] = os.Getenv("OPENAI_API_KEY")

	// Create Docker service options
	options := docker.ContainerOptions{
		ImageName:     "enchanted-twin-graphrag",
		ImageTag:      "latest",
		ContextPath:   contextPath,
		ContainerName: "enchanted-twin-graphrag",
		EnvVars:       envVars,
		Volumes:       volumes,
		Detached:      true,
	}

	// Create Docker service
	dockerService, err := docker.NewService(options, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker service: %w", err)
	}

	return &GraphRAGService{
		dockerService: dockerService,
		dataDir:       dataDir,
		runMode:       runMode,
	}, nil
}

// Logger returns the service's logger
func (s *GraphRAGService) Logger() *slog.Logger {
	return s.dockerService.Logger()
}

// CheckImageExists checks if the GraphRAG image exists and returns its creation time
func (s *GraphRAGService) CheckImageExists(ctx context.Context) (bool, time.Time, error) {
	return s.dockerService.CheckImageExists(ctx)
}

// BuildImage builds the GraphRAG Docker image
func (s *GraphRAGService) BuildImage(ctx context.Context) error {
	return s.dockerService.BuildImage(ctx)
}

// StartContainer starts the GraphRAG container
func (s *GraphRAGService) StartContainer(ctx context.Context) error {
	return s.dockerService.StartContainer(ctx)
}

// StopContainer stops the GraphRAG container
func (s *GraphRAGService) StopContainer(ctx context.Context) error {
	return s.dockerService.StopContainer(ctx)
}

// RemoveContainer removes the GraphRAG container
func (s *GraphRAGService) RemoveContainer(ctx context.Context) error {
	return s.dockerService.RemoveContainer(ctx)
}

// GetContainerLogs gets logs from the GraphRAG container
func (s *GraphRAGService) GetContainerLogs(ctx context.Context) (string, error) {
	return s.dockerService.GetContainerLogs(ctx)
}

// ExecuteCommand executes a command in the GraphRAG container
func (s *GraphRAGService) ExecuteCommand(ctx context.Context, command []string) (string, error) {
	return s.dockerService.ExecuteCommand(ctx, command)
}

// Close cleans up any resources
func (s *GraphRAGService) Close() error {
	return s.dockerService.Close()
}

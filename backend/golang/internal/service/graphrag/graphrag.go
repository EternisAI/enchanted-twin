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
	dockerService   *docker.Service
	graphragDataDir string // Directory for GraphRAG data/config
	inputDataDir    string // Directory for input data
}

// NewGraphRAGService creates a new GraphRAG service instance
// The second parameter (formerly runMode) can be used to specify a separate input data directory
// If left empty, inputDataDir will be the same as graphragDataDir
func NewGraphRAGService(graphragDataDir string, inputDataDir string, logger *slog.Logger) (*GraphRAGService, error) {
	// Use default logger if none is provided
	if logger == nil {
		logger = slog.Default()
	}

	// Require valid data directories
	if graphragDataDir == "" {
		return nil, fmt.Errorf("graphrag data directory must be specified")
	}

	// Make graphragDataDir absolute to avoid Docker volume mounting issues
	if !filepath.IsAbs(graphragDataDir) {
		absPath, err := filepath.Abs(graphragDataDir)
		if err != nil {
			return nil, fmt.Errorf("failed to get absolute path for graphrag data directory: %w", err)
		}
		graphragDataDir = absPath
		logger.Info("Converted to absolute path", slog.String("graphragDataDir", graphragDataDir))
	}

	// If input data directory is not specified, use the GraphRAG data directory
	if inputDataDir == "" {
		inputDataDir = graphragDataDir
	}

	// Make inputDataDir absolute if it's not already
	if !filepath.IsAbs(inputDataDir) {
		absPath, err := filepath.Abs(inputDataDir)
		if err != nil {
			return nil, fmt.Errorf("failed to get absolute path for input data directory: %w", err)
		}
		inputDataDir = absPath
		logger.Info("Converted to absolute path", slog.String("inputDataDir", inputDataDir))
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

	// Setup volumes - always use absolute paths for Docker
	volumes := make(map[string]string)

	// Create the input data directory if it doesn't exist
	inputDataPath := filepath.Join(inputDataDir, "input_data")
	if err := os.MkdirAll(inputDataPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create input data directory: %w", err)
	}

	// Ensure path is absolute
	if !filepath.IsAbs(inputDataPath) {
		absPath, err := filepath.Abs(inputDataPath)
		if err != nil {
			return nil, fmt.Errorf("failed to get absolute path for input data directory: %w", err)
		}
		inputDataPath = absPath
	}

	// Create the graphrag root directory if it doesn't exist
	graphragRootDir := filepath.Join(graphragDataDir, "graphrag_root")
	if err := os.MkdirAll(graphragRootDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create graphrag root directory: %w", err)
	}

	// Ensure path is absolute
	if !filepath.IsAbs(graphragRootDir) {
		absPath, err := filepath.Abs(graphragRootDir)
		if err != nil {
			return nil, fmt.Errorf("failed to get absolute path for graphrag root directory: %w", err)
		}
		graphragRootDir = absPath
	}

	logger.Info("Creating directories with absolute paths",
		slog.String("input_data", inputDataPath),
		slog.String("graphrag_root", graphragRootDir))

	volumes[inputDataPath] = "/app/input_data"
	volumes[graphragRootDir] = "/app/graphrag_root"

	// Setup environment variables
	envVars := make(map[string]string)

	// Optional environment variables to control container behavior
	envVars["DO_INIT"] = "auto"
	envVars["DO_INDEX"] = "false"

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
		dockerService:   dockerService,
		graphragDataDir: graphragDataDir,
		inputDataDir:    inputDataDir,
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
	return s.dockerService.RunContainer(ctx)
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

// RunIndexing executes a GraphRAG indexing operation
func (s *GraphRAGService) RunIndexing(ctx context.Context, dataPath string) error {
	logger := s.Logger()
	logger.Info("Running GraphRAG indexing",
		slog.String("data_path", dataPath))

	// Find project root to locate context path
	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Use same approach as in NewGraphRAGService to find project root
	projectRoot := pwd
	if filepath.Base(projectRoot) == "golang" {
		projectRoot = filepath.Dir(filepath.Dir(projectRoot))
	} else {
		for {
			if _, err := os.Stat(filepath.Join(projectRoot, "go.mod")); err == nil {
				projectRoot = filepath.Dir(filepath.Dir(projectRoot))
				break
			}
			parentDir := filepath.Dir(projectRoot)
			if parentDir == projectRoot {
				return fmt.Errorf("could not find project root")
			}
			projectRoot = parentDir
		}
	}

	// Create the full path to the Dockerfile directory
	dockerfilePath := filepath.Join("backend", "service", "graphrag")
	contextPath := filepath.Join(projectRoot, dockerfilePath)

	// Setup environment for indexing
	envVars := make(map[string]string)
	envVars["DO_INIT"] = "auto"
	envVars["DO_INDEX"] = "true"
	envVars["OPENAI_API_KEY"] = os.Getenv("OPENAI_API_KEY")

	// Setup volumes for the data paths
	volumes := make(map[string]string)

	// Create the graphrag root directory if it doesn't exist
	graphragRootDir := filepath.Join(s.graphragDataDir, "graphrag_root")
	if err := os.MkdirAll(graphragRootDir, 0755); err != nil {
		return fmt.Errorf("failed to create graphrag root directory: %w", err)
	}

	// Ensure the path is absolute for Docker
	if !filepath.IsAbs(graphragRootDir) {
		absPath, err := filepath.Abs(graphragRootDir)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for graphrag root directory: %w", err)
		}
		graphragRootDir = absPath
	}

	volumes[graphragRootDir] = "/app/graphrag_root"
	logger.Info("Mounting graphrag data directory",
		slog.String("host_path", graphragRootDir),
		slog.String("container_path", "/app/graphrag_root"))

	// Require a valid input data path
	if dataPath == "" {
		return fmt.Errorf("input data path must be provided")
	}

	// Ensure input data path is absolute
	if !filepath.IsAbs(dataPath) {
		absPath, err := filepath.Abs(dataPath)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for input data directory: %w", err)
		}
		dataPath = absPath
		logger.Info("Converted input data path to absolute path",
			slog.String("dataPath", dataPath))
	}

	// Verify input data path exists
	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		return fmt.Errorf("input data directory does not exist: %s", dataPath)
	}

	volumes[dataPath] = "/app/input_data"
	logger.Info("Mounting input data directory",
		slog.String("host_path", dataPath),
		slog.String("container_path", "/app/input_data"))

	// Create container options
	options := docker.ContainerOptions{
		ImageName:     "enchanted-twin-graphrag",
		ImageTag:      "latest",
		ContextPath:   contextPath,
		ContainerName: "enchanted-twin-graphrag",
		EnvVars:       envVars,
		Volumes:       volumes,
		Detached:      false, // We want to wait for indexing to complete
	}

	// Create Docker service
	dockerService, err := docker.NewService(options, logger)
	if err != nil {
		return fmt.Errorf("failed to create Docker service for indexing: %w", err)
	}
	defer func() {
		if err := dockerService.Close(); err != nil {
			logger.Error("Failed to close Docker service", slog.Any("error", err))
		}
	}()

	// Check if container already exists and remove it if it does
	_ = dockerService.StopContainer(ctx)
	_ = dockerService.RemoveContainer(ctx)

	// Start container and wait for it to complete
	if err := dockerService.RunContainer(ctx); err != nil {
		return fmt.Errorf("failed to run indexing container: %w", err)
	}

	logger.Info("GraphRAG indexing completed successfully")
	return nil
}

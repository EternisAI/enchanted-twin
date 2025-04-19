package bootstrap

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/EternisAI/enchanted-twin/internal/service/graphrag"
)

// GraphRAGConfig holds configuration for the GraphRAG service
type GraphRAGConfig struct {
	GraphRAGRoot string
	InputDataDir string
	ForceRebuild bool
}

// DefaultGraphRAGConfig returns the default configuration for GraphRAG
func DefaultGraphRAGConfig() GraphRAGConfig {
	return GraphRAGConfig{
		GraphRAGRoot: "./data/graphrag", // Use a permanent directory, not a temp folder
		InputDataDir: "./output",
		ForceRebuild: true,
	}
}

// InitGraphRAG initializes and starts the GraphRAG service
func InitGraphRAG(ctx context.Context, config GraphRAGConfig, logger *slog.Logger) (*graphrag.GraphRAGService, error) {
	// Create a new GraphRAG service
	service, err := graphrag.NewGraphRAGService(config.GraphRAGRoot, config.InputDataDir, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create GraphRAG service: %w", err)
	}

	// Check if GraphRAG image exists, build if it doesn't or if forced
	if err := ensureGraphRAGImage(ctx, service, config.ForceRebuild); err != nil {
		// Close Docker client to avoid leaks
		if closeErr := service.Close(); closeErr != nil {
			fmt.Printf("warning: failed to close Docker client: %v\n", closeErr)
		}
		return nil, fmt.Errorf("failed to ensure GraphRAG image exists: %w", err)
	}

	// Start the GraphRAG container
	if err := service.StartContainer(ctx); err != nil {
		// Close Docker client to avoid leaks
		if closeErr := service.Close(); closeErr != nil {
			fmt.Printf("warning: failed to close Docker client: %v\n", closeErr)
		}
		return nil, fmt.Errorf("failed to start GraphRAG container: %w", err)
	}

	// Return the service
	return service, nil
}

// ensureGraphRAGImage ensures the GraphRAG Docker image exists, building it if necessary or if forced
func ensureGraphRAGImage(ctx context.Context, service *graphrag.GraphRAGService, forceRebuild bool) error {
	// Check if image exists
	imageExists, _, err := service.CheckImageExists(ctx)
	if err != nil {
		return fmt.Errorf("failed to check if GraphRAG image exists: %w", err)
	}

	// Build image if it doesn't exist or if forced rebuild is requested
	if !imageExists {
		service.Logger().Info("GraphRAG image doesn't exist, building it")

		// Stop and remove any existing container with the same name
		// Ignore errors since container might not exist
		_ = service.StopContainer(ctx)
		_ = service.RemoveContainer(ctx)

		if err := service.BuildImage(ctx); err != nil {
			return fmt.Errorf("failed to build GraphRAG image: %w", err)
		}
		return nil
	}

	// If image exists but force rebuild is enabled
	if forceRebuild {
		service.Logger().Info("Forcing GraphRAG image rebuild - removing any existing container first")

		// First try to stop and remove any existing container to avoid conflicts
		// Ignore errors since container might not exist
		_ = service.StopContainer(ctx)
		_ = service.RemoveContainer(ctx)

		// Now rebuild the image
		if err := service.BuildImage(ctx); err != nil {
			return fmt.Errorf("failed to rebuild GraphRAG image: %w", err)
		}
		return nil
	}

	// Image exists and no rebuild requested
	service.Logger().Info("GraphRAG image exists, using existing image")
	return nil
}

// ShutdownGraphRAG stops and cleans up the GraphRAG service
func ShutdownGraphRAG(ctx context.Context, service *graphrag.GraphRAGService) error {
	if service == nil {
		return nil
	}

	// Stop the container
	if err := service.StopContainer(ctx); err != nil {
		return fmt.Errorf("failed to stop GraphRAG container: %w", err)
	}

	// Close the Docker client
	if err := service.Close(); err != nil {
		return fmt.Errorf("failed to close Docker client: %w", err)
	}

	return nil
}

package docker

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/log"
)

// ContainerOptions represents options for running a Docker container
type ContainerOptions struct {
	// Image settings
	ImageName   string
	ImageTag    string
	ContextPath string

	// Container settings
	ContainerName string
	EnvVars       map[string]string
	Ports         map[string]string // host:container
	Volumes       map[string]string // host:container
	Command       []string

	// Runtime options
	Detached      bool
	RestartPolicy string
	NetworkMode   string
}

// Service manages Docker containers using the Docker CLI
type Service struct {
	options ContainerOptions
	logger  *log.Logger
}

// NewService creates a new Docker service
func NewService(options ContainerOptions, logger *log.Logger) (*Service, error) {
	// Check if Docker is available
	cmd := exec.Command("docker", "--version")
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("docker not available: %w", err)
	}

	// Validate options
	if options.ImageName == "" {
		return nil, fmt.Errorf("image name is required")
	}

	// Set defaults
	if options.ImageTag == "" {
		options.ImageTag = "latest"
	}
	if options.ContainerName == "" {
		options.ContainerName = fmt.Sprintf("%s-container", strings.ReplaceAll(options.ImageName, "/", "-"))
	}
	if options.EnvVars == nil {
		options.EnvVars = make(map[string]string)
	}
	if options.Volumes == nil {
		options.Volumes = make(map[string]string)
	}
	if options.Ports == nil {
		options.Ports = make(map[string]string)
	}

	return &Service{
		options: options,
		logger:  logger,
	}, nil
}

// Logger returns the service's logger
func (s *Service) Logger() *log.Logger {
	return s.logger
}

// FullImageName returns the full image name with tag
func (s *Service) FullImageName() string {
	return fmt.Sprintf("%s:%s", s.options.ImageName, s.options.ImageTag)
}

// CheckImageExists checks if the Docker image exists and returns its creation time
func (s *Service) CheckImageExists(ctx context.Context) (bool, time.Time, error) {
	cmd := exec.CommandContext(
		ctx,
		"docker",
		"image",
		"inspect",
		"--format", "{{.Created}}",
		s.FullImageName(),
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Check if it's a "not found" error
		if strings.Contains(stderr.String(), "No such image") {
			return false, time.Time{}, nil
		}
		return false, time.Time{}, fmt.Errorf("failed to inspect image: %s: %w", stderr.String(), err)
	}

	// Parse the creation time
	createdStr := strings.TrimSpace(stdout.String())
	if createdStr == "" {
		return true, time.Time{}, nil
	}

	created, err := time.Parse(time.RFC3339, createdStr)
	if err != nil {
		s.logger.Warn("Failed to parse image creation time", slog.String("time", createdStr), slog.Any("error", err))
		return true, time.Time{}, nil
	}

	return true, created, nil
}

// BuildImage builds the Docker image
func (s *Service) BuildImage(ctx context.Context) error {
	// Ensure context directory exists
	if s.options.ContextPath != "" {
		if _, err := os.Stat(s.options.ContextPath); err != nil {
			return fmt.Errorf("context directory not found: %w", err)
		}
	} else {
		return fmt.Errorf("context path is required for building an image")
	}

	s.logger.Info("Building Docker image (with cache disabled)",
		slog.String("image", s.FullImageName()),
		slog.String("context_path", s.options.ContextPath))

	// Build the image using docker CLI
	args := []string{
		"build",
		// "--no-cache", // Force rebuild without using cache
		"-t", s.FullImageName(),
	}

	// Add any additional build arguments
	if s.options.ContextPath != "" {
		args = append(args, s.options.ContextPath)
	} else {
		args = append(args, ".")
	}

	cmd := exec.CommandContext(ctx, "docker", args...)

	// Connect pipes
	var stderr bytes.Buffer
	cmd.Stdout = os.Stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker build failed: %s: %w", stderr.String(), err)
	}

	s.logger.Info("Successfully built Docker image", slog.String("image", s.FullImageName()))
	return nil
}

// RunContainer starts the Docker container
func (s *Service) RunContainer(ctx context.Context) error {
	// Check if container already exists
	cmd := exec.CommandContext(
		ctx,
		"docker",
		"ps",
		"-a",
		"--filter", fmt.Sprintf("name=%s", s.options.ContainerName),
		"--format", "{{.ID}}|{{.Status}}",
	)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to check if container exists: %w", err)
	}

	containerInfo := strings.TrimSpace(stdout.String())
	if containerInfo != "" {
		// Container exists
		parts := strings.Split(containerInfo, "|")
		containerID := parts[0]
		status := parts[1]

		if strings.HasPrefix(status, "Up") {
			s.logger.Info("Container is already running", "container", s.options.ContainerName)
			return nil
		}

		// Container exists but is not running
		s.logger.Info("Starting existing container", slog.String("container", s.options.ContainerName))
		cmd = exec.CommandContext(ctx, "docker", "start", containerID)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to start existing container: %w", err)
		}

		s.logger.Info("Started existing container", slog.String("container", s.options.ContainerName))
		return nil
	}

	// Container doesn't exist, create and start it
	// Ensure host directories for volumes exist
	s.logger.Debug("Creating host directories for volumes")
	for hostPath, containerPath := range s.options.Volumes {
		// Only create directories if they're absolute paths
		if filepath.IsAbs(hostPath) {
			s.logger.Debug("Creating directory",
				slog.String("host_path", hostPath),
				slog.String("container_path", containerPath))

			if err := os.MkdirAll(hostPath, 0755); err != nil {
				return fmt.Errorf("failed to create host directory for volume: %w", err)
			}
		}
	}

	// Create and start the container
	args := []string{"run"}

	if s.options.Detached {
		args = append(args, "-d")
	} else {
		args = append(args, "-i")
	}

	// Container name
	args = append(args, "--name", s.options.ContainerName)

	// Environment variables
	for name, value := range s.options.EnvVars {
		args = append(args, "-e", fmt.Sprintf("%s=%s", name, value))
	}

	// Volumes
	for hostPath, containerPath := range s.options.Volumes {
		args = append(args, "-v", fmt.Sprintf("%s:%s", hostPath, containerPath))
	}

	// Ports
	for hostPort, containerPort := range s.options.Ports {
		args = append(args, "-p", fmt.Sprintf("%s:%s", hostPort, containerPort))
	}

	// Restart policy
	if s.options.RestartPolicy != "" {
		args = append(args, "--restart", s.options.RestartPolicy)
	}

	// Network mode
	if s.options.NetworkMode != "" {
		args = append(args, "--network", s.options.NetworkMode)
	}

	// Image name
	args = append(args, s.FullImageName())

	// Command override
	if len(s.options.Command) > 0 {
		args = append(args, s.options.Command...)
	}

	cmd = exec.CommandContext(ctx, "docker", args...)
	var stderr bytes.Buffer
	cmd.Stdout = os.Stdout
	cmd.Stderr = &stderr
	cmd.Stdin = os.Stdin

	s.logger.Debug("Running docker cmd", "docker", strings.Join(args, " "))

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create and start container: %s: %w", stderr.String(), err)
	}

	s.logger.Debug("Started container", "container", s.options.ContainerName)
	return nil
}

// StopContainer stops the Docker container
func (s *Service) StopContainer(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "stop", s.options.ContainerName)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop container: %s: %w", stderr.String(), err)
	}

	s.logger.Info("Stopped container", slog.String("container", s.options.ContainerName))
	return nil
}

// RemoveContainer removes the Docker container
func (s *Service) RemoveContainer(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "rm", "-f", s.options.ContainerName)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to remove container: %s: %w", stderr.String(), err)
	}

	s.logger.Info("Removed container", "container", s.options.ContainerName)
	return nil
}

// GetContainerLogs gets logs from the Docker container
func (s *Service) GetContainerLogs(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", "logs", "--tail", "100", s.options.ContainerName)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to get container logs: %s: %w", stderr.String(), err)
	}

	return stdout.String(), nil
}

// ExecuteCommand executes a command in the Docker container
func (s *Service) ExecuteCommand(ctx context.Context, command []string) (string, error) {
	args := append([]string{"exec", s.options.ContainerName}, command...)
	cmd := exec.CommandContext(ctx, "docker", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to execute command: %s: %w", stderr.String(), err)
	}

	return stdout.String(), nil
}

// Close performs any cleanup needed
func (s *Service) Close() error {
	// No resources to clean up
	return nil
}

// Options returns a copy of the service's container options
func (s *Service) Options() ContainerOptions {
	return s.options
}

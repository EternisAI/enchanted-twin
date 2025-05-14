package podman

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField("component", "podman")

// PodmanManager handles interactions with the Podman container runtime.
type PodmanManager interface {
	// IsInstalled checks if Podman is installed on the system
	IsInstalled(ctx context.Context) (bool, error)

	// IsMachineInstalled checks if a Podman machine exists
	IsMachineInstalled(ctx context.Context) (bool, error)

	// IsMachineRunning checks if a Podman machine is running
	IsMachineRunning(ctx context.Context) (bool, error)

	// IsContainerRunning checks if a container is running
	IsContainerRunning(ctx context.Context, containerID string) (bool, error)

	// CheckContainerExists checks if a container exists
	CheckContainerExists(ctx context.Context, containerName string) (bool, string, error)

	// PullImage pulls the specified image
	PullImage(ctx context.Context, imageURL string) error

	// RunContainer runs a container from the specified image
	// and returns the container ID if successful
	RunContainer(ctx context.Context, containerConfig ContainerConfig) (string, error)

	// StartContainer starts an existing container
	StartContainer(ctx context.Context, containerID string) error

	// RemoveContainer removes a container
	RemoveContainer(ctx context.Context, containerID string) error

	// StopContainer stops a container
	StopContainer(ctx context.Context, containerID string) error

	// CleanupContainer cleans up a container
	CleanupContainer(ctx context.Context, containerName string) error

	// ExecCommand executes a command with the given arguments
	ExecCommand(ctx context.Context, cmd string, args []string) (string, error)
}

// ContainerConfig defines configuration for a container.
type ContainerConfig struct {
	ImageURL     string            // Image URL to run
	Name         string            // Container name
	Command      []string          // Command to run in container
	Ports        map[string]string // Port mappings (host:container)
	Volumes      map[string]string // Volume mappings (host:container)
	Environment  map[string]string // Environment variables
	PullIfNeeded bool              // Pull image if it doesn't exist
	ExtraArgs    []string          // Additional arguments to pass to the container
}

// DefaultManager is the standard implementation of PodmanManager.
type DefaultManager struct {
	// The executable name to use
	executable string
	// The default timeout for commands
	defaultTimeout time.Duration
	// The default machine name
	defaultMachine string
}

// NewManager creates a new PodmanManager.
func NewManager() PodmanManager {
	return &DefaultManager{
		executable:     "podman",
		defaultTimeout: 60 * time.Second,
		defaultMachine: "podman-machine-default",
	}
}

// IsInstalled checks if Podman is installed.
func (m *DefaultManager) IsInstalled(ctx context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, m.defaultTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, m.executable, "--version")
	err := cmd.Run()
	if err != nil {
		log.WithError(err).Debug("Podman not installed or not in PATH")
		return false, nil
	}
	return true, nil
}

// IsMachineInstalled checks if a Podman machine exists.
func (m *DefaultManager) IsMachineInstalled(ctx context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, m.defaultTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, m.executable, "machine", "list", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		log.WithError(err).Debug("Failed to list Podman machines")
		return false, errors.Wrap(err, "failed to list Podman machines")
	}

	var machines []map[string]interface{}
	if err := json.Unmarshal(output, &machines); err != nil {
		log.WithError(err).Debug("Failed to parse machine list output")
		return false, errors.Wrap(err, "failed to parse machine list output")
	}

	for _, machine := range machines {
		if name, ok := machine["Name"].(string); ok && name == m.defaultMachine {
			return true, nil
		}
	}

	return false, nil
}

// IsMachineRunning checks if a Podman machine is running.
func (m *DefaultManager) IsMachineRunning(ctx context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, m.defaultTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, m.executable, "machine", "list", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		log.WithError(err).Debug("Failed to list Podman machines")
		return false, errors.Wrap(err, "failed to list Podman machines")
	}

	var machines []map[string]interface{}
	if err := json.Unmarshal(output, &machines); err != nil {
		log.WithError(err).Debug("Failed to parse machine list output")
		return false, errors.Wrap(err, "failed to parse machine list output")
	}

	for _, machine := range machines {
		name, nameOk := machine["Name"].(string)
		running, runningOk := machine["Running"].(bool)

		if nameOk && runningOk && name == m.defaultMachine && running {
			return true, nil
		}
	}

	return false, nil
}

func (m *DefaultManager) IsContainerRunning(ctx context.Context, containerID string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, m.defaultTimeout)
	defer cancel()

	cmd := m.executable
	args := []string{"container", "inspect", containerID, "--format", "{{.State.Running}}"}

	execCmd := exec.CommandContext(ctx, cmd, args...)
	output, err := execCmd.Output()
	if err != nil {
		log.WithError(err).Debug("Failed to inspect container")
		return false, err
	}

	return strings.TrimSpace(string(output)) == "true", nil
}

func (m *DefaultManager) CheckContainerExists(ctx context.Context, containerName string) (bool, string, error) {
	ctx, cancel := context.WithTimeout(ctx, m.defaultTimeout)

	defer cancel()

	cmd := m.executable
	args := []string{"container", "ls", "-a", "--filter", fmt.Sprintf("name=%s", containerName), "--format", "{{.ID}}"}

	execCmd := exec.CommandContext(ctx, cmd, args...)
	output, err := execCmd.Output()
	if err != nil {
		log.WithError(err).Debug("Failed to list containers")
		return false, "", err
	}

	containerID := strings.TrimSpace(string(output))
	return containerID != "", containerID, nil
}

// PullImage pulls the specified image.
func (m *DefaultManager) PullImage(ctx context.Context, imageURL string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute) // Longer timeout for image pulls
	defer cancel()

	log.WithField("image", imageURL).Info("Pulling image")
	cmd := exec.CommandContext(ctx, m.executable, "pull", imageURL)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.WithError(err).WithField("output", string(output)).Error("Failed to pull image")
		return errors.Wrapf(err, "failed to pull image: %s", string(output))
	}

	log.WithField("image", imageURL).Info("Image pulled successfully")
	return nil
}

// RunContainer runs a container from the specified image.
func (m *DefaultManager) RunContainer(ctx context.Context, config ContainerConfig) (string, error) {
	if config.PullIfNeeded {
		imageExists, err := m.imageExists(ctx, config.ImageURL)
		if err != nil {
			return "", errors.Wrap(err, "failed to check if image exists")
		}

		if !imageExists {
			log.WithField("image", config.ImageURL).Info("Image not found, pulling")
			if err := m.PullImage(ctx, config.ImageURL); err != nil {
				return "", errors.Wrap(err, "failed to pull image")
			}
		}
	}

	args := []string{"run", "-d"}

	if config.Name != "" {
		args = append(args, "--name", config.Name)
	}

	for host, container := range config.Ports {
		args = append(args, "-p", fmt.Sprintf("%s:%s", host, container))
	}

	for host, container := range config.Volumes {
		args = append(args, "-v", fmt.Sprintf("%s:%s", host, container))
	}

	for key, value := range config.Environment {
		args = append(args, "-e", fmt.Sprintf("%s=%s", key, value))
	}

	args = append(args, config.ImageURL)

	if len(config.Command) > 0 {
		args = append(args, config.Command...)
	}

	if len(config.ExtraArgs) > 0 {
		args = append(args, config.ExtraArgs...)
	}

	ctx, cancel := context.WithTimeout(ctx, m.defaultTimeout)
	defer cancel()

	log.WithField("args", args).Debug("Running container")
	cmd := exec.CommandContext(ctx, m.executable, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.WithError(err).WithField("output", string(output)).Error("Failed to run container")
		return "", errors.Wrapf(err, "failed to run container: %s", string(output))
	}

	containerID := strings.TrimSpace(string(output))
	log.WithField("containerId", containerID).Info("Container started successfully")
	return containerID, nil
}

// StartContainer starts an existing container.
func (m *DefaultManager) StartContainer(ctx context.Context, containerID string) error {
	ctx, cancel := context.WithTimeout(ctx, m.defaultTimeout)
	defer cancel()

	cmd := m.executable
	args := []string{"container", "start", containerID}

	execCmd := exec.CommandContext(ctx, cmd, args...)
	output, err := execCmd.CombinedOutput()
	if err != nil {
		log.WithError(err).WithField("output", string(output)).Debug("Failed to start container")
		return fmt.Errorf("failed to start container: %w", err)
	}

	return nil
}

func (m *DefaultManager) StopContainer(ctx context.Context, containerID string) error {
	ctx, cancel := context.WithTimeout(ctx, m.defaultTimeout)
	defer cancel()

	cmd := m.executable
	args := []string{"container", "stop", containerID}

	execCmd := exec.CommandContext(ctx, cmd, args...)
	output, err := execCmd.CombinedOutput()
	if err != nil {
		log.WithError(err).WithField("output", string(output)).Debug("Failed to stop container")
		return fmt.Errorf("failed to stop container: %w", err)
	}

	return nil
}

// RemoveContainer removes a container.
func (m *DefaultManager) RemoveContainer(ctx context.Context, containerID string) error {
	ctx, cancel := context.WithTimeout(ctx, m.defaultTimeout)
	defer cancel()

	cmd := m.executable
	args := []string{"container", "rm", "-f", containerID}

	execCmd := exec.CommandContext(ctx, cmd, args...)
	output, err := execCmd.CombinedOutput()
	if err != nil {
		log.WithError(err).WithField("output", string(output)).Debug("Failed to remove container")
		return fmt.Errorf("failed to remove container: %w", err)
	}

	return nil
}

// This is useful for cleaning up when the application shuts down.
func (m *DefaultManager) CleanupContainer(ctx context.Context, containerName string) error {
	ctx, cancel := context.WithTimeout(ctx, m.defaultTimeout)
	defer cancel()

	cmd := m.executable
	args := []string{"container", "ls", "-a", "--filter", fmt.Sprintf("name=%s", containerName), "--format", "{{.ID}}"}

	execCmd := exec.CommandContext(ctx, cmd, args...)
	output, err := execCmd.Output()
	if err != nil {
		log.WithError(err).Debug("Failed to list  containers")
		return fmt.Errorf("failed to list containers: %w", err)
	}

	containerIDs := strings.Split(strings.TrimSpace(string(output)), "\n")

	if len(containerIDs) == 0 || (len(containerIDs) == 1 && containerIDs[0] == "") {
		log.Debug("No containers found to clean up")
		return nil
	}

	log.WithField("count", len(containerIDs)).
		WithField("ids", containerIDs).
		Info("Found containers to clean up")

	for _, containerID := range containerIDs {
		if containerID == "" {
			continue
		}

		log.WithField("containerId", containerID).Info("Cleaning up container")

		rmCmd := exec.CommandContext(ctx, cmd, "container", "rm", "-f", containerID)
		rmOutput, err := rmCmd.CombinedOutput()
		if err != nil {
			log.WithError(err).
				WithField("output", string(rmOutput)).
				WithField("containerId", containerID).
				Warn("Failed to remove container during cleanup")
		} else {
			log.WithField("containerId", containerID).Info("Successfully removed container")
		}
	}

	return nil
}

// imageExists checks if an image exists locally.
func (m *DefaultManager) imageExists(ctx context.Context, imageURL string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, m.defaultTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, m.executable, "image", "exists", imageURL)
	err := cmd.Run()
	return err == nil, nil
}

// ExecCommand executes a command with the given arguments.
func (m *DefaultManager) ExecCommand(ctx context.Context, cmd string, args []string) (string, error) {
	execCmd := exec.CommandContext(ctx, cmd, args...)
	output, err := execCmd.CombinedOutput()
	if err != nil {
		return "", errors.Wrapf(err, "failed to execute command: %s", string(output))
	}
	return string(output), nil
}

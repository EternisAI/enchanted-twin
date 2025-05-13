package podman

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/pkg/errors"
)

const (
	// DefaultKokoroImage is the default Kokoro image URL.
	DefaultKokoroImage = "ghcr.io/remsky/kokoro-fastapi-cpu:latest"

	// DefaultKokoroContainerName is the default name for the Kokoro container.
	DefaultKokoroContainerName = "kokoro-fastapi"

	// DefaultKokoroPort is the default port the Kokoro API listens on.
	DefaultKokoroPort = "8880"
)

// KokoroManager provides specialized functions for handling the Kokoro container.
type KokoroManager struct {
	podman PodmanManager
}

// NewKokoroManager creates a new KokoroManager.
func NewKokoroManager() *KokoroManager {
	return &KokoroManager{
		podman: NewManager(),
	}
}

// VerifyPodmanInstalled checks if Podman is installed.
func (k *KokoroManager) VerifyPodmanInstalled(ctx context.Context) (bool, error) {
	return k.podman.IsInstalled(ctx)
}

// VerifyPodmanMachineInstalled checks if a Podman machine exists.
func (k *KokoroManager) VerifyPodmanMachineInstalled(ctx context.Context) (bool, error) {
	return k.podman.IsMachineInstalled(ctx)
}

// VerifyPodmanRunning checks if a Podman machine is running.
func (k *KokoroManager) VerifyPodmanRunning(ctx context.Context) (bool, error) {
	return k.podman.IsMachineRunning(ctx)
}

// PullKokoroImage pulls the Kokoro image.
func (k *KokoroManager) PullKokoroImage(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute) // Long timeout for big images
	defer cancel()

	log.WithField("image", DefaultKokoroImage).Info("Pulling Kokoro image")
	return k.podman.PullImage(ctx, DefaultKokoroImage)
}

// Returns the container ID if successful.
func (k *KokoroManager) RunKokoroContainer(ctx context.Context, hostPort string) (string, error) {
	// First check if Podman is running
	running, err := k.VerifyPodmanRunning(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to verify if Podman is running")
	}

	if !running {
		return "", errors.New("podman machine is not running")
	}

	// Configure the container
	containerConfig := ContainerConfig{
		ImageURL:     DefaultKokoroImage,
		Name:         DefaultKokoroContainerName,
		Ports:        map[string]string{hostPort: DefaultKokoroPort},
		Environment:  map[string]string{},
		PullIfNeeded: true, // Pull the image if it doesn't exist
	}

	// Run the container
	containerID, err := k.podman.RunContainer(ctx, containerConfig)
	if err != nil {
		return "", errors.Wrap(err, "failed to run Kokoro container")
	}

	// Wait a moment for the container to fully start
	time.Sleep(2 * time.Second)

	// Check if the container is running
	if !k.isContainerRunning(ctx, containerID) {
		return "", fmt.Errorf("container failed to start properly: %s", containerID)
	}

	log.WithField("containerId", containerID).
		WithField("port", hostPort).
		Info("Kokoro container started successfully")

	return containerID, nil
}

// isContainerRunning checks if a container is running.
func (k *KokoroManager) isContainerRunning(ctx context.Context, containerID string) bool {
	m, ok := k.podman.(*DefaultManager)
	if !ok {
		return false
	}

	ctx, cancel := context.WithTimeout(ctx, m.defaultTimeout)
	defer cancel()

	cmd := m.executable
	args := []string{"container", "inspect", containerID, "--format", "{{.State.Running}}"}

	execCmd := exec.CommandContext(ctx, cmd, args...)
	output, err := execCmd.Output()
	if err != nil {
		log.WithError(err).Debug("Failed to inspect container")
		return false
	}

	return strings.TrimSpace(string(output)) == "true"
}

// IsContainerRunning checks if a container is running (exported version).
func (k *KokoroManager) IsContainerRunning(ctx context.Context, containerID string) (bool, error) {
	m, ok := k.podman.(*DefaultManager)
	if !ok {
		return false, fmt.Errorf("invalid podman manager type")
	}

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

// Returns: exists (bool), containerID (string), error.
func (k *KokoroManager) CheckContainerExists(ctx context.Context, containerName string) (bool, string, error) {
	m, ok := k.podman.(*DefaultManager)
	if !ok {
		return false, "", fmt.Errorf("invalid podman manager type")
	}

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

// StartContainer starts an existing container.
func (k *KokoroManager) StartContainer(ctx context.Context, containerID string) error {
	m, ok := k.podman.(*DefaultManager)
	if !ok {
		return fmt.Errorf("invalid podman manager type")
	}

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

// RemoveContainer removes a container.
func (k *KokoroManager) RemoveContainer(ctx context.Context, containerID string) error {
	m, ok := k.podman.(*DefaultManager)
	if !ok {
		return fmt.Errorf("invalid podman manager type")
	}

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
func (k *KokoroManager) CleanupKokoroContainers(ctx context.Context) error {
	m, ok := k.podman.(*DefaultManager)
	if !ok {
		return fmt.Errorf("invalid podman manager type")
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(ctx, m.defaultTimeout)
	defer cancel()

	// Find all containers with the kokoro name
	cmd := m.executable
	args := []string{"container", "ls", "-a", "--filter", fmt.Sprintf("name=%s", DefaultKokoroContainerName), "--format", "{{.ID}}"}

	execCmd := exec.CommandContext(ctx, cmd, args...)
	output, err := execCmd.Output()
	if err != nil {
		log.WithError(err).Debug("Failed to list Kokoro containers")
		return fmt.Errorf("failed to list Kokoro containers: %w", err)
	}

	containerIDs := strings.Split(strings.TrimSpace(string(output)), "\n")

	// No containers found
	if len(containerIDs) == 0 || (len(containerIDs) == 1 && containerIDs[0] == "") {
		log.Debug("No Kokoro containers found to clean up")
		return nil
	}

	log.WithField("count", len(containerIDs)).
		WithField("ids", containerIDs).
		Info("Found Kokoro containers to clean up")

	// Remove each container
	for _, containerID := range containerIDs {
		if containerID == "" {
			continue
		}

		log.WithField("containerId", containerID).Info("Cleaning up Kokoro container")

		// Force remove the container
		rmCmd := exec.CommandContext(ctx, cmd, "container", "rm", "-f", containerID)
		rmOutput, err := rmCmd.CombinedOutput()
		if err != nil {
			log.WithError(err).
				WithField("output", string(rmOutput)).
				WithField("containerId", containerID).
				Warn("Failed to remove Kokoro container during cleanup")
		} else {
			log.WithField("containerId", containerID).Info("Successfully removed Kokoro container")
		}
	}

	return nil
}

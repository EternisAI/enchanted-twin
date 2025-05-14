package container

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField("component", "podman")

// DefaultManager implements the ContainerManager interface for the Podman
// container runtime. The majority of the logic remains unchanged – only the
// type name is updated to avoid confusion with the new generic
// ContainerManager interface.

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
	// imageProgress stores the latest overall progress percentage for a given image URL.
	imageProgress map[string]int
	progressMu    sync.RWMutex
}

// newPodmanManager creates a ContainerManager backed by the Podman CLI. It is
// used internally by NewManager() in manager.go when the CONTAINER_RUNTIME
// environment variable is unset or set to "podman".
func newPodmanManager() ContainerManager {
	return &DefaultManager{
		executable:     "podman",
		defaultTimeout: 60 * time.Second,
		defaultMachine: "podman-machine-default",
		imageProgress:  make(map[string]int),
	}
}

// Ensure DefaultManager satisfies ContainerManager at compile-time
var _ ContainerManager = (*DefaultManager)(nil)

// Executable returns the CLI executable used by this manager ("podman").
func (m *DefaultManager) Executable() string {
	return m.executable
}

// DefaultTimeout returns the default command timeout for this manager.
func (m *DefaultManager) DefaultTimeout() time.Duration {
	return m.defaultTimeout
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

// PullImage starts pulling an image and streams the overall progress percentage (0-100)
// on a buffered channel. The returned channel is closed once the pull completes
// (successfully or with an error). If callers are not interested in consumption,
// the send operations are non-blocking to avoid goroutine leaks.
func (m *DefaultManager) PullImage(ctx context.Context, imageURL string) (<-chan int, error) {
	progressCh := make(chan int, 1) // small buffer so we never block

	// If the image already exists locally signal immediate completion.
	exists, err := m.imageExists(ctx, imageURL)
	if err != nil {
		close(progressCh)
		return progressCh, err
	}
	if exists {
		// send 100% then close
		progressCh <- 100
		close(progressCh)
		m.progressMu.Lock()
		m.imageProgress[imageURL] = 100
		m.progressMu.Unlock()
		return progressCh, nil
	}

	// Run the pull in its own goroutine so the caller gets the channel immediately.
	go func() {
		defer func() {
			// make sure channel is closed even on panic
			if r := recover(); r != nil {
				log.WithField("recover", r).Error("panic in PullImage goroutine")
			}
			close(progressCh)
		}()

		// Podman prints progress information to stderr in plain text. We start the
		// command without a timeout here because the context provided by the caller
		// already contains one (the caller can cancel at will).
		cmd := exec.CommandContext(ctx, m.executable, "pull", imageURL)

		stdoutPipe, _ := cmd.StdoutPipe()
		stderrPipe, _ := cmd.StderrPipe()

		if err := cmd.Start(); err != nil {
			log.WithError(err).Error("Failed to start podman pull command")
			return
		}

		// Combine both stdout & stderr so we parse all output.
		scanner := bufio.NewScanner(io.MultiReader(stdoutPipe, stderrPipe))
		percentRe := regexp.MustCompile(`(\d{1,3})%`)

		for scanner.Scan() {
			line := scanner.Text()
			if matches := percentRe.FindStringSubmatch(line); matches != nil {
				if p, perr := strconv.Atoi(matches[1]); perr == nil {
					if p > 100 {
						p = 100
					}
					// update internal cache
					m.progressMu.Lock()
					m.imageProgress[imageURL] = p
					m.progressMu.Unlock()

					// non-blocking send so we don't deadlock if nobody is listening
					select {
					case progressCh <- p:
					default:
					}
				}
			}
		}

		// Wait for command completion
		_ = cmd.Wait()

		// make sure we store 100% at the end
		m.progressMu.Lock()
		m.imageProgress[imageURL] = 100
		m.progressMu.Unlock()

		select {
		case progressCh <- 100:
		default:
		}
	}()

	return progressCh, nil
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
			_, err := m.PullImage(ctx, config.ImageURL)
			if err != nil {
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

func (m *DefaultManager) GetImageProgress(ctx context.Context, imageURL string) (int, error) {
	// First attempt to read cached progress from the pull goroutine.
	m.progressMu.RLock()
	if p, ok := m.imageProgress[imageURL]; ok {
		m.progressMu.RUnlock()
		return p, nil
	}
	m.progressMu.RUnlock()

	// Fallback: image already exists (100) or not yet pulled (0).
	exists, err := m.imageExists(ctx, imageURL)
	if err != nil {
		return 0, err
	}
	if exists {
		return 100, nil
	}
	return 0, nil
}

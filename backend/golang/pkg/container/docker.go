package container

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var dockerLog = logrus.WithField("component", "docker")

// DockerManager implements the ContainerManager interface using the Docker CLI.
// Most Docker sub-commands mirror Podman, so we reuse nearly all of the logic
// from the DefaultManager implementation. Where Docker differs (or lacks
// functionality) we provide Docker-specific overrides.
type DockerManager struct {
	executable     string
	defaultTimeout time.Duration
	// dummy field to keep structure similar; not used by Docker
	defaultMachine string
	// imageProgress caches the overall download percentage for images currently being pulled
	imageProgress map[string]int
	progressMu    sync.RWMutex
}

// newDockerManager is called from NewManager() when CONTAINER_RUNTIME=docker.
func newDockerManager() ContainerManager {
	return &DockerManager{
		executable:     "docker",
		defaultTimeout: 60 * time.Second,
		defaultMachine: "docker-daemon", // placeholder – Docker has no machine concept
		imageProgress:  make(map[string]int),
	}
}

// Ensure DockerManager satisfies ContainerManager at compile-time.
var _ ContainerManager = (*DockerManager)(nil)

// helper to wrap exec.CommandContext creation
func (m *DockerManager) commandContext(ctx context.Context, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, m.executable, args...)
}

// Executable returns the underlying CLI executable – "docker".
func (m *DockerManager) Executable() string {
	return m.executable
}

// DefaultTimeout returns the default timeout to use for CLI calls.
func (m *DockerManager) DefaultTimeout() time.Duration {
	return m.defaultTimeout
}

// IsInstalled checks if the Docker CLI is available.
func (m *DockerManager) IsInstalled(ctx context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, m.defaultTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, m.executable, "--version")
	if err := cmd.Run(); err != nil {
		dockerLog.WithError(err).Debug("Docker not installed or not in PATH")
		return false, nil
	}
	return true, nil
}

// Docker has no "machine" concept – always report installed.
func (m *DockerManager) IsMachineInstalled(ctx context.Context) (bool, error) {
	return true, nil
}

// Docker assumes the daemon is running if the CLI responds.
func (m *DockerManager) IsMachineRunning(ctx context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, m.defaultTimeout)
	defer cancel()

	if err := m.commandContext(ctx, "info").Run(); err != nil {
		return false, nil
	}
	return true, nil
}

func (m *DockerManager) IsContainerRunning(ctx context.Context, containerID string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, m.defaultTimeout)
	defer cancel()

	args := []string{"container", "inspect", containerID, "--format", "{{.State.Running}}"}
	output, err := m.commandContext(ctx, args...).Output()
	if err != nil {
		dockerLog.WithError(err).Debug("Failed to inspect container")
		return false, err
	}
	return strings.TrimSpace(string(output)) == "true", nil
}

func (m *DockerManager) CheckContainerExists(ctx context.Context, containerName string) (bool, string, error) {
	ctx, cancel := context.WithTimeout(ctx, m.defaultTimeout)
	defer cancel()

	args := []string{"container", "ls", "-a", "--filter", fmt.Sprintf("name=%s", containerName), "--format", "{{.ID}}"}
	output, err := m.commandContext(ctx, args...).Output()
	if err != nil {
		dockerLog.WithError(err).Debug("Failed to list containers")
		return false, "", err
	}

	containerID := strings.TrimSpace(string(output))
	return containerID != "", containerID, nil
}

// PullImage behaves like the Podman implementation: it starts the download and emits overall
// progress (0-100) on the returned channel. If the image already exists locally, the channel
// immediately receives 100 and is closed.
func (m *DockerManager) PullImage(ctx context.Context, imageURL string) (<-chan int, error) {
	progressCh := make(chan int, 1)

	// Check if already present
	exists, err := m.imageExists(ctx, imageURL)
	if err != nil {
		close(progressCh)
		return progressCh, err
	}
	if exists {
		progressCh <- 100
		close(progressCh)
		m.progressMu.Lock()
		m.imageProgress[imageURL] = 100
		m.progressMu.Unlock()
		return progressCh, nil
	}

	go func() {
		defer close(progressCh)

		// Docker's CLI prints per-layer progress. We approximate overall progress by counting
		// layers that reached 100 %.
		cmd := m.commandContext(ctx, "pull", imageURL)

		stdout, _ := cmd.StdoutPipe()
		stderr, _ := cmd.StderrPipe()
		if err := cmd.Start(); err != nil {
			dockerLog.WithError(err).Error("Failed to start docker pull command")
			return
		}

		scanner := bufio.NewScanner(io.MultiReader(stdout, stderr))

		// Heuristic: track amount of lines containing "Download complete" vs total distinct
		// layers we have seen so far.
		layerSet := make(map[string]struct{})
		completed := make(map[string]struct{})

		layerIDRe := regexp.MustCompile(`([a-f0-9]{12})`) // first column is layer id (12 chars)

		sendProgress := func() {
			total := len(layerSet)
			if total == 0 {
				return
			}
			comp := len(completed)
			perc := int(float64(comp) / float64(total) * 100)
			m.progressMu.Lock()
			m.imageProgress[imageURL] = perc
			m.progressMu.Unlock()
			select {
			case progressCh <- perc:
			default:
			}
		}

		for scanner.Scan() {
			line := scanner.Text()
			idMatch := layerIDRe.FindString(line)
			if idMatch != "" {
				layerSet[idMatch] = struct{}{}
				if strings.Contains(line, "Download complete") || strings.Contains(line, "Pull complete") {
					completed[idMatch] = struct{}{}
				}
			}
			sendProgress()
		}

		_ = cmd.Wait()

		// ensure 100 at the end
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

// imageExists checks if an image is available locally using "docker image inspect".
func (m *DockerManager) imageExists(ctx context.Context, imageURL string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, m.defaultTimeout)
	defer cancel()

	args := []string{"image", "inspect", imageURL, "--format", "{{.Id}}"}
	err := m.commandContext(ctx, args...).Run()
	return err == nil, nil
}

func (m *DockerManager) RunContainer(ctx context.Context, config ContainerConfig) (string, error) {
	if config.PullIfNeeded {
		exists, err := m.imageExists(ctx, config.ImageURL)
		if err != nil {
			return "", errors.Wrap(err, "failed to check if image exists")
		}
		if !exists {
			dockerLog.WithField("image", config.ImageURL).Info("Image not found, pulling")
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

	dockerLog.WithField("args", args).Debug("Running container")
	output, err := m.commandContext(ctx, args...).CombinedOutput()
	if err != nil {
		dockerLog.WithError(err).WithField("output", string(output)).Error("Failed to run container")
		return "", errors.Wrapf(err, "failed to run container: %s", string(output))
	}

	containerID := strings.TrimSpace(string(output))
	return containerID, nil
}

func (m *DockerManager) StartContainer(ctx context.Context, containerID string) error {
	ctx, cancel := context.WithTimeout(ctx, m.defaultTimeout)
	defer cancel()

	args := []string{"container", "start", containerID}
	output, err := m.commandContext(ctx, args...).CombinedOutput()
	if err != nil {
		dockerLog.WithError(err).WithField("output", string(output)).Debug("Failed to start container")
		return errors.Wrap(err, "failed to start container")
	}
	return nil
}

func (m *DockerManager) StopContainer(ctx context.Context, containerID string) error {
	ctx, cancel := context.WithTimeout(ctx, m.defaultTimeout)
	defer cancel()

	args := []string{"container", "stop", containerID}
	output, err := m.commandContext(ctx, args...).CombinedOutput()
	if err != nil {
		dockerLog.WithError(err).WithField("output", string(output)).Debug("Failed to stop container")
		return errors.Wrap(err, "failed to stop container")
	}
	return nil
}

func (m *DockerManager) RemoveContainer(ctx context.Context, containerID string) error {
	ctx, cancel := context.WithTimeout(ctx, m.defaultTimeout)
	defer cancel()

	args := []string{"container", "rm", "-f", containerID}
	output, err := m.commandContext(ctx, args...).CombinedOutput()
	if err != nil {
		dockerLog.WithError(err).WithField("output", string(output)).Debug("Failed to remove container")
		return errors.Wrap(err, "failed to remove container")
	}
	return nil
}

func (m *DockerManager) CleanupContainer(ctx context.Context, containerName string) error {
	ctx, cancel := context.WithTimeout(ctx, m.defaultTimeout)
	defer cancel()

	args := []string{"container", "ls", "-a", "--filter", fmt.Sprintf("name=%s", containerName), "--format", "{{.ID}}"}
	output, err := m.commandContext(ctx, args...).Output()
	if err != nil {
		dockerLog.WithError(err).Debug("Failed to list containers for cleanup")
		return errors.Wrap(err, "failed to list containers")
	}

	containerIDs := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(containerIDs) == 0 || (len(containerIDs) == 1 && containerIDs[0] == "") {
		return nil
	}

	for _, id := range containerIDs {
		if id == "" {
			continue
		}
		rmArgs := []string{"container", "rm", "-f", id}
		rmOutput, err := m.commandContext(ctx, rmArgs...).CombinedOutput()
		if err != nil {
			dockerLog.WithError(err).WithField("output", string(rmOutput)).Warn("Failed to remove container during cleanup")
		}
	}
	return nil
}

func (m *DockerManager) ExecCommand(ctx context.Context, cmd string, args []string) (string, error) {
	execCmd := exec.CommandContext(ctx, cmd, args...)
	output, err := execCmd.CombinedOutput()
	if err != nil {
		return "", errors.Wrapf(err, "failed to execute command: %s", string(output))
	}
	return string(output), nil
}

func (m *DockerManager) GetImageProgress(ctx context.Context, imageURL string) (int, error) {
	// Return cached value if present.
	m.progressMu.RLock()
	if p, ok := m.imageProgress[imageURL]; ok {
		m.progressMu.RUnlock()
		return p, nil
	}
	m.progressMu.RUnlock()

	// Fallback to existence heuristic.
	exists, err := m.imageExists(ctx, imageURL)
	if err != nil {
		return 0, err
	}
	if exists {
		return 100, nil
	}
	return 0, nil
}

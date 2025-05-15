package container

import (
	"context"
	"time"

	"github.com/docker/docker/client"
	"github.com/pkg/errors"
)

type ContainerManager interface {
	PullImage(ctx context.Context, imageURL string) error
	RunContainer(ctx context.Context, config ContainerConfig) (string, error)
	StartContainer(ctx context.Context, containerID string) error
	StopContainer(ctx context.Context, containerID string) error
	RemoveContainer(ctx context.Context, containerID string) error
	CheckContainerExists(ctx context.Context, name string) (bool, string, error)
	CleanupContainer(ctx context.Context, name string) error
}

type ContainerConfig struct {
	ImageURL     string
	Name         string
	Ports        map[string]string
	Environment  map[string]string
	PullIfNeeded bool
}

type ReadinessCheck func(ctx context.Context) error

func NewManager(runtime string) (ContainerManager, error) {
	switch runtime {
	case "docker":
		host := detectDockerHost()
		return dockerManager(client.WithHost(host))
	case "podman":
		endpoint := detectPodmanHost()
		if endpoint == "" {
			return nil, errors.New(`cannot locate Podman socket - run "podman machine start" and/or set PODMAN_HOST`)
		}
		return dockerManager(client.WithHost(endpoint))

	default:
		return nil, errors.New("unsupported container runtime")
	}
}

func StartContainerWithReadiness(ctx context.Context, mgr ContainerManager, config ContainerConfig, check ReadinessCheck, timeout time.Duration) (string, error) {
	exists, containerID, err := mgr.CheckContainerExists(ctx, config.Name)
	if err != nil {
		return "", errors.Wrap(err, "failed to check container existence")
	}

	if exists {
		err = mgr.StartContainer(ctx, containerID)
		if err == nil && (check == nil || WaitForReady(ctx, check, timeout) == nil) {
			return containerID, nil
		}
		_ = mgr.RemoveContainer(ctx, containerID) // Remove if start or readiness fails
	}

	if config.PullIfNeeded {
		if err := mgr.PullImage(ctx, config.ImageURL); err != nil {
			return "", errors.Wrap(err, "failed to pull image")
		}
	}

	containerID, err = mgr.RunContainer(ctx, config)
	if err != nil {
		return "", errors.Wrap(err, "failed to run container")
	}

	if check != nil {
		if err := WaitForReady(ctx, check, timeout); err != nil {
			_ = mgr.StopContainer(ctx, containerID)
			_ = mgr.RemoveContainer(ctx, containerID)
			return "", errors.Wrap(err, "container failed readiness check")
		}
	}

	return containerID, nil
}

func WaitForReady(ctx context.Context, check ReadinessCheck, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "readiness check timed out")
		case <-ticker.C:
			if err := check(ctx); err == nil {
				return nil
			}
		}
	}
}

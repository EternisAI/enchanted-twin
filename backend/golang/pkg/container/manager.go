package container

import (
	"context"
	"time"
)

// ContainerManager provides an abstraction over a container runtime (e.g. Podman, Docker).
//
// Each implementation should shell out to the underlying CLI to perform the
// requested operations. The method set intentionally matches the existing
// Podman functionality so that the rest of the codebase can stay unchanged.
//
// The Executable and DefaultTimeout helpers are exposed so that callers who
// need to run arbitrary commands (e.g. PostgresManager diagnostic helpers)
// can do so without relying on implementation details.
// NOTE: "machine"-related methods are effectively no-ops for Docker because it
// does not use the Podman machine concept. They simply return (true, nil).
//
// Any additional container runtime can be supported by implementing this
// interface and updating the factory in NewManager() accordingly.
//
// If the CONTAINER_RUNTIME environment variable is not set, Podman is used by
// default because it is the historical choice for this project.
// Valid values (case-insensitive): "podman", "docker".
//
// In unit tests you can swap the returned implementation by setting the env
// variable before calling NewManager().
//
// IMPORTANT: All methods should be concurrency-safe.

type ContainerManager interface {
	IsInstalled(ctx context.Context) (bool, error)
	IsMachineInstalled(ctx context.Context) (bool, error)
	IsMachineRunning(ctx context.Context) (bool, error)
	IsContainerRunning(ctx context.Context, containerID string) (bool, error)
	CheckContainerExists(ctx context.Context, containerName string) (bool, string, error)
	PullImage(ctx context.Context, imageURL string) error
	RunContainer(ctx context.Context, containerConfig ContainerConfig) (string, error)
	StartContainer(ctx context.Context, containerID string) error
	RemoveContainer(ctx context.Context, containerID string) error
	StopContainer(ctx context.Context, containerID string) error
	CleanupContainer(ctx context.Context, containerName string) error
	ExecCommand(ctx context.Context, cmd string, args []string) (string, error)

	// Helper accessors
	Executable() string
	DefaultTimeout() time.Duration

	GetImageProgress(ctx context.Context, imageURL string) (int, error)
}

// NewManager returns a ContainerManager backed by Podman or Docker depending on
// the CONTAINER_RUNTIME environment variable. Any value other than "docker"
// results in the Podman implementation being used.
func NewManager(containerRuntime string) ContainerManager {
	switch containerRuntime {
	case "docker":
		return newDockerManager()
	default:
		return newPodmanManager()
	}
}

type ImageContainer struct {
	ImageURL    string
	ContainerID string
	DefaultPort string
}

var KokoroContainer = ImageContainer{
	ImageURL:    "ghcr.io/remsky/kokoro-fastapi-cpu:latest",
	ContainerID: "kokoro-fastapi",
	DefaultPort: "8880",
}

var PostgresContainer = ImageContainer{
	ImageURL:    "pgvector/pgvector:pg17",
	ContainerID: "enchanted-twin-postgres-pgvector",
	DefaultPort: "5432",
}

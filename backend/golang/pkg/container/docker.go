package container

import (
	"context"
	"encoding/json"
	"io"
	"strings"

	"github.com/docker/docker/api/types/container"
	imageapi "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/pkg/errors"
)

type DockerManager struct {
	cli *client.Client
}

func (m *DockerManager) PullImage(ctx context.Context, imageURL string, progress ProgressCallback) error {
	rc, err := m.cli.ImagePull(ctx, imageURL, imageapi.PullOptions{})
	if err != nil {
		return err
	}
	defer rc.Close()

	dec := json.NewDecoder(rc)
	type pullEvent struct {
		Status   string `json:"status"`
		ID       string `json:"id"`
		Error    string `json:"error"`
		Progress struct {
			Current int64 `json:"current"`
			Total   int64 `json:"total"`
		} `json:"progressDetail"`
	}

	// Track per-layer progress so we can aggregate.
	layers := make(map[string]struct{ current, total int64 })

	for {
		var evt pullEvent
		if err := dec.Decode(&evt); err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		if evt.Error != "" {
			return errors.New(evt.Error)
		}

		// Update layer progress if we have valid data.
		if evt.Progress.Total > 0 {
			layers[evt.ID] = struct {
				current, total int64
			}{evt.Progress.Current, evt.Progress.Total}
		}

		// Calculate aggregate totals and emit callback.
		if progress != nil {
			var cur, tot int64
			for _, p := range layers {
				cur += p.current
				tot += p.total
			}
			if tot > 0 {
				progress(float64(cur)/float64(tot), evt.Status)
			}
		}
	}

	// Ensure caller sees 100 % even if Docker sent no final progress.
	if progress != nil {
		progress(1, "completed")
	}
	return nil
}

func (m *DockerManager) RunContainer(ctx context.Context, config ContainerConfig) (string, error) {
	var portBindings map[nat.Port][]nat.PortBinding
	if len(config.Ports) > 0 {
		portBindings = make(map[nat.Port][]nat.PortBinding)
		for host, cont := range config.Ports {
			portBindings[nat.Port(cont+"/tcp")] = []nat.PortBinding{{HostPort: host}}
		}
	}

	resp, err := m.cli.ContainerCreate(ctx, &container.Config{
		Image: config.ImageURL,
		Env:   formatEnv(config.Environment),
	}, &container.HostConfig{
		PortBindings: portBindings,
	}, nil, nil, config.Name)
	if err != nil {
		return "", err
	}

	err = m.cli.ContainerStart(ctx, resp.ID, container.StartOptions{})
	return resp.ID, err
}

func (m *DockerManager) StartContainer(ctx context.Context, containerID string) error {
	return m.cli.ContainerStart(ctx, containerID, container.StartOptions{})
}

func (m *DockerManager) StopContainer(ctx context.Context, containerID string) error {
	return m.cli.ContainerStop(ctx, containerID, container.StopOptions{})
}

func (m *DockerManager) RemoveContainer(ctx context.Context, containerID string) error {
	return m.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
}

func (m *DockerManager) CheckContainerExists(ctx context.Context, name string) (bool, string, error) {
	containers, err := m.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return false, "", err
	}
	for _, c := range containers {
		for _, n := range c.Names {
			if strings.TrimPrefix(n, "/") == name {
				return true, c.ID, nil
			}
		}
	}
	return false, "", nil
}

func (m *DockerManager) CleanupContainer(ctx context.Context, name string) error {
	containers, err := m.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return err
	}
	for _, c := range containers {
		for _, n := range c.Names {
			if strings.TrimPrefix(n, "/") == name {
				_ = m.cli.ContainerStop(ctx, c.ID, container.StopOptions{})
				_ = m.cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true})
			}
		}
	}
	return nil
}

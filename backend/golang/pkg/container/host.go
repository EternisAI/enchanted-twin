package container

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/docker/docker/client"
	"github.com/pkg/errors"
)

func dockerManager(opts ...client.Opt) (ContainerManager, error) {
	dockerOpts := []client.Opt{
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	}
	dockerOpts = append(dockerOpts, opts...)
	cli, err := client.NewClientWithOpts(dockerOpts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create container client")
	}
	return &DockerManager{cli: cli}, nil
}

func detectDockerHost() string {
	cmd := exec.Command("docker", "context", "inspect",
		"--format", "{{ .Endpoints.docker.Host }}")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err == nil {
		if h := strings.TrimSpace(out.String()); h != "" && h != "null" {
			return h
		}
	}
	return "unix:///var/run/docker.sock"
}

func detectPodmanHost() string {
	if h := os.Getenv("PODMAN_HOST"); h != "" {
		return h
	}
	if h := os.Getenv("DOCKER_HOST"); strings.HasPrefix(h, "ssh://") || strings.HasPrefix(h, "unix://") {
		return h
	}
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" && runtime.GOOS != "darwin" {
		sock := filepath.Join(dir, "podman", "podman.sock")
		if _, err := os.Stat(sock); err == nil {
			return "unix://" + sock
		}
	}
	if endpoint, _ := probePodmanSocket(); endpoint != "" {
		return endpoint
	}
	return ""
}

func probePodmanSocket() (string, error) {
	out, err := exec.Command("podman", "machine", "inspect").Output()
	if err != nil {
		return "", err
	}

	fmt.Println(string(out))

	var machines []struct {
		State          string `json:"State"`
		ConnectionInfo struct {
			PodmanSocket struct {
				Path string `json:"Path"`
			} `json:"PodmanSocket"`
		} `json:"ConnectionInfo"`
	}
	if err := json.Unmarshal(out, &machines); err != nil {
		return "", err
	}

	for _, m := range machines {
		if m.State == "running" && m.ConnectionInfo.PodmanSocket.Path != "" {
			return "unix://" + m.ConnectionInfo.PodmanSocket.Path, nil
		}
	}
	return "", fmt.Errorf("no running machines")
}

func formatEnv(env map[string]string) []string {
	var result []string
	for k, v := range env {
		result = append(result, k+"="+v)
	}
	return result
}

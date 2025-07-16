package pylocal

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type Client struct {
	logger    *slog.Logger
	serverCmd *exec.Cmd
	serverURL string
}

func NewClient(logger *slog.Logger, projectDir string) (*Client, error) {
	if projectDir == "" {
		return nil, fmt.Errorf("projectDir cannot be empty")
	}
	if _, err := os.Stat(projectDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("project directory does not exist: %s", projectDir)
	}

	client := &Client{
		logger: logger,
	}

	logger.Info("Starting Python server", "projectDir", projectDir)

	// Start the Python server with unbuffered output using uv
	serverCmd := exec.Command("uv", "run", "--", "python3", "-u", "main.py")
	serverCmd.Dir = projectDir

	// Create pipes to capture output
	stdout, err := serverCmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	serverCmd.Stderr = os.Stderr
	serverCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := serverCmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start Python server: %w", err)
	}

	client.serverCmd = serverCmd

	// Parse the port from server output
	port, err := client.parsePortFromOutput(stdout)
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("failed to get server port: %w", err)
	}

	client.serverURL = fmt.Sprintf("http://localhost:%d", port)

	logger.Info("Python server started", "url", client.serverURL)

	return client, nil
}

func (c *Client) parsePortFromOutput(stdout io.ReadCloser) (int, error) {
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if portStr, found := strings.CutPrefix(line, "SERVER_PORT:"); found {
			port, err := strconv.Atoi(portStr)
			if err != nil {
				return 0, fmt.Errorf("failed to parse port: %w", err)
			}
			return port, nil
		}
	}
	return 0, fmt.Errorf("port not found in server output")
}

func (c *Client) Infer(input string) (string, error) {
	start := time.Now()

	requestBody := map[string]string{"input": input}
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := httpClient.Post(c.serverURL+"/infer", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var response map[string]string
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if errorMsg, exists := response["error"]; exists {
		return "", fmt.Errorf("server error: %s", errorMsg)
	}

	elapsed := time.Since(start)
	c.logger.Info("inference completed", "duration", elapsed, "input", input)

	return response["output"], nil
}

func (c *Client) Close() error {
	if c.serverCmd != nil && c.serverCmd.Process != nil {
		// Kill the entire process group to ensure all subprocesses are terminated
		pgid, err := syscall.Getpgid(c.serverCmd.Process.Pid)
		if err == nil {
			_ = syscall.Kill(-pgid, syscall.SIGTERM)
		}
		_ = c.serverCmd.Process.Kill()
		_ = c.serverCmd.Wait()
	}
	return nil
}

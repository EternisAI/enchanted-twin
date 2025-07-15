package pyhttp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"
)

const (
	serverURL = "http://localhost:8080"
)

type Client struct {
	serverCmd *exec.Cmd
}

func NewClient() (*Client, error) {
	client := &Client{}

	// Start the Python server with unbuffered output
	serverCmd := exec.Command("python3", "-u", "sample/sample.py")
	serverCmd.Stdout = os.Stdout
	serverCmd.Stderr = os.Stderr
	if err := serverCmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start Python server: %w", err)
	}

	client.serverCmd = serverCmd

	// Wait for server to be ready
	if err := client.waitForServerReady(10 * time.Second); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("server failed to start: %w", err)
	}

	return client, nil
}

func (c *Client) waitForServerReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(serverURL + "/infer")
		if err == nil {
			_ = resp.Body.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("server not ready within %v", timeout)
}

func (c *Client) Infer(input string) (string, error) {
	requestBody := map[string]string{"input": input}
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := http.Post(serverURL+"/infer", "application/json", bytes.NewBuffer(jsonData))
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

	return response["output"], nil
}

func (c *Client) Close() error {
	if c.serverCmd != nil && c.serverCmd.Process != nil {
		_ = c.serverCmd.Process.Kill()
		_ = c.serverCmd.Wait()
	}
	return nil
}

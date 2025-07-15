package pysocket

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"
)

const (
	serverHost = "localhost"
	serverPort = "8080"
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
		client.Close()
		return nil, fmt.Errorf("server failed to start: %w", err)
	}

	return client, nil
}

func (c *Client) waitForServerReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.Dial("tcp", net.JoinHostPort(serverHost, serverPort))
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("server not ready within %v", timeout)
}

func (c *Client) Infer(input string) (string, error) {
	conn, err := net.Dial("tcp", net.JoinHostPort(serverHost, serverPort))
	if err != nil {
		return "", fmt.Errorf("failed to connect to server: %w", err)
	}
	defer conn.Close()

	// Send the infer command with input data
	message := fmt.Sprintf("infer:%s\n", input)
	_, err = conn.Write([]byte(message))
	if err != nil {
		return "", fmt.Errorf("failed to send command: %w", err)
	}

	// Read the response
	scanner := bufio.NewScanner(conn)
	if scanner.Scan() {
		response := scanner.Text()
		return response, nil
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return "", fmt.Errorf("no response received")
}

func (c *Client) Close() error {
	if c.serverCmd != nil && c.serverCmd.Process != nil {
		c.serverCmd.Process.Kill()
		c.serverCmd.Wait()
	}
	return nil
}

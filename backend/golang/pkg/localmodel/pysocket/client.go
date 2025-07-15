package pysocket

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
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
	// first check if file exists
	if _, err := os.Stat("sample/sample.py"); os.IsNotExist(err) {
		return nil, fmt.Errorf("sample/sample.py does not exist")
	}

	client := &Client{}

	// Check if server is already running
	if !isServerRunning() {
		// Start the Python server
		serverCmd := exec.Command("python3", "sample/sample.py")
		serverCmd.Dir = "."
		serverCmd.Stdout = os.Stdout
		serverCmd.Stderr = os.Stderr

		if err := serverCmd.Start(); err != nil {
			return nil, fmt.Errorf("failed to start Python server: %w", err)
		}

		client.serverCmd = serverCmd

		// Wait for server to start up (give it time to load the model)
		if err := waitForServer(30 * time.Second); err != nil {
			if client.serverCmd != nil {
				client.serverCmd.Process.Kill()
			}
			return nil, fmt.Errorf("server failed to start: %w", err)
		}
	}

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		client.Close()
		os.Exit(0)
	}()

	return client, nil
}

func (c *Client) Infer(input string) (string, error) {
	conn, err := net.Dial("tcp", net.JoinHostPort(serverHost, serverPort))
	if err != nil {
		return "", fmt.Errorf("failed to connect to server: %w", err)
	}
	defer conn.Close()

	// Send the infer command
	_, err = conn.Write([]byte("infer\n"))
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
		if err := c.serverCmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill server process: %w", err)
		}
		c.serverCmd.Wait()
	}
	return nil
}

func isServerRunning() bool {
	conn, err := net.Dial("tcp", net.JoinHostPort(serverHost, serverPort))
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func waitForServer(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if isServerRunning() {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("server did not start within %v", timeout)
}

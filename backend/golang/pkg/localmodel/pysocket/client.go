package pysocket

import (
	"bufio"
	"fmt"
	"net"
	"os/exec"
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

	// Start the Python server
	serverCmd := exec.Command("python3", "sample/sample.py")
	if err := serverCmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start Python server: %w", err)
	}

	client.serverCmd = serverCmd

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
		c.serverCmd.Process.Kill()
		c.serverCmd.Wait()
	}
	return nil
}

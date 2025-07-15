package pysocket

import (
	"testing"
	"time"
)

func TestPySocketInfer(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Wait for Python server to start and load model
	time.Sleep(6 * time.Second)

	response, err := client.Infer("test input")
	if err != nil {
		t.Fatalf("Infer failed: %v", err)
	}

	if response == "" {
		t.Fatal("Response should not be empty")
	}
}

package pysocket

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPySocketInfer(t *testing.T) {
	client, err := NewClient()
	assert.NoError(t, err)
	defer client.Close()

	// Wait for Python server to start and load model
	time.Sleep(6 * time.Second)

	response, err := client.Infer("test input")
	assert.NoError(t, err)

	t.Logf("Response: '%s'", response)
}

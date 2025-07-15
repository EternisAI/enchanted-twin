package pyhttp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPySocketInfer(t *testing.T) {
	client, err := NewClient()
	assert.NoError(t, err)
	defer func() { _ = client.Close() }()

	response, err := client.Infer("test input")
	assert.NoError(t, err)

	t.Logf("Response: '%s'", response)
}

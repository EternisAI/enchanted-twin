package pyhttp

import (
	"context"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/stretchr/testify/assert"
)

func TestPySocketGenerate(t *testing.T) {
	t.Skip("Skipping test")
	client, err := NewClient(log.Default())
	assert.NoError(t, err)
	defer func() { _ = client.Close() }()

	response, err := client.Anonymize(context.Background(), "Please anonymize this text: My name is Alice and I work at Google.")
	assert.NoError(t, err)

	t.Logf("Response: %+v", response)
}

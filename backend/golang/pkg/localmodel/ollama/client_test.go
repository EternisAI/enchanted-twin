package ollama

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewOllamaClient_Anonymize(t *testing.T) {
	t.Skip()
	client := NewOllamaClient("http://localhost:11434/v1")

	prompt := "My name is Alice and I work at Google."
	result, err := client.Anonymize(context.Background(), "openai/gpt-4.1-mini", prompt)
	assert.NoError(t, err)

	t.Logf("Anonymization result: %+v", result)
}

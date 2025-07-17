package ollama

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewOllamaClient_Anonymize(t *testing.T) {
	t.Skip()
	client := NewOllamaClient("http://localhost:11434/v1", "qwen3-0.6b-q4_k_m")

	prompt := "My name is Alice and I work at Google."
	result, err := client.Anonymize(context.Background(), prompt)
	assert.NoError(t, err)

	t.Logf("Anonymization result: %+v", result)
}

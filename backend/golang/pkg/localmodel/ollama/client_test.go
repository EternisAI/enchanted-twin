package ollama

import (
	"context"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/stretchr/testify/assert"
)

func TestNewOllamaClient_Anonymize(t *testing.T) {
	// t.Skip()
	client := NewOllamaClient("http://localhost:11435/v1", "qwen3-4b_q4_k_m", log.Default())

	prompt := "I'm cooking chicken with Elisabeth."
	result, err := client.Anonymize(context.Background(), prompt)
	assert.NoError(t, err)

	t.Logf("Anonymization result: %+v", result)
}

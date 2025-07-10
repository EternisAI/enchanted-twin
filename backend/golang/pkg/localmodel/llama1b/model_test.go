package llama1b

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/shared/constant"
	"github.com/stretchr/testify/assert"
)

func TestLlamaModel_InteractiveMode(t *testing.T) {
	binaryPath := os.Getenv("LLAMA_BINARY_PATH")
	if binaryPath == "" {
		t.Skip("LLAMA_BINARY_PATH not set")
	}

	modelDir := os.Getenv("LLAMA_MODEL_DIR")
	if modelDir == "" {
		t.Skip("LLAMA_MODEL_DIR not set")
	}

	tokenizerName := os.Getenv("LLAMA_TOKENIZER_NAME")
	if tokenizerName == "" {
		tokenizerName = "meta-llama/Llama-3.2-1B"
	}

	model, err := NewLlamaModel(context.Background(), binaryPath, modelDir, tokenizerName, true)
	assert.NoError(t, err)
	defer func() { _ = model.Close() }()

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage("You are a helpful assistant."),
		openai.UserMessage("Say hello"),
	}

	start := time.Now()
	response, err := model.Completions(context.Background(), messages, nil, "llama-1b")
	elapsed := time.Since(start)

	assert.NoError(t, err)
	assert.Equal(t, constant.Assistant("assistant"), response.Role)
	assert.NotEmpty(t, response.Content)
	t.Logf("Response: %s", response.Content)
	t.Logf("Completions Inferencing time: %v", elapsed)
}

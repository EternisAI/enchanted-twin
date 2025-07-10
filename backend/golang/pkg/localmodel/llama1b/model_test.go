package llama1b

import (
	"context"
	"os"
	"testing"

	"github.com/openai/openai-go"
	"github.com/stretchr/testify/assert"
)

func TestNewLlamaModel(t *testing.T) {
	binaryPath := "/usr/bin/llama"
	modelDir := "/models/llama"

	model := NewLlamaModel(binaryPath, modelDir)

	assert.NotNil(t, model)
	assert.Equal(t, binaryPath, model.binaryPath)
	assert.Equal(t, modelDir, model.modelDir)
}

func TestLlamaModel_extractPrompts(t *testing.T) {
	t.Skip()
	model := NewLlamaModel("/bin/llama", "/models")

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage("You are a helpful assistant."),
		openai.UserMessage("Hello, how are you?"),
	}

	systemPrompt, userPrompt, err := model.extractPrompts(messages)

	assert.NoError(t, err)
	assert.Equal(t, "You are a helpful assistant.", systemPrompt)
	assert.Equal(t, "Hello, how are you?", userPrompt)
}

func TestLlamaModel_executeCLI(t *testing.T) {
	t.Skip()

	model := NewLlamaModel("/usr/bin/llama", "/models")

	output, err := model.executeCLI(context.Background(), "System prompt", "User prompt")

	assert.NoError(t, err)
	assert.NotEmpty(t, output)
}

func TestLlamaModel_Completions(t *testing.T) {
	t.Skip("Skipping integration test - requires llama binary")

	binaryPath := os.Getenv("LLAMA_BINARY_PATH")
	if binaryPath == "" {
		t.Skip("LLAMA_BINARY_PATH not set")
	}

	modelDir := os.Getenv("LLAMA_MODEL_DIR")
	if modelDir == "" {
		t.Skip("LLAMA_MODEL_DIR not set")
	}

	model := NewLlamaModel(binaryPath, modelDir)

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage("You are a helpful assistant."),
		openai.UserMessage("Say hello"),
	}

	response, err := model.Completions(context.Background(), messages, nil, "llama-1b")

	assert.NoError(t, err)
	assert.Equal(t, "assistant", response.Role)
	assert.NotEmpty(t, response.Content)
}

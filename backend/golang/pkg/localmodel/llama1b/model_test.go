package llama1b

import (
	"context"
	"os"
	"testing"
	"time"

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
	assert.Equal(t, "meta-llama/Llama-3.2-1B", model.tokenizerName)
	assert.False(t, model.interactiveMode)
	assert.Equal(t, 1000, model.maxTokens)
}

func TestNewInteractiveLlamaModel(t *testing.T) {
	binaryPath := "/usr/bin/llama"
	modelDir := "/models/llama"
	tokenizerName := "custom-tokenizer"

	model := NewInteractiveLlamaModel(binaryPath, modelDir, tokenizerName)

	assert.NotNil(t, model)
	assert.Equal(t, binaryPath, model.binaryPath)
	assert.Equal(t, modelDir, model.modelDir)
	assert.Equal(t, tokenizerName, model.tokenizerName)
	assert.True(t, model.interactiveMode)
	assert.Equal(t, 1000, model.maxTokens)
}

func TestLlamaModel_SetConfiguration(t *testing.T) {
	model := NewLlamaModel("/usr/bin/llama", "/models")

	model.SetMaxTokens(2000)
	model.SetTemperature(0.9)
	model.SetSessionTimeout(10 * time.Minute)

	assert.Equal(t, 2000, model.maxTokens)
	assert.Equal(t, 0.9, model.temperature)
	assert.Equal(t, 10*time.Minute, model.sessionTimeout)
}

func TestLlamaModel_extractPrompts(t *testing.T) {
	model := NewLlamaModel("/bin/llama", "/models")

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage("You are a helpful assistant."),
		openai.UserMessage("Hello, how are you?"),
		openai.AssistantMessage("I'm doing well, thank you!"),
		openai.UserMessage("What's the weather like?"),
	}

	systemPrompt, userPrompt, err := model.extractPrompts(messages)

	assert.NoError(t, err)
	assert.Equal(t, "You are a helpful assistant.", systemPrompt)
	expectedUserPrompt := "User: Hello, how are you?\nAssistant: I'm doing well, thank you!\nUser: What's the weather like?"
	assert.Equal(t, expectedUserPrompt, userPrompt)
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

func TestLlamaModel_InteractiveMode(t *testing.T) {
	t.Skip("Skipping integration test - requires llama binary")

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

	model := NewInteractiveLlamaModel(binaryPath, modelDir, tokenizerName)
	defer func() { _ = model.Close() }()

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage("You are a helpful assistant."),
		openai.UserMessage("Say hello"),
	}

	response, err := model.Completions(context.Background(), messages, nil, "llama-1b")

	assert.NoError(t, err)
	assert.Equal(t, "assistant", response.Role)
	assert.NotEmpty(t, response.Content)
}

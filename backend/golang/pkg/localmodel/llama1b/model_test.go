package llama1b

import (
	"context"
	"encoding/json"
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

	systemPrompt := "Find names in the input text. For each name found, create a JSON mapping where the original name is the key and a completely different, unrelated name is the value. The replacement name must be different from the original. Return only JSON."

	model, err := NewLlamaModel(context.Background(), binaryPath, modelDir, tokenizerName, true, systemPrompt)
	assert.NoError(t, err)
	defer func() { _ = model.Close() }()

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("I am John"),
	}

	// First Inferencing
	start := time.Now()
	response, err := model.Completions(context.Background(), messages, nil, "llama-1b")
	elapsed := time.Since(start)

	assert.NoError(t, err)
	assert.Equal(t, constant.Assistant("assistant"), response.Role)
	assert.NotEmpty(t, response.Content)

	var anonymizeMap map[string]string
	if err := json.Unmarshal([]byte(response.Content), &anonymizeMap); err != nil {
		t.Log("Fail to parse response, the response is not in Json map of string to string format")
	}

	t.Logf("Original Response: %s", response.Content)
	t.Logf("Anonymize Map: %v", anonymizeMap)
	t.Logf("Completions Inferencing time: %v", elapsed)

	// Second Inferencing
	messages = []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("I am Emily"),
	}
	start = time.Now()
	response, err = model.Completions(context.Background(), messages, nil, "llama-1b")
	elapsed = time.Since(start)
	t.Logf("Response: %s", response.Content)
	t.Logf("Completions Inferencing time: %v", elapsed)
}

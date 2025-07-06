package coreml

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/openai/openai-go"
)

func TestCoreMLInference(t *testing.T) {
	binaryPath := "./coreml-cli-v2"
	modelPath := "./test_coreml"
	input := "Testing"
	service := NewService(binaryPath, modelPath, false)

	start := time.Now()
	_, err := service.Infer(input)
	elapsed := time.Since(start)

	if err != nil {
		t.Logf("Error: %v", err)
	}
	// t.Logf("Result: %s", result)
	t.Logf("Inference time: %v", elapsed)
}

func BenchmarkCoreMLInference(b *testing.B) {
	binaryPath := "./coreml-cli"
	modelPath := "./test_coreml"
	input := "./cat.jpg"
	service := NewService(binaryPath, modelPath, false)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.Infer(input)
		if err != nil {
			b.Fatalf("Error: %v", err)
		}
	}
}

func TestCoreMLInteractiveMode(t *testing.T) {
	binaryPath := "./coreml-cli-v2"
	modelPath := "./test_coreml"
	input := "Testing"
	service := NewService(binaryPath, modelPath, true)
	defer func() { _ = service.Close() }()

	start := time.Now()
	_, err := service.Infer(input)
	elapsed := time.Since(start)

	if err != nil {
		t.Logf("Error: %v", err)
	}
	// t.Logf("Result: %s", result)
	t.Logf("Interactive inference time: %v", elapsed)
}

func TestCoreMLInteractiveMultipleInferences(t *testing.T) {
	binaryPath := "./coreml-cli-v2"
	modelPath := "./test_coreml"
	service := NewService(binaryPath, modelPath, true)
	defer func() { _ = service.Close() }()

	inputs := []string{"Testing1", "Testing2", "Testing3"}

	for i, input := range inputs {
		start := time.Now()
		_, err := service.Infer(input)
		elapsed := time.Since(start)

		if err != nil {
			t.Logf("Error on inference %d: %v", i+1, err)
		}
		t.Logf("Interactive inference %d time: %v", i+1, elapsed)
	}
}

func BenchmarkCoreMLInteractiveMode(b *testing.B) {
	binaryPath := "./coreml-cli"
	modelPath := "./test_coreml"
	input := "test"
	service := NewService(binaryPath, modelPath, true)
	defer func() { _ = service.Close() }()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.Infer(input)
		if err != nil {
			b.Fatalf("Error: %v", err)
		}
	}
}

func TestCoreMLCompletionsInteractive(t *testing.T) {
	ctx := context.Background()
	mockProcess := NewMockSuccessfulProcess()
	service := NewServiceWithProcess("./mock-binary", "./mock-model", true, mockProcess)
	defer func() { _ = service.Close() }()

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Hello, how are you?"),
	}

	response, err := service.Completions(ctx, messages, nil, "test-model")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if response.Content != "Mock completion response" {
		t.Errorf("Expected 'Mock completion response', got: %s", response.Content)
	}

	if response.Role != "assistant" {
		t.Errorf("Expected assistant role, got: %v", response.Role)
	}
}

func TestCoreMLCompletionsNonInteractive(t *testing.T) {
	ctx := context.Background()
	mockProcess := NewMockSuccessfulProcess()
	service := NewServiceWithProcess("./mock-binary", "./mock-model", false, mockProcess)

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Hello, how are you?"),
	}

	response, err := service.Completions(ctx, messages, nil, "test-model")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if response.Content != "Mock completion response" {
		t.Errorf("Expected 'Mock completion response', got: %s", response.Content)
	}
}

func TestCoreMLEmbeddingsInteractive(t *testing.T) {
	ctx := context.Background()
	mockProcess := NewMockSuccessfulProcess()
	service := NewServiceWithProcess("./mock-binary", "./mock-model", true, mockProcess)
	defer func() { _ = service.Close() }()

	inputs := []string{"hello world", "test input"}
	embeddings, err := service.Embeddings(ctx, inputs, "test-model")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(embeddings) != 2 {
		t.Errorf("Expected 2 embeddings, got: %d", len(embeddings))
	}

	expectedEmbedding := []float64{0.1, 0.2, 0.3, 0.4, 0.5}
	for i, embedding := range embeddings {
		if len(embedding) != len(expectedEmbedding) {
			t.Errorf("Embedding %d: expected length %d, got %d", i, len(expectedEmbedding), len(embedding))
		}
		for j, val := range embedding {
			if val != expectedEmbedding[j] {
				t.Errorf("Embedding %d[%d]: expected %f, got %f", i, j, expectedEmbedding[j], val)
			}
		}
	}
}

func TestCoreMLEmbeddingsSingle(t *testing.T) {
	ctx := context.Background()
	mockProcess := NewMockSuccessfulProcess()
	service := NewServiceWithProcess("./mock-binary", "./mock-model", true, mockProcess)
	defer func() { _ = service.Close() }()

	embedding, err := service.Embedding(ctx, "hello world", "test-model")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	expectedEmbedding := []float64{0.1, 0.2, 0.3, 0.4, 0.5}
	if len(embedding) != len(expectedEmbedding) {
		t.Errorf("Expected length %d, got %d", len(expectedEmbedding), len(embedding))
	}

	for i, val := range embedding {
		if val != expectedEmbedding[i] {
			t.Errorf("Embedding[%d]: expected %f, got %f", i, expectedEmbedding[i], val)
		}
	}
}

func TestCoreMLCompletionsFailure(t *testing.T) {
	ctx := context.Background()
	mockProcess := NewMockFailingProcess("model failed to load")
	service := NewServiceWithProcess("./mock-binary", "./mock-model", true, mockProcess)
	defer func() { _ = service.Close() }()

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Hello, how are you?"),
	}

	_, err := service.Completions(ctx, messages, nil, "test-model")
	if err == nil {
		t.Fatal("Expected error, got none")
	}

	if err.Error() != "completion failed: model failed to load" {
		t.Errorf("Expected 'completion failed: model failed to load', got: %s", err.Error())
	}
}

func TestCoreMLEmbeddingsFailure(t *testing.T) {
	ctx := context.Background()
	mockProcess := NewMockFailingProcess("embedding model unavailable")
	service := NewServiceWithProcess("./mock-binary", "./mock-model", true, mockProcess)
	defer func() { _ = service.Close() }()

	_, err := service.Embeddings(ctx, []string{"test"}, "test-model")
	if err == nil {
		t.Fatal("Expected error, got none")
	}

	if err.Error() != "embedding failed: embedding model unavailable" {
		t.Errorf("Expected 'embedding failed: embedding model unavailable', got: %s", err.Error())
	}
}

func TestCoreMLProcessRestart(t *testing.T) {
	ctx := context.Background()
	mockProcess := NewMockUnresponsiveProcess()
	service := NewServiceWithProcess("./mock-binary", "./mock-model", true, mockProcess)
	defer func() { _ = service.Close() }()

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Hello, how are you?"),
	}

	_, err := service.Completions(ctx, messages, nil, "test-model")
	if err == nil {
		t.Fatal("Expected error due to unresponsive process, got none")
	}
}

func TestCoreMLServiceImplementsAIInterface(t *testing.T) {
	mockProcess := NewMockSuccessfulProcess()
	service := NewServiceWithProcess("./mock-binary", "./mock-model", true, mockProcess)
	defer func() { _ = service.Close() }()

	var _ interface {
		Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) (openai.ChatCompletionMessage, error)
		Embeddings(ctx context.Context, inputs []string, model string) ([][]float64, error)
		Embedding(ctx context.Context, input string, model string) ([]float64, error)
	} = service
}

func TestCoreMLServiceWithDifferentModels(t *testing.T) {
	ctx := context.Background()
	mockProcess := NewMockSuccessfulProcess()
	service := &Service{
		binaryPath:      "./mock-binary",
		completionModel: "./completion-model",
		embeddingModel:  "./embedding-model",
		interactive:     true,
		process:         mockProcess,
	}
	defer func() { _ = service.Close() }()

	if err := service.startInteractiveProcess(); err != nil {
		t.Fatalf("Failed to start interactive process: %v", err)
	}

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Test completion"),
	}
	_, err := service.Completions(ctx, messages, nil, "test-model")
	if err != nil {
		t.Fatalf("Completion failed: %v", err)
	}

	_, err = service.Embeddings(ctx, []string{"test embedding"}, "test-model")
	if err != nil {
		t.Fatalf("Embedding failed: %v", err)
	}
}

func TestCoreMLIntegrationCompletion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	binaryPath := "./coreml-cli-v2"
	modelPath := "./test_coreml"

	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Skip("CoreML binary not found, skipping integration test")
	}

	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		t.Skip("CoreML model not found, skipping integration test")
	}

	ctx := context.Background()
	service := NewService(binaryPath, modelPath, true)
	defer func() { _ = service.Close() }()

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Hello, how are you?"),
	}

	response, err := service.Completions(ctx, messages, nil, "test-model")
	if err != nil {
		t.Logf("Integration test failed (expected for mock protocol): %v", err)
	} else {
		t.Logf("Response: %s", response.Content)
	}
}

func TestCoreMLIntegrationEmbedding(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	binaryPath := "./coreml-cli-v2"
	modelPath := "./jina-v2"

	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Skip("CoreML binary not found, skipping integration test")
	}

	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		t.Skip("CoreML embedding model not found, skipping integration test")
	}

	ctx := context.Background()
	service := NewService(binaryPath, modelPath, true)
	defer func() { _ = service.Close() }()

	embeddings, err := service.Embeddings(ctx, []string{"test input"}, "jina-v2")
	if err != nil {
		t.Logf("Integration test failed (expected for mock protocol): %v", err)
	} else {
		t.Logf("Embeddings shape: %dx%d", len(embeddings), len(embeddings[0]))
	}
}

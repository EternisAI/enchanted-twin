package coreml

import (
	"testing"
	"time"
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
	defer service.Close()

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
	defer service.Close()

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
	defer service.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.Infer(input)
		if err != nil {
			b.Fatalf("Error: %v", err)
		}
	}
}

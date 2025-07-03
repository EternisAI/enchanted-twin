package coreml

import (
	"testing"
)

func TestCoreMLInference(t *testing.T) {
	binaryPath := "./coreml-cli"
	modelPath := "./test_coreml"
	input := "1"
	service := NewService(binaryPath, modelPath)

	result, err := service.Infer(input)
	if err != nil {
		t.Logf("Error: %v", err)
	}
	t.Logf("Result: %s", result)
}

func BenchmarkCoreMLInference(b *testing.B) {
	binaryPath := "./coreml-cli"
	modelPath := "./test_coreml"
	input := "1"
	service := NewService(binaryPath, modelPath)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.Infer(input)
		if err != nil {
			b.Fatalf("Error: %v", err)
		}
	}
}

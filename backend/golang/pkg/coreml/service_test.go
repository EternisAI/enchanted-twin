package coreml

import (
	"testing"
)

func TestCoreMLInference(t *testing.T) {
	binaryPath := "./coreml-cli"
	modelPath := "./test_coreml"
	service := NewService(binaryPath, modelPath)

	result, err := service.Infer("1234")
	if err != nil {
		t.Logf("Error: %v", err)
	}
	t.Logf("Result: %s", result)
}

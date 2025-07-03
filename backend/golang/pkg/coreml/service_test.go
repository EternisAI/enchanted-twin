package coreml

import (
	"testing"
)

func TestNewService(t *testing.T) {
	binaryPath := "/path/to/binary"
	modelPath := "/path/to/model"

	service := NewService(binaryPath, modelPath)

	if service.binaryPath != binaryPath {
		t.Errorf("Expected binaryPath %s, got %s", binaryPath, service.binaryPath)
	}
	if service.modelPath != modelPath {
		t.Errorf("Expected modelPath %s, got %s", modelPath, service.modelPath)
	}
}

func TestNewDefaultService(t *testing.T) {
	service := NewDefaultService()

	expectedBinaryPath := "./coreml-cli"
	expectedModelPath := "./test_coreml"

	if service.binaryPath != expectedBinaryPath {
		t.Errorf("Expected binaryPath %s, got %s", expectedBinaryPath, service.binaryPath)
	}
	if service.modelPath != expectedModelPath {
		t.Errorf("Expected modelPath %s, got %s", expectedModelPath, service.modelPath)
	}

	result, err := service.Infer(3)
	if err != nil {
		t.Logf("Error: %v", err)
	}
	t.Logf("Result: %s", result)
}

func TestInfer_BinaryNotFound(t *testing.T) {
	service := NewService("/nonexistent/binary", "/nonexistent/model")

	_, err := service.Infer(1.0)
	if err == nil {
		t.Error("Expected error when binary not found")
	}
}

func TestInfer_ModelNotFound(t *testing.T) {
	service := NewService("../../coreml-cli", "/nonexistent/model")

	_, err := service.Infer(1.0)
	if err == nil {
		t.Error("Expected error when model not found")
	}
}

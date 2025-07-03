package coreml

import (
	"fmt"
	"os"
	"os/exec"
)

type Service struct {
	binaryPath string
	modelPath  string
}

func NewService(binaryPath, modelPath string) *Service {
	return &Service{
		binaryPath: binaryPath,
		modelPath:  modelPath,
	}
}

func (s *Service) Infer(inputValue string) (string, error) {
	if _, err := os.Stat(s.binaryPath); os.IsNotExist(err) {
		return "", fmt.Errorf("coreml-cli binary not found at %s", s.binaryPath)
	}

	if _, err := os.Stat(s.modelPath); os.IsNotExist(err) {
		return "", fmt.Errorf("model not found at %s", s.modelPath)
	}

	cmd := exec.Command(s.binaryPath, "infer", s.modelPath, inputValue)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to execute coreml-cli: %w, output: %s", err, string(output))
	}

	return string(output), nil
}

package coreml

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
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

func (s *Service) Infer(inputValue float64) (string, error) {
	if _, err := os.Stat(s.binaryPath); os.IsNotExist(err) {
		return "", fmt.Errorf("coreml-cli binary not found at %s", s.binaryPath)
	}

	if _, err := os.Stat(s.modelPath); os.IsNotExist(err) {
		return "", fmt.Errorf("model not found at %s", s.modelPath)
	}

	inputStr := strconv.FormatFloat(inputValue, 'f', -1, 64)

	cmd := exec.Command(s.binaryPath, "infer", s.modelPath, inputStr)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to execute coreml-cli: %w, output: %s", err, string(output))
	}

	// Extract the numeric result from the output
	outputStr := string(output)

	// Look for the output line that contains the result
	re := regexp.MustCompile(`📤 Output: \[([^\]]+)\]`)
	matches := re.FindStringSubmatch(outputStr)

	if len(matches) > 1 {
		return matches[1], nil
	}

	// Debug: if we can't parse, let's see what we got
	if outputStr == "" {
		return "DEBUG: empty output", nil
	}

	// Fallback to returning the full output if we can't parse it
	return fmt.Sprintf("DEBUG: %s", strings.TrimSpace(outputStr)), nil
}

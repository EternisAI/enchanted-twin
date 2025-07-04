package coreml

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
)

type Service struct {
	binaryPath  string
	modelPath   string
	interactive bool
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	stdout      io.ReadCloser
	scanner     *bufio.Scanner
	mu          sync.Mutex
}

func NewService(binaryPath, modelPath string, interactive bool) *Service {
	s := &Service{
		binaryPath:  binaryPath,
		modelPath:   modelPath,
		interactive: interactive,
	}

	if interactive {
		if err := s.startInteractiveProcess(); err != nil {
			s.interactive = false
		}
	}

	return s
}

func (s *Service) Infer(inputValue string) (string, error) {
	if s.interactive {
		return s.inferInteractive(inputValue)
	}
	return s.inferNonInteractive(inputValue)
}

func (s *Service) inferInteractive(inputValue string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for retries := 0; retries < 2; retries++ {
		if s.cmd == nil || s.stdin == nil || s.scanner == nil {
			if err := s.restartInteractiveProcess(); err != nil {
				if retries == 1 {
					return "", fmt.Errorf("failed to restart interactive process: %w", err)
				}
				continue
			}
		}

		if s.cmd.ProcessState != nil && s.cmd.ProcessState.Exited() {
			if err := s.restartInteractiveProcess(); err != nil {
				if retries == 1 {
					return "", fmt.Errorf("failed to restart interactive process after exit: %w", err)
				}
				continue
			}
		}

		input := map[string]interface{}{
			"inputs": []string{inputValue},
		}
		inputJSON, err := json.Marshal(input)
		if err != nil {
			return "", fmt.Errorf("failed to marshal input JSON: %w", err)
		}

		if _, err := s.stdin.Write(append(inputJSON, '\n')); err != nil {
			if retries < 1 {
				s.restartInteractiveProcess()
				continue
			}
			return "", fmt.Errorf("failed to write to stdin: %w", err)
		}

		if !s.scanner.Scan() {
			if err := s.scanner.Err(); err != nil {
				if retries < 1 {
					s.restartInteractiveProcess()
					continue
				}
				return "", fmt.Errorf("failed to read from stdout: %w", err)
			}
			return "", fmt.Errorf("no response from interactive process")
		}

		response := strings.TrimSpace(s.scanner.Text())
		return response, nil
	}

	return "", fmt.Errorf("failed to get response after retries")
}

func (s *Service) inferNonInteractive(inputValue string) (string, error) {
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

func (s *Service) startInteractiveProcess() error {
	if _, err := os.Stat(s.binaryPath); os.IsNotExist(err) {
		return fmt.Errorf("coreml-cli binary not found at %s", s.binaryPath)
	}

	if _, err := os.Stat(s.modelPath); os.IsNotExist(err) {
		return fmt.Errorf("model not found at %s", s.modelPath)
	}

	s.cmd = exec.Command(s.binaryPath, "interactive", s.modelPath)

	stdin, err := s.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	s.stdin = stdin

	stdout, err := s.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	s.stdout = stdout
	s.scanner = bufio.NewScanner(stdout)

	if err := s.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start interactive process: %w", err)
	}

	return nil
}

func (s *Service) stopInteractiveProcess() error {
	if s.cmd == nil {
		return nil
	}

	if s.stdin != nil {
		s.stdin.Close()
	}
	if s.stdout != nil {
		s.stdout.Close()
	}

	if s.cmd.Process != nil {
		if err := s.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process: %w", err)
		}
	}

	s.cmd = nil
	s.stdin = nil
	s.stdout = nil
	s.scanner = nil

	return nil
}

func (s *Service) restartInteractiveProcess() error {
	s.stopInteractiveProcess()
	return s.startInteractiveProcess()
}

func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.interactive {
		return s.stopInteractiveProcess()
	}
	return nil
}

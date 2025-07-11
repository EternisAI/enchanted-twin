package llama1b

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

type interactiveSession struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	scanner *bufio.Scanner
	cancel  context.CancelFunc
}

type LlamaResponse struct {
	GeneratedText   string `json:"generated_text"`
	TokensGenerated int    `json:"tokens_generated"`
}

type LlamaAnonymizer struct {
	binaryPath     string
	modelPath      string
	sessionTimeout time.Duration
	systemPrompt   string // TODO this wont be present for the final anonymizer model

	// Interactive session management
	session      *interactiveSession
	sessionMutex sync.RWMutex
}

func NewLlamaAnonymizer(binaryPath, modelPath string) (*LlamaAnonymizer, error) {
	// This will be the fixed tool in the correct Anonymizer model , using this system prompt on this inferencing POC model first
	systemPrompt := "Find names in the input text. For each name found, create a JSON mapping where the original name is the key and a completely different, unrelated name is the value. The replacement name must be different from the original. Return only JSON."

	anonymizer := &LlamaAnonymizer{
		binaryPath:     binaryPath,
		modelPath:      modelPath,
		sessionTimeout: 5 * time.Minute,
		systemPrompt:   systemPrompt,
	}

	if err := anonymizer.startInteractiveSession(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to start interactive session: %w", err)
	}

	return anonymizer, nil
}

func (a *LlamaAnonymizer) Close() error {
	a.sessionMutex.Lock()
	defer a.sessionMutex.Unlock()

	if a.session != nil {
		return a.closeSession()
	}
	return nil
}

func (a *LlamaAnonymizer) Anonymize(ctx context.Context, input string) (map[string]string, error) {
	return a.anonymizeInteractive(ctx, input)
}

func (a *LlamaAnonymizer) anonymizeInteractive(ctx context.Context, input string) (map[string]string, error) {
	a.sessionMutex.RLock()
	defer a.sessionMutex.RUnlock()

	if a.session == nil {
		return nil, fmt.Errorf("interactive session not initialized")
	}

	output, err := a.sendInteractiveMessage(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to send interactive message: %w", err)
	}

	var response LlamaResponse
	if err := json.Unmarshal([]byte(output), &response); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w,\noutput:%s", err, output)
	}

	var result map[string]string
	if err := json.Unmarshal([]byte(response.GeneratedText), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w,\noutput:%s", err, response.GeneratedText)
	}

	return result, nil
}

func (a *LlamaAnonymizer) startInteractiveSession(ctx context.Context) error {
	sessionCtx, cancel := context.WithCancel(ctx)

	args := []string{
		"--interactive",
		"--system-prompt", a.systemPrompt,
		"--local-model-directory", a.modelPath,
		"--tokenizer-name", "meta-llama/Llama-3.2-1B",
		"--output-format", "json",
	}

	cmd := exec.CommandContext(sessionCtx, a.binaryPath, args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("failed to start interactive session: %w", err)
	}

	a.session = &interactiveSession{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  stdout,
		scanner: bufio.NewScanner(stdout),
		cancel:  cancel,
	}
	return nil
}

func (a *LlamaAnonymizer) closeSession() error {
	if a.session == nil {
		return nil
	}

	a.session.cancel()
	_ = a.session.stdin.Close()
	_ = a.session.stdout.Close()
	_ = a.session.cmd.Wait()

	a.session = nil
	return nil
}

func (a *LlamaAnonymizer) sendInteractiveMessage(ctx context.Context, input string) (string, error) {
	if a.session == nil {
		return "", fmt.Errorf("no active interactive session")
	}

	if _, err := fmt.Fprintf(a.session.stdin, "%s\n", input); err != nil {
		return "", fmt.Errorf("failed to send input to interactive session: %w", err)
	}

	responseChan := make(chan string, 1)
	errChan := make(chan error, 1)

	go func() {
		if a.session.scanner.Scan() {
			responseChan <- a.session.scanner.Text()
		} else {
			if err := a.session.scanner.Err(); err != nil {
				errChan <- fmt.Errorf("failed to read from interactive session: %w", err)
			} else {
				errChan <- fmt.Errorf("interactive session closed unexpectedly")
			}
		}
	}()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case responseText := <-responseChan:
		return responseText, nil
	case err := <-errChan:
		return "", err
	case <-time.After(a.sessionTimeout):
		return "", fmt.Errorf("interactive session timeout after %v", a.sessionTimeout)
	}
}

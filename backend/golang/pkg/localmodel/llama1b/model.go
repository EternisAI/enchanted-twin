package llama1b

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/openai/openai-go"

	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

var _ ai.Completion = (*LlamaModel)(nil)

type LlamaModel struct {
	binaryPath      string
	modelDir        string
	tokenizerName   string
	interactiveMode bool
	sessionTimeout  time.Duration
	maxTokens       int
	temperature     float64

	// Interactive session management
	session      *interactiveSession
	sessionMutex sync.RWMutex
}

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

// Usage on binary:.
func NewLlamaModel(
	binaryPath string,
	modelDir string,
) *LlamaModel {
	return &LlamaModel{
		binaryPath:      binaryPath,
		modelDir:        modelDir,
		tokenizerName:   "meta-llama/Llama-3.2-1B",
		interactiveMode: false,
		sessionTimeout:  5 * time.Minute,
		maxTokens:       1000,
		temperature:     0.7,
	}
}

func NewInteractiveLlamaModel(
	binaryPath string,
	modelDir string,
	tokenizerName string,
) *LlamaModel {
	return &LlamaModel{
		binaryPath:      binaryPath,
		modelDir:        modelDir,
		tokenizerName:   tokenizerName,
		interactiveMode: true,
		sessionTimeout:  5 * time.Minute,
		maxTokens:       1000,
		temperature:     0.7,
	}
}

func (m *LlamaModel) SetMaxTokens(tokens int) {
	m.maxTokens = tokens
}

func (m *LlamaModel) SetTemperature(temp float64) {
	m.temperature = temp
}

func (m *LlamaModel) SetSessionTimeout(timeout time.Duration) {
	m.sessionTimeout = timeout
}

func (m *LlamaModel) Close() error {
	m.sessionMutex.Lock()
	defer m.sessionMutex.Unlock()

	if m.session != nil {
		return m.closeSession()
	}
	return nil
}

func (m *LlamaModel) Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) (openai.ChatCompletionMessage, error) {
	if m.interactiveMode {
		return m.completionsInteractive(ctx, messages)
	}
	return m.completionsOneTime(ctx, messages)
}

func (m *LlamaModel) completionsOneTime(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion) (openai.ChatCompletionMessage, error) {
	systemPrompt, userPrompt, err := m.extractPrompts(messages)
	if err != nil {
		return openai.ChatCompletionMessage{}, fmt.Errorf("failed to extract prompts: %w", err)
	}

	output, err := m.executeCLI(ctx, systemPrompt, userPrompt)
	if err != nil {
		return openai.ChatCompletionMessage{}, fmt.Errorf("failed to execute CLI: %w", err)
	}

	response := openai.ChatCompletionMessage{
		Role:    "assistant",
		Content: output,
	}

	return response, nil
}

func (m *LlamaModel) completionsInteractive(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion) (openai.ChatCompletionMessage, error) {
	m.sessionMutex.Lock()
	defer m.sessionMutex.Unlock()

	systemPrompt, userPrompt, err := m.extractPrompts(messages)
	if err != nil {
		return openai.ChatCompletionMessage{}, fmt.Errorf("failed to extract prompts: %w", err)
	}

	if m.session == nil {
		if err := m.startInteractiveSession(ctx, systemPrompt); err != nil {
			return openai.ChatCompletionMessage{}, fmt.Errorf("failed to start interactive session: %w", err)
		}
	}

	output, err := m.sendInteractiveMessage(ctx, systemPrompt, userPrompt)
	if err != nil {
		return openai.ChatCompletionMessage{}, fmt.Errorf("failed to send interactive message: %w", err)
	}

	response := openai.ChatCompletionMessage{
		Role:    "assistant",
		Content: output,
	}

	return response, nil
}

func (m *LlamaModel) extractPrompts(messages []openai.ChatCompletionMessageParamUnion) (string, string, error) {
	var systemPrompt string
	var conversationHistory []string

	for _, msg := range messages {
		if msg.OfSystem != nil && msg.OfSystem.Content.OfString.Value != "" {
			systemPrompt = msg.OfSystem.Content.OfString.Value
		} else if msg.OfUser != nil && msg.OfUser.Content.OfString.Value != "" {
			conversationHistory = append(conversationHistory, "User: "+msg.OfUser.Content.OfString.Value)
		} else if msg.OfAssistant != nil && msg.OfAssistant.Content.OfString.Value != "" {
			conversationHistory = append(conversationHistory, "Assistant: "+msg.OfAssistant.Content.OfString.Value)
		}
	}

	// Combine conversation history into user prompt
	userPrompt := strings.Join(conversationHistory, "\n")

	return systemPrompt, userPrompt, nil
}

func (m *LlamaModel) startInteractiveSession(ctx context.Context, systemPrompt string) error {
	sessionCtx, cancel := context.WithCancel(ctx)

	args := []string{
		"--interactive",
		"--local-model-directory", m.modelDir,
		"--tokenizer-name", m.tokenizerName,
		"--output-format", "json",
	}

	if systemPrompt != "" {
		args = append(args, "--system-prompt", systemPrompt)
	}

	cmd := exec.CommandContext(sessionCtx, m.binaryPath, args...)

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

	m.session = &interactiveSession{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  stdout,
		scanner: bufio.NewScanner(stdout),
		cancel:  cancel,
	}

	return nil
}

func (m *LlamaModel) closeSession() error {
	if m.session == nil {
		return nil
	}

	m.session.cancel()
	_ = m.session.stdin.Close()
	_ = m.session.stdout.Close()

	// Wait for process to terminate, but don't return error since termination is expected
	_ = m.session.cmd.Wait()

	m.session = nil
	return nil
}

func (m *LlamaModel) sendInteractiveMessage(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if m.session == nil {
		return "", fmt.Errorf("no active interactive session")
	}

	// Send the user prompt to the interactive session
	// System prompt should already be set when starting the session
	if _, err := fmt.Fprintf(m.session.stdin, "%s\n", userPrompt); err != nil {
		return "", fmt.Errorf("failed to send prompt to interactive session: %w", err)
	}

	// Read the response with timeout
	responseChan := make(chan string, 1)
	errChan := make(chan error, 1)

	go func() {
		if m.session.scanner.Scan() {
			responseChan <- m.session.scanner.Text()
		} else {
			if err := m.session.scanner.Err(); err != nil {
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
		// Parse JSON response
		var response LlamaResponse
		if err := json.Unmarshal([]byte(responseText), &response); err != nil {
			return "", fmt.Errorf("failed to parse JSON response: %w", err)
		}
		return response.GeneratedText, nil
	case err := <-errChan:
		return "", err
	case <-time.After(m.sessionTimeout):
		return "", fmt.Errorf("interactive session timeout after %v", m.sessionTimeout)
	}
}

func (m *LlamaModel) executeCLI(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	args := []string{
		"--system-prompt", systemPrompt,
		"--local-model-directory", m.modelDir,
		"--tokenizer-name", m.tokenizerName,
		"--output-format", "json",
	}

	cmd := exec.CommandContext(ctx, m.binaryPath, args...)
	cmd.Stdin = strings.NewReader(userPrompt)

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("CLI execution failed: %w", err)
	}

	// Parse JSON response
	var response LlamaResponse
	if err := json.Unmarshal(output, &response); err != nil {
		return "", fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return response.GeneratedText, nil
}

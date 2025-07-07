package coreml

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"github.com/openai/openai-go"
)

type CompletionService struct {
	binaryPath  string
	modelPath   string
	interactive bool
	process     BinaryProcess
	mu          sync.Mutex
}

func NewCompletionService(binaryPath, modelPath string, interactive bool) *CompletionService {
	return &CompletionService{
		binaryPath:  binaryPath,
		modelPath:   modelPath,
		interactive: interactive,
		process:     NewRealBinaryProcess(),
	}
}

func NewCompletionServiceWithProcess(binaryPath, modelPath string, interactive bool, process BinaryProcess) *CompletionService {
	return &CompletionService{
		binaryPath:  binaryPath,
		modelPath:   modelPath,
		interactive: interactive,
		process:     process,
	}
}

func (s *CompletionService) Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) (openai.ChatCompletionMessage, error) {
	params := openai.ChatCompletionNewParams{
		Messages: messages,
		Model:    model,
		Tools:    tools,
	}
	return s.ParamsCompletions(ctx, params)
}

func (s *CompletionService) ParamsCompletions(ctx context.Context, params openai.ChatCompletionNewParams) (openai.ChatCompletionMessage, error) {
	if s.interactive {
		return s.completionsInteractive(ctx, params)
	}
	return s.completionsNonInteractive(ctx, params)
}

func (s *CompletionService) completionsInteractive(ctx context.Context, params openai.ChatCompletionNewParams) (openai.ChatCompletionMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for retries := 0; retries < 2; retries++ {
		if !s.process.IsRunning() {
			if err := s.process.Restart(ctx, s.binaryPath, s.modelPath); err != nil {
				if retries == 1 {
					return openai.ChatCompletionMessage{}, fmt.Errorf("failed to restart process: %w", err)
				}
				continue
			}
		}

		req := Request{
			Operation: OperationCompletion,
			Model:     params.Model,
			Data: CompletionRequest{
				Messages:    params.Messages,
				Tools:       params.Tools,
				Temperature: getFloatPointer(params.Temperature),
				MaxTokens:   getIntPointer(params.MaxTokens),
			},
		}

		reqJSON, err := json.Marshal(req)
		if err != nil {
			return openai.ChatCompletionMessage{}, fmt.Errorf("failed to marshal request: %w", err)
		}

		if err := s.process.Write(append(reqJSON, '\n')); err != nil {
			if retries < 1 {
				_ = s.process.Restart(ctx, s.binaryPath, s.modelPath)
				continue
			}
			return openai.ChatCompletionMessage{}, fmt.Errorf("failed to write request: %w", err)
		}

		respLine, err := s.process.ReadLine()
		if err != nil {
			if retries < 1 {
				_ = s.process.Restart(ctx, s.binaryPath, s.modelPath)
				continue
			}
			return openai.ChatCompletionMessage{}, fmt.Errorf("failed to read response: %w", err)
		}

		var resp Response
		if err := json.Unmarshal([]byte(respLine), &resp); err != nil {
			return openai.ChatCompletionMessage{}, fmt.Errorf("failed to unmarshal response: %w", err)
		}

		if !resp.Success {
			return openai.ChatCompletionMessage{}, fmt.Errorf("completion failed: %s", resp.Error)
		}

		var completionResp CompletionResponse
		respData, err := json.Marshal(resp.Data)
		if err != nil {
			return openai.ChatCompletionMessage{}, fmt.Errorf("failed to marshal response data: %w", err)
		}

		if err := json.Unmarshal(respData, &completionResp); err != nil {
			return openai.ChatCompletionMessage{}, fmt.Errorf("failed to unmarshal completion response: %w", err)
		}

		return completionResp.Message, nil
	}

	return openai.ChatCompletionMessage{}, fmt.Errorf("failed to get completion after retries")
}

func (s *CompletionService) completionsNonInteractive(ctx context.Context, params openai.ChatCompletionNewParams) (openai.ChatCompletionMessage, error) {
	if s.process != nil {
		return s.completionsInteractive(ctx, params)
	}

	req := Request{
		Operation: OperationCompletion,
		Model:     params.Model,
		Data: CompletionRequest{
			Messages:    params.Messages,
			Tools:       params.Tools,
			Temperature: getFloatPointer(params.Temperature),
			MaxTokens:   getIntPointer(params.MaxTokens),
		},
	}

	reqJSON, err := json.Marshal(req)
	if err != nil {
		return openai.ChatCompletionMessage{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	cmd := exec.CommandContext(ctx, s.binaryPath, "completion", s.modelPath)
	cmd.Stdin = strings.NewReader(string(reqJSON))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return openai.ChatCompletionMessage{}, fmt.Errorf("failed to execute completion: %w, output: %s", err, string(output))
	}

	var resp Response
	if err := json.Unmarshal(output, &resp); err != nil {
		return openai.ChatCompletionMessage{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if !resp.Success {
		return openai.ChatCompletionMessage{}, fmt.Errorf("completion failed: %s", resp.Error)
	}

	var completionResp CompletionResponse
	respData, err := json.Marshal(resp.Data)
	if err != nil {
		return openai.ChatCompletionMessage{}, fmt.Errorf("failed to marshal response data: %w", err)
	}

	if err := json.Unmarshal(respData, &completionResp); err != nil {
		return openai.ChatCompletionMessage{}, fmt.Errorf("failed to unmarshal completion response: %w", err)
	}

	return completionResp.Message, nil
}

func (s *CompletionService) Start(ctx context.Context) error {
	if s.interactive && s.process != nil {
		return s.process.Start(ctx, s.binaryPath, s.modelPath)
	}
	return nil
}

func (s *CompletionService) Close() error {
	if s.process != nil {
		return s.process.Stop()
	}
	return nil
}

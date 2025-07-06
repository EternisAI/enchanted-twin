package coreml

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
)

type Service struct {
	binaryPath      string
	modelPath       string
	completionModel string
	embeddingModel  string
	interactive     bool
	cmd             *exec.Cmd
	stdin           io.WriteCloser
	stdout          io.ReadCloser
	scanner         *bufio.Scanner
	mu              sync.Mutex
	process         BinaryProcess
}

func NewService(binaryPath, modelPath string, interactive bool) *Service {
	s := &Service{
		binaryPath:      binaryPath,
		modelPath:       modelPath,
		completionModel: modelPath,
		embeddingModel:  modelPath,
		interactive:     interactive,
		process:         NewRealBinaryProcess(),
	}

	if interactive {
		if err := s.startInteractiveProcess(); err != nil {
			s.interactive = false
		}
	}

	return s
}

func NewServiceWithModels(binaryPath, completionModelPath, embeddingModelPath string, interactive bool) *Service {
	s := &Service{
		binaryPath:      binaryPath,
		modelPath:       completionModelPath,
		completionModel: completionModelPath,
		embeddingModel:  embeddingModelPath,
		interactive:     interactive,
		process:         NewRealBinaryProcess(),
	}

	if interactive {
		if err := s.startInteractiveProcess(); err != nil {
			s.interactive = false
		}
	}

	return s
}

func NewServiceWithProcess(binaryPath, modelPath string, interactive bool, process BinaryProcess) *Service {
	s := &Service{
		binaryPath:      binaryPath,
		modelPath:       modelPath,
		completionModel: modelPath,
		embeddingModel:  modelPath,
		interactive:     interactive,
		process:         process,
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
	if s.process != nil {
		s.mu.Lock()
		defer s.mu.Unlock()

		for retries := 0; retries < 2; retries++ {
			if !s.process.IsRunning() {
				if err := s.process.Restart(context.Background(), s.binaryPath, s.modelPath); err != nil {
					if retries == 1 {
						return "", fmt.Errorf("failed to restart process: %w", err)
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

			if err := s.process.Write(append(inputJSON, '\n')); err != nil {
				if retries < 1 {
					s.process.Restart(context.Background(), s.binaryPath, s.modelPath)
					continue
				}
				return "", fmt.Errorf("failed to write request: %w", err)
			}

			response, err := s.process.ReadLine()
			if err != nil {
				if retries < 1 {
					s.process.Restart(context.Background(), s.binaryPath, s.modelPath)
					continue
				}
				return "", fmt.Errorf("failed to read response: %w", err)
			}

			return strings.TrimSpace(response), nil
		}

		return "", fmt.Errorf("failed to get response after retries")
	}

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
	if s.process != nil {
		return s.process.Start(context.Background(), s.binaryPath, s.modelPath)
	}

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

func (s *Service) Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) (openai.ChatCompletionMessage, error) {
	params := openai.ChatCompletionNewParams{
		Messages:    messages,
		Model:       model,
		Tools:       tools,
		Temperature: param.Opt[float64]{Value: 1.0},
	}
	return s.ParamsCompletions(ctx, params)
}

func (s *Service) ParamsCompletions(ctx context.Context, params openai.ChatCompletionNewParams) (openai.ChatCompletionMessage, error) {
	if s.interactive {
		return s.completionsInteractive(ctx, params)
	}
	return s.completionsNonInteractive(ctx, params)
}

func (s *Service) completionsInteractive(ctx context.Context, params openai.ChatCompletionNewParams) (openai.ChatCompletionMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for retries := 0; retries < 2; retries++ {
		if !s.process.IsRunning() {
			if err := s.process.Restart(ctx, s.binaryPath, s.completionModel); err != nil {
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
				s.process.Restart(ctx, s.binaryPath, s.completionModel)
				continue
			}
			return openai.ChatCompletionMessage{}, fmt.Errorf("failed to write request: %w", err)
		}

		respLine, err := s.process.ReadLine()
		if err != nil {
			if retries < 1 {
				s.process.Restart(ctx, s.binaryPath, s.completionModel)
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

func (s *Service) completionsNonInteractive(ctx context.Context, params openai.ChatCompletionNewParams) (openai.ChatCompletionMessage, error) {
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

	cmd := exec.CommandContext(ctx, s.binaryPath, "completion", s.completionModel)
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

func (s *Service) Embeddings(ctx context.Context, inputs []string, model string) ([][]float64, error) {
	if s.interactive {
		return s.embeddingsInteractive(ctx, inputs, model)
	}
	return s.embeddingsNonInteractive(ctx, inputs, model)
}

func (s *Service) Embedding(ctx context.Context, input string, model string) ([]float64, error) {
	embeddings, err := s.Embeddings(ctx, []string{input}, model)
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}
	return embeddings[0], nil
}

func (s *Service) embeddingsInteractive(ctx context.Context, inputs []string, model string) ([][]float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for retries := 0; retries < 2; retries++ {
		if !s.process.IsRunning() {
			if err := s.process.Restart(ctx, s.binaryPath, s.embeddingModel); err != nil {
				if retries == 1 {
					return nil, fmt.Errorf("failed to restart process: %w", err)
				}
				continue
			}
		}

		req := Request{
			Operation: OperationEmbedding,
			Model:     model,
			Data: EmbeddingRequest{
				Inputs: inputs,
			},
		}

		reqJSON, err := json.Marshal(req)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}

		if err := s.process.Write(append(reqJSON, '\n')); err != nil {
			if retries < 1 {
				s.process.Restart(ctx, s.binaryPath, s.embeddingModel)
				continue
			}
			return nil, fmt.Errorf("failed to write request: %w", err)
		}

		respLine, err := s.process.ReadLine()
		if err != nil {
			if retries < 1 {
				s.process.Restart(ctx, s.binaryPath, s.embeddingModel)
				continue
			}
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		var resp Response
		if err := json.Unmarshal([]byte(respLine), &resp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response: %w", err)
		}

		if !resp.Success {
			return nil, fmt.Errorf("embedding failed: %s", resp.Error)
		}

		var embeddingResp EmbeddingResponse
		respData, err := json.Marshal(resp.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal response data: %w", err)
		}

		if err := json.Unmarshal(respData, &embeddingResp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal embedding response: %w", err)
		}

		return embeddingResp.Embeddings, nil
	}

	return nil, fmt.Errorf("failed to get embeddings after retries")
}

func (s *Service) embeddingsNonInteractive(ctx context.Context, inputs []string, model string) ([][]float64, error) {
	if s.process != nil {
		return s.embeddingsInteractive(ctx, inputs, model)
	}

	req := Request{
		Operation: OperationEmbedding,
		Model:     model,
		Data: EmbeddingRequest{
			Inputs: inputs,
		},
	}

	reqJSON, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	cmd := exec.CommandContext(ctx, s.binaryPath, "embedding", s.embeddingModel)
	cmd.Stdin = strings.NewReader(string(reqJSON))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to execute embedding: %w, output: %s", err, string(output))
	}

	var resp Response
	if err := json.Unmarshal(output, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("embedding failed: %s", resp.Error)
	}

	var embeddingResp EmbeddingResponse
	respData, err := json.Marshal(resp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response data: %w", err)
	}

	if err := json.Unmarshal(respData, &embeddingResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal embedding response: %w", err)
	}

	return embeddingResp.Embeddings, nil
}

func getFloatPointer(opt param.Opt[float64]) *float64 {
	if opt.Value != 0 {
		return &opt.Value
	}
	return nil
}

func getIntPointer(opt param.Opt[int64]) *int {
	if opt.Value != 0 {
		val := int(opt.Value)
		return &val
	}
	return nil
}

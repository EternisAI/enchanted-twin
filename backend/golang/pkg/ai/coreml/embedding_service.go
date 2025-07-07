package coreml

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

type EmbeddingService struct {
	binaryPath  string
	modelPath   string
	interactive bool
	process     BinaryProcess
	mu          sync.Mutex
}

func NewEmbeddingService(binaryPath, modelPath string, interactive bool) *EmbeddingService {
	return &EmbeddingService{
		binaryPath:  binaryPath,
		modelPath:   modelPath,
		interactive: interactive,
		process:     NewRealBinaryProcess(),
	}
}

func NewEmbeddingServiceWithProcess(binaryPath, modelPath string, interactive bool, process BinaryProcess) *EmbeddingService {
	return &EmbeddingService{
		binaryPath:  binaryPath,
		modelPath:   modelPath,
		interactive: interactive,
		process:     process,
	}
}

func (s *EmbeddingService) Embeddings(ctx context.Context, inputs []string, model string) ([][]float64, error) {
	if s.interactive {
		return s.embeddingsInteractive(ctx, inputs, model)
	}
	return s.embeddingsNonInteractive(ctx, inputs, model)
}

func (s *EmbeddingService) Embedding(ctx context.Context, input string, model string) ([]float64, error) {
	embeddings, err := s.Embeddings(ctx, []string{input}, model)
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}
	return embeddings[0], nil
}

func (s *EmbeddingService) embeddingsInteractive(ctx context.Context, inputs []string, model string) ([][]float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for retries := 0; retries < 2; retries++ {
		if !s.process.IsRunning() {
			if err := s.process.Restart(ctx, s.binaryPath, s.modelPath); err != nil {
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
				_ = s.process.Restart(ctx, s.binaryPath, s.modelPath)
				continue
			}
			return nil, fmt.Errorf("failed to write request: %w", err)
		}

		respLine, err := s.process.ReadLine()
		if err != nil {
			if retries < 1 {
				_ = s.process.Restart(ctx, s.binaryPath, s.modelPath)
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

func (s *EmbeddingService) embeddingsNonInteractive(ctx context.Context, inputs []string, model string) ([][]float64, error) {
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

	cmd := exec.CommandContext(ctx, s.binaryPath, "embedding", s.modelPath)
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

func (s *EmbeddingService) Start(ctx context.Context) error {
	if s.interactive && s.process != nil {
		return s.process.Start(ctx, s.binaryPath, s.modelPath)
	}
	return nil
}

func (s *EmbeddingService) Close() error {
	if s.process != nil {
		return s.process.Stop()
	}
	return nil
}

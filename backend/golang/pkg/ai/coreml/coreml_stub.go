//go:build !coreml

package coreml

import (
	"context"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

type Service struct{}

func NewService(modelPath string, logger *log.Logger) (*Service, error) {
	return nil, fmt.Errorf("CoreML support not enabled - build with -tags coreml")
}

func (s *Service) Close() {}

func (s *Service) Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) (openai.ChatCompletionMessage, error) {
	return openai.ChatCompletionMessage{}, fmt.Errorf("CoreML support not enabled")
}

func (s *Service) CompletionsStream(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) ai.Stream {
	errCh := make(chan error, 1)
	errCh <- fmt.Errorf("CoreML support not enabled")
	return ai.Stream{
		Content:   make(chan ai.StreamDelta),
		ToolCalls: make(chan openai.ChatCompletionMessageToolCall),
		Err:       errCh,
	}
}

func (s *Service) Embeddings(ctx context.Context, inputs []string, model string) ([][]float64, error) {
	return nil, fmt.Errorf("CoreML support not enabled")
}

func (s *Service) Embedding(ctx context.Context, text string, model string) ([]float64, error) {
	return nil, fmt.Errorf("CoreML support not enabled")
}
package evolvingmemory

import (
	"context"

	"github.com/openai/openai-go"
	"github.com/stretchr/testify/mock"

	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

// MockCompletionsService mocks the AI completions service
type MockCompletionsService struct {
	mock.Mock
}

func (m *MockCompletionsService) ParamsCompletions(ctx context.Context, params openai.ChatCompletionNewParams) (openai.ChatCompletionMessage, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return openai.ChatCompletionMessage{}, args.Error(1)
	}
	return args.Get(0).(openai.ChatCompletionMessage), args.Error(1)
}

func (m *MockCompletionsService) Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) (openai.ChatCompletionMessage, error) {
	args := m.Called(ctx, messages, tools, model)
	if args.Get(0) == nil {
		return openai.ChatCompletionMessage{}, args.Error(1)
	}
	return args.Get(0).(openai.ChatCompletionMessage), args.Error(1)
}

func (m *MockCompletionsService) CompletionsWithMessages(ctx context.Context, messages []ai.Message, tools []openai.ChatCompletionToolParam, model string) (ai.Message, error) {
	args := m.Called(ctx, messages, tools, model)
	if args.Get(0) == nil {
		return ai.Message{}, args.Error(1)
	}
	return args.Get(0).(ai.Message), args.Error(1)
}

// MockEmbeddingsService mocks the embeddings service
type MockEmbeddingsService struct {
	mock.Mock
}

func (m *MockEmbeddingsService) Embeddings(ctx context.Context, inputs []string, model string) ([][]float64, error) {
	args := m.Called(ctx, inputs, model)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([][]float64), args.Error(1)
}

func (m *MockEmbeddingsService) Embedding(ctx context.Context, input string, model string) ([]float64, error) {
	args := m.Called(ctx, input, model)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]float64), args.Error(1)
}

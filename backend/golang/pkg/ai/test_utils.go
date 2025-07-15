package ai

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/openai/openai-go"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockLlamaAnonymizer for testing that implements LlamaAnonymizerInterface.
type MockLlamaAnonymizer struct {
	mock.Mock
}

func (m *MockLlamaAnonymizer) Anonymize(ctx context.Context, text string) (map[string]string, error) {
	args := m.Called(ctx, text)
	if result, ok := args.Get(0).(map[string]string); ok {
		return result, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockLlamaAnonymizer) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockCompletionsService for testing.
type MockCompletionsService struct {
	mock.Mock
}

// MockServiceWithRawCompletions implements both interfaces for testing LLM anonymizer.
type MockServiceWithRawCompletions struct {
	*MockCompletionsService
}

// RawCompletions method to support the LLM anonymizer type check.
func (m *MockServiceWithRawCompletions) RawCompletions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) (PrivateCompletionResult, error) {
	args := m.Called(ctx, messages, tools, model)
	if result, ok := args.Get(0).(PrivateCompletionResult); ok {
		return result, args.Error(1)
	}
	return PrivateCompletionResult{}, args.Error(1)
}

func (m *MockCompletionsService) Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string, priority Priority) (PrivateCompletionResult, error) {
	args := m.Called(ctx, messages, tools, model, priority)
	return args.Get(0).(PrivateCompletionResult), args.Error(1) //nolint:errcheck
}

func (m *MockCompletionsService) CompletionsStream(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) Stream {
	args := m.Called(ctx, messages, tools, model)
	return args.Get(0).(Stream) //nolint:errcheck
}

func (m *MockCompletionsService) RawCompletions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) (PrivateCompletionResult, error) {
	args := m.Called(ctx, messages, tools, model)
	return args.Get(0).(PrivateCompletionResult), args.Error(1) //nolint:errcheck
}

func (m *MockCompletionsService) Embedding(ctx context.Context, input []string, model string) ([][]float32, error) {
	args := m.Called(ctx, input, model)
	return args.Get(0).([][]float32), args.Error(1) //nolint:errcheck
}

func (m *MockCompletionsService) EnablePrivateCompletions(service *PrivateCompletionsService, defaultPriority Priority) {
	m.Called(service, defaultPriority)
}

func (m *MockCompletionsService) GetPrivateCompletionsService() *PrivateCompletionsService {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*PrivateCompletionsService) //nolint:errcheck
}

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	// Create tables
	_, err = db.Exec(`
		CREATE TABLE conversation_dicts (
			conversation_id TEXT PRIMARY KEY,
			dict_data TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		CREATE TABLE anonymized_messages (
			conversation_id TEXT NOT NULL,
			message_hash TEXT NOT NULL,
			anonymized_at DATETIME NOT NULL,
			PRIMARY KEY (conversation_id, message_hash)
		)
	`)
	require.NoError(t, err)

	return db
}

package ai

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/openai/openai-go/v3"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockLlamaAnonymizer for testing.
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

type MockCompletionsService struct {
	mock.Mock
}

func (m *MockCompletionsService) Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolUnionParam, model string, priority Priority) (PrivateCompletionResult, error) {
	args := m.Called(ctx, messages, tools, model, priority)
	if result, ok := args.Get(0).(PrivateCompletionResult); ok {
		return result, args.Error(1)
	}
	return PrivateCompletionResult{}, args.Error(1)
}

func (m *MockCompletionsService) CompletionsStream(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolUnionParam, model string) Stream {
	args := m.Called(ctx, messages, tools, model)
	return args.Get(0).(Stream) //nolint:errcheck
}

func (m *MockCompletionsService) RawCompletions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolUnionParam, model string) (PrivateCompletionResult, error) {
	args := m.Called(ctx, messages, tools, model)
	if result, ok := args.Get(0).(PrivateCompletionResult); ok {
		return result, args.Error(1)
	}
	return PrivateCompletionResult{}, args.Error(1)
}

func (m *MockCompletionsService) Embedding(ctx context.Context, input []string, model string) ([][]float32, error) {
	args := m.Called(ctx, input, model)
	if result, ok := args.Get(0).([][]float32); ok {
		return result, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockCompletionsService) EnablePrivateCompletions(service *PrivateCompletionsService, defaultPriority Priority) {
	m.Called(service, defaultPriority)
}

func (m *MockCompletionsService) GetPrivateCompletionsService() *PrivateCompletionsService {
	args := m.Called()
	if result, ok := args.Get(0).(*PrivateCompletionsService); ok {
		return result
	}
	return nil
}

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	_, err = db.Exec(`
		CREATE TABLE conversation_dicts (
			conversation_id TEXT PRIMARY KEY NOT NULL,
			dict_data TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		CREATE TABLE anonymized_messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			conversation_id TEXT NOT NULL,
			message_hash TEXT NOT NULL,
			anonymized_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(conversation_id, message_hash),
			FOREIGN KEY (conversation_id) REFERENCES conversation_dicts(conversation_id) ON DELETE CASCADE
		)
	`)
	require.NoError(t, err)

	_, err = db.Exec(`CREATE INDEX idx_conversation_messages ON anonymized_messages(conversation_id)`)
	require.NoError(t, err)

	_, err = db.Exec(`CREATE INDEX idx_message_hash ON anonymized_messages(message_hash)`)
	require.NoError(t, err)

	_, err = db.Exec(`CREATE INDEX idx_conversation_updated ON conversation_dicts(updated_at)`)
	require.NoError(t, err)

	return db
}

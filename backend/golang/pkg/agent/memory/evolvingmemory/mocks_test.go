package evolvingmemory

import (
	"context"

	"github.com/openai/openai-go"
	"github.com/stretchr/testify/mock"
	"github.com/weaviate/weaviate/entities/models"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory/storage"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

// MockStorage mocks the storage interface to avoid Weaviate client issues.
type MockStorage struct {
	mock.Mock
}

func (m *MockStorage) GetByID(ctx context.Context, id string) (*memory.TextDocument, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	doc, _ := args.Get(0).(*memory.TextDocument)
	return doc, args.Error(1)
}

func (m *MockStorage) Update(ctx context.Context, id string, doc memory.TextDocument, vector []float32) error {
	args := m.Called(ctx, id, doc, vector)
	return args.Error(0)
}

func (m *MockStorage) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockStorage) StoreBatch(ctx context.Context, objects []*models.Object) error {
	args := m.Called(ctx, objects)
	return args.Error(0)
}

func (m *MockStorage) DeleteAll(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockStorage) Query(ctx context.Context, queryText string) (memory.QueryResult, error) {
	args := m.Called(ctx, queryText)
	if args.Get(0) == nil {
		return memory.QueryResult{}, args.Error(1)
	}
	result, _ := args.Get(0).(memory.QueryResult)
	return result, args.Error(1)
}

func (m *MockStorage) QueryWithDistance(ctx context.Context, queryText string, metadataFilters ...map[string]string) (memory.QueryWithDistanceResult, error) {
	args := m.Called(ctx, queryText, metadataFilters)
	if args.Get(0) == nil {
		return memory.QueryWithDistanceResult{}, args.Error(1)
	}
	result, _ := args.Get(0).(memory.QueryWithDistanceResult)
	return result, args.Error(1)
}

func (m *MockStorage) EnsureSchemaExists(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockStorage) GetDocumentReference(ctx context.Context, memoryID string) (*storage.DocumentReference, error) {
	args := m.Called(ctx, memoryID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	docRef, _ := args.Get(0).(*storage.DocumentReference)
	return docRef, args.Error(1)
}

func (m *MockStorage) GetDocumentReferences(ctx context.Context, memoryID string) ([]*storage.DocumentReference, error) {
	args := m.Called(ctx, memoryID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	docRefs, _ := args.Get(0).([]*storage.DocumentReference)
	return docRefs, args.Error(1)
}

func (m *MockStorage) StoreDocument(ctx context.Context, content, docType, originalID string, metadata map[string]string) (string, error) {
	args := m.Called(ctx, content, docType, originalID, metadata)
	return args.String(0), args.Error(1)
}

func (m *MockStorage) GetStoredDocument(ctx context.Context, documentID string) (*storage.StoredDocument, error) {
	args := m.Called(ctx, documentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	storedDoc, _ := args.Get(0).(*storage.StoredDocument)
	return storedDoc, args.Error(1)
}

// Ensure MockStorage implements the storage interface.
var _ storage.Interface = (*MockStorage)(nil)

// MockCompletionsService mocks the AI completions service.
type MockCompletionsService struct {
	mock.Mock
}

func (m *MockCompletionsService) ParamsCompletions(ctx context.Context, params openai.ChatCompletionNewParams) (openai.ChatCompletionMessage, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return openai.ChatCompletionMessage{}, args.Error(1)
	}
	msg, _ := args.Get(0).(openai.ChatCompletionMessage)
	return msg, args.Error(1)
}

func (m *MockCompletionsService) Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) (openai.ChatCompletionMessage, error) {
	args := m.Called(ctx, messages, tools, model)
	if args.Get(0) == nil {
		return openai.ChatCompletionMessage{}, args.Error(1)
	}
	msg, _ := args.Get(0).(openai.ChatCompletionMessage)
	return msg, args.Error(1)
}

func (m *MockCompletionsService) CompletionsWithMessages(ctx context.Context, messages []ai.Message, tools []openai.ChatCompletionToolParam, model string) (ai.Message, error) {
	args := m.Called(ctx, messages, tools, model)
	if args.Get(0) == nil {
		return ai.Message{}, args.Error(1)
	}
	msg, _ := args.Get(0).(ai.Message)
	return msg, args.Error(1)
}

func (m *MockCompletionsService) Embeddings(ctx context.Context, inputs []string, model string) ([][]float64, error) {
	args := m.Called(ctx, inputs, model)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	embeddings, _ := args.Get(0).([][]float64)
	return embeddings, args.Error(1)
}

func (m *MockCompletionsService) Embedding(ctx context.Context, input string, model string) ([]float64, error) {
	args := m.Called(ctx, input, model)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	embedding, _ := args.Get(0).([]float64)
	return embedding, args.Error(1)
}

// MockEmbeddingsService mocks the embeddings service.
type MockEmbeddingsService struct {
	mock.Mock
}

func (m *MockEmbeddingsService) ParamsCompletions(ctx context.Context, params openai.ChatCompletionNewParams) (openai.ChatCompletionMessage, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return openai.ChatCompletionMessage{}, args.Error(1)
	}
	msg, _ := args.Get(0).(openai.ChatCompletionMessage)
	return msg, args.Error(1)
}

func (m *MockEmbeddingsService) Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) (openai.ChatCompletionMessage, error) {
	args := m.Called(ctx, messages, tools, model)
	if args.Get(0) == nil {
		return openai.ChatCompletionMessage{}, args.Error(1)
	}
	msg, _ := args.Get(0).(openai.ChatCompletionMessage)
	return msg, args.Error(1)
}

func (m *MockEmbeddingsService) CompletionsWithMessages(ctx context.Context, messages []ai.Message, tools []openai.ChatCompletionToolParam, model string) (ai.Message, error) {
	args := m.Called(ctx, messages, tools, model)
	if args.Get(0) == nil {
		return ai.Message{}, args.Error(1)
	}
	msg, _ := args.Get(0).(ai.Message)
	return msg, args.Error(1)
}

func (m *MockEmbeddingsService) Embeddings(ctx context.Context, inputs []string, model string) ([][]float64, error) {
	args := m.Called(ctx, inputs, model)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	embeddings, _ := args.Get(0).([][]float64)
	return embeddings, args.Error(1)
}

func (m *MockEmbeddingsService) Embedding(ctx context.Context, input string, model string) ([]float64, error) {
	args := m.Called(ctx, input, model)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	embedding, _ := args.Get(0).([]float64)
	return embedding, args.Error(1)
}

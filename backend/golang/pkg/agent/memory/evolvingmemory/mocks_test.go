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

func (m *MockStorage) GetByID(ctx context.Context, id string) (*memory.MemoryFact, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	fact, _ := args.Get(0).(*memory.MemoryFact)
	return fact, args.Error(1)
}

func (m *MockStorage) Update(ctx context.Context, id string, fact *memory.MemoryFact, vector []float32) error {
	args := m.Called(ctx, id, fact, vector)
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

func (m *MockStorage) Query(ctx context.Context, queryText string, filter *memory.Filter) (memory.QueryResult, error) {
	args := m.Called(ctx, queryText, filter)
	if args.Get(0) == nil {
		return memory.QueryResult{}, args.Error(1)
	}
	result, _ := args.Get(0).(memory.QueryResult)
	return result, args.Error(1)
}

func (m *MockStorage) EnsureSchemaExists(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockStorage) GetDocumentReferences(ctx context.Context, memoryID string) ([]*storage.DocumentReference, error) {
	args := m.Called(ctx, memoryID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	docRefs, _ := args.Get(0).([]*storage.DocumentReference)
	return docRefs, args.Error(1)
}

func (m *MockStorage) UpsertDocument(ctx context.Context, doc memory.Document) (string, error) {
	args := m.Called(ctx, doc)
	return args.String(0), args.Error(1)
}

func (m *MockStorage) GetStoredDocument(ctx context.Context, documentID string) (*storage.StoredDocument, error) {
	args := m.Called(ctx, documentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	doc, _ := args.Get(0).(*storage.StoredDocument)
	return doc, args.Error(1)
}

func (m *MockStorage) GetStoredDocumentsBatch(ctx context.Context, documentIDs []string) ([]*storage.StoredDocument, error) {
	args := m.Called(ctx, documentIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	docs, _ := args.Get(0).([]*storage.StoredDocument)
	return docs, args.Error(1)
}

func (m *MockStorage) GetFactsByIDs(ctx context.Context, factIDs []string) ([]*memory.MemoryFact, error) {
	args := m.Called(ctx, factIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	facts, _ := args.Get(0).([]*memory.MemoryFact)
	return facts, args.Error(1)
}

// Ensure MockStorage implements the storage interface.
var _ storage.Interface = (*MockStorage)(nil)

// MockCompletionsService mocks the AI completions service.
type MockCompletionsService struct {
	mock.Mock
}

func (m *MockCompletionsService) Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) (ai.PrivateResult, error) {
	args := m.Called(ctx, messages, tools, model)
	if args.Get(0) == nil {
		return ai.PrivateResult{}, args.Error(1)
	}
	// Check if it's a PrivateResult or just a ChatCompletionMessage
	if result, ok := args.Get(0).(ai.PrivateResult); ok {
		return result, args.Error(1)
	}
	// Fallback: wrap ChatCompletionMessage in PrivateResult
	if msg, ok := args.Get(0).(openai.ChatCompletionMessage); ok {
		return ai.PrivateResult{
			Message:          msg,
			ReplacementRules: make(map[string]string),
		}, args.Error(1)
	}
	return ai.PrivateResult{}, args.Error(1)
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

func (m *MockEmbeddingsService) Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) (openai.ChatCompletionMessage, error) {
	args := m.Called(ctx, messages, tools, model)
	if args.Get(0) == nil {
		return openai.ChatCompletionMessage{}, args.Error(1)
	}
	msg, _ := args.Get(0).(openai.ChatCompletionMessage)
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

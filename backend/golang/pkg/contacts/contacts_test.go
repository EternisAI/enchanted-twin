package contacts

import (
	"context"
	"fmt"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcweaviate "github.com/testcontainers/testcontainers-go/modules/weaviate"
	"github.com/weaviate/weaviate-go-client/v5/weaviate"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/contacts/storage"
	"github.com/EternisAI/enchanted-twin/pkg/helpers"
)

// MockMemoryStorage is a mock implementation of memory.Storage
type MockMemoryStorage struct {
	mock.Mock
}

func (m *MockMemoryStorage) Store(ctx context.Context, documents []memory.Document, progressCallback memory.ProgressCallback) error {
	args := m.Called(ctx, documents, progressCallback)
	return args.Error(0)
}

func (m *MockMemoryStorage) Query(ctx context.Context, query string, filter *memory.Filter) (memory.QueryResult, error) {
	// Return hardcoded conversation data
	if filter != nil && filter.ContactName != nil {
		switch *filter.ContactName {
		case "Alice Johnson":
			return memory.QueryResult{
				Documents: []memory.TextDocument{
					{
						FieldContent: "Alice: I work in software engineering and love solving complex problems",
					},
				},
			}, nil
		case "Bob Smith":
			return memory.QueryResult{
				Documents: []memory.TextDocument{
					{
						FieldContent: "Bob: I'm a teacher and enjoy hiking on weekends",
					},
				},
			}, nil
		default:
			return memory.QueryResult{Documents: []memory.TextDocument{}}, nil
		}
	}
	return memory.QueryResult{}, nil
}

func (m *MockMemoryStorage) QueryWithDistance(ctx context.Context, query string, metadataFilters ...map[string]string) (memory.QueryWithDistanceResult, error) {
	args := m.Called(ctx, query, metadataFilters)
	return args.Get(0).(memory.QueryWithDistanceResult), args.Error(1)
}

// MockAI is a mock implementation of AI interface
type MockAI struct {
	mock.Mock
	callCount int
}

func (m *MockAI) Completions(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam, model string) (openai.ChatCompletionMessage, error) {
	// Simple mock that returns different insights based on a counter
	// This works because we know the order of calls in our tests
	m.callCount++
	switch m.callCount {
	case 1:
		// First call is for Alice Johnson
		return openai.ChatCompletionMessage{
			Content: "Alice is a software engineer who is passionate about problem-solving and technical challenges.",
		}, nil
	case 2:
		// Second call is for Bob Smith
		return openai.ChatCompletionMessage{
			Content: "Bob is an educator who values work-life balance and enjoys outdoor activities like hiking.",
		}, nil
	case 3:
		// Third call is for updating Alice's insight
		return openai.ChatCompletionMessage{
			Content: "Alice is a software engineer who is passionate about problem-solving and technical challenges.",
		}, nil
	default:
		return openai.ChatCompletionMessage{Content: ""}, nil
	}
}

func TestContactsWorkflowWithRealWeaviate(t *testing.T) {
	ctx := context.Background()

	// Start Weaviate container
	weaviateContainer, err := tcweaviate.Run(ctx, "semitechnologies/weaviate:1.30.6")
	defer func() {
		if err := testcontainers.TerminateContainer(weaviateContainer); err != nil {
			t.Logf("failed to terminate container: %s", err)
		}
	}()
	require.NoError(t, err, "failed to start weaviate container")

	host, err := weaviateContainer.Host(ctx)
	require.NoError(t, err)

	port, err := weaviateContainer.MappedPort(ctx, "8080")
	require.NoError(t, err)

	client, err := weaviate.NewClient(weaviate.Config{
		Host:   fmt.Sprintf("%s:%s", host, port.Port()),
		Scheme: "http",
	})
	require.NoError(t, err, "failed to create weaviate client")

	logger := log.Default()

	// Ensure schema exists
	err = storage.EnsureContactSchemaExists(client, logger)
	require.NoError(t, err, "failed to ensure schema exists")

	// Create real storage
	contactStorage := storage.NewStorage(client, logger)

	// Setup mocks for memory and AI
	mockMemory := new(MockMemoryStorage)
	mockAI := new(MockAI)

	// Create service with real storage and mocked memory/AI
	service := NewService(contactStorage, mockMemory, mockAI, "test-model")

	// Test 1: Add first contact
	contact1, err := service.AddContact(ctx, "Alice Johnson", []string{"colleague", "engineer"})
	require.NoError(t, err)
	assert.NotEmpty(t, contact1.ID)
	assert.Equal(t, "Alice Johnson", contact1.ContactName)
	assert.Equal(t, []string{"colleague", "engineer"}, contact1.Tags)
	assert.Equal(t, "Alice is a software engineer who is passionate about problem-solving and technical challenges.", helpers.SafeDeref(contact1.Insight))

	// Test 2: Add second contact
	contact2, err := service.AddContact(ctx, "Bob Smith", []string{"friend"})
	require.NoError(t, err)
	assert.NotEmpty(t, contact2.ID)
	assert.Equal(t, "Bob Smith", contact2.ContactName)
	assert.Equal(t, []string{"friend"}, contact2.Tags)
	assert.Equal(t, "Bob is an educator who values work-life balance and enjoys outdoor activities like hiking.", helpers.SafeDeref(contact2.Insight))

	// Test 3: List contacts - should have both contacts with insights
	contacts, err := contactStorage.ListContacts(ctx)
	require.NoError(t, err)
	assert.Len(t, contacts, 2)

	// Verify both contacts have insights
	for _, contact := range contacts {
		assert.NotNil(t, contact.Insight)
		assert.NotEmpty(t, helpers.SafeDeref(contact.Insight))
		t.Logf("Contact: %s, Insight: %s", contact.ContactName, helpers.SafeDeref(contact.Insight))
	}

	// Test 4: Update contact insight
	err = service.UpdateContactInsight(ctx, contact1.ID)
	require.NoError(t, err)

	// Verify the insight was updated
	updatedContact, err := contactStorage.GetContact(ctx, contact1.ID)
	require.NoError(t, err)
	assert.NotNil(t, updatedContact.Insight)
	assert.Equal(t, "Alice is a software engineer who is passionate about problem-solving and technical challenges.", helpers.SafeDeref(updatedContact.Insight))

	// Test 5: Final list to verify all contacts still have insights
	finalContacts, err := contactStorage.ListContacts(ctx)
	require.NoError(t, err)
	assert.Len(t, finalContacts, 2)

	for _, contact := range finalContacts {
		assert.NotNil(t, contact.Insight)
		assert.NotEmpty(t, helpers.SafeDeref(contact.Insight))
	}
}

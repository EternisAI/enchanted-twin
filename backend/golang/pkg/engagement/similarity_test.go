package engagement

import (
	"context"
	"os"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
)

type MockMemoryService struct {
	mock.Mock
}

func (m *MockMemoryService) Store(ctx context.Context, documents []memory.Document, progressCallback memory.ProgressCallback) error {
	args := m.Called(ctx, documents, progressCallback)
	return args.Error(0)
}

func (m *MockMemoryService) Query(ctx context.Context, query string, filter *memory.Filter) (memory.QueryResult, error) {
	args := m.Called(ctx, query, filter)
	err := args.Error(1)
	if err != nil {
		return memory.QueryResult{}, err
	}
	result, ok := args.Get(0).(memory.QueryResult)
	if !ok {
		return memory.QueryResult{}, nil
	}
	return result, err
}

func TestCheckForSimilarFriendMessages(t *testing.T) {
	ctx := context.Background()

	// Setup mock services
	mockMemoryService := &MockMemoryService{}
	logger := log.New(os.Stdout)

	friendService := &FriendService{
		memoryService: mockMemoryService,
		logger:        logger,
	}

	testMessage := "How are you doing today?"
	expectedFilter := &memory.Filter{
		Distance: SimilarityThreshold,
		Tags: &memory.TagsFilter{
			All: []string{"sent_message"},
		},
	}

	// Test case 1: No similar messages found
	mockMemoryService.On("Query", ctx, testMessage, expectedFilter).Return(
		memory.QueryResult{
			Facts: []memory.MemoryFact{},
		}, nil)

	isSimilar, err := friendService.CheckForSimilarFriendMessages(ctx, testMessage)
	assert.NoError(t, err)
	assert.False(t, isSimilar)
	mockMemoryService.AssertExpectations(t)

	// Test case 2: Similar message found (within distance threshold)
	mockMemoryService.ExpectedCalls = nil // Reset mock
	mockMemoryService.On("Query", ctx, testMessage, expectedFilter).Return(
		memory.QueryResult{
			Facts: []memory.MemoryFact{
				{
					ID:      "test-id",
					Source:  "friend",
					Content: "How are you feeling today?",
					Metadata: map[string]string{
						"type":          "friend",
						"activity_type": "question",
					},
				},
			},
		}, nil)

	isSimilar, err = friendService.CheckForSimilarFriendMessages(ctx, testMessage)
	assert.NoError(t, err)
	assert.True(t, isSimilar)
	mockMemoryService.AssertExpectations(t)
}

func TestStoreSentMessage(t *testing.T) {
	logger := log.New(os.Stdout)
	mockMemory := &MockMemoryService{}

	friendService := &FriendService{
		logger:        logger,
		memoryService: mockMemory,
	}

	ctx := context.Background()
	testMessage := "Test message"
	activityType := "poke_message"

	mockMemory.On("Store", ctx, mock.MatchedBy(func(docs []memory.Document) bool {
		if len(docs) != 1 {
			return false
		}
		doc := docs[0]
		return doc.Content() == testMessage &&
			doc.Metadata()["type"] == FriendMetadataType &&
			doc.Metadata()["activity_type"] == activityType &&
			len(doc.Tags()) == 2 &&
			doc.Tags()[0] == "sent_message" &&
			doc.Tags()[1] == activityType
	}), mock.AnythingOfType("memory.ProgressCallback")).Return(nil)

	err := friendService.StoreSentMessage(ctx, testMessage, activityType)

	assert.NoError(t, err)
	mockMemory.AssertExpectations(t)
}

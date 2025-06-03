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

func (m *MockMemoryService) Query(ctx context.Context, query string) (memory.QueryResult, error) {
	args := m.Called(ctx, query)
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

func (m *MockMemoryService) QueryWithDistance(ctx context.Context, query string, metadataFilters ...map[string]string) (memory.QueryWithDistanceResult, error) {
	args := m.Called(ctx, query, metadataFilters)
	err := args.Error(1)
	if err != nil {
		return memory.QueryWithDistanceResult{}, err
	}
	result, ok := args.Get(0).(memory.QueryWithDistanceResult)
	if !ok {
		return memory.QueryWithDistanceResult{}, nil
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
	expectedFilters := []map[string]string{{"type": FriendMetadataType}}

	// Test case 1: No similar messages found
	mockMemoryService.On("QueryWithDistance", ctx, testMessage, expectedFilters).Return(
		memory.QueryWithDistanceResult{
			Documents: []memory.DocumentWithDistance{},
		}, nil)

	isSimilar, err := friendService.CheckForSimilarFriendMessages(ctx, testMessage)
	assert.NoError(t, err)
	assert.False(t, isSimilar)
	mockMemoryService.AssertExpectations(t)

	// Test case 2: Similar message found (distance below threshold)
	mockMemoryService.ExpectedCalls = nil // Reset mock
	mockMemoryService.On("QueryWithDistance", ctx, testMessage, expectedFilters).Return(
		memory.QueryWithDistanceResult{
			Documents: []memory.DocumentWithDistance{
				{
					Document: memory.TextDocument{
						FieldSource:  "friend",
						FieldContent: "How are you feeling today?",
						FieldMetadata: map[string]string{
							"type":          "friend",
							"activity_type": "question",
						},
					},
					Distance: 0.05, // Below threshold
				},
			},
		}, nil)

	isSimilar, err = friendService.CheckForSimilarFriendMessages(ctx, testMessage)
	assert.NoError(t, err)
	assert.True(t, isSimilar)
	mockMemoryService.AssertExpectations(t)

	// Test case 3: Message found but distance above threshold
	mockMemoryService.ExpectedCalls = nil // Reset mock
	mockMemoryService.On("QueryWithDistance", ctx, testMessage, expectedFilters).Return(
		memory.QueryWithDistanceResult{
			Documents: []memory.DocumentWithDistance{
				{
					Document: memory.TextDocument{
						FieldContent: "What's the weather like?",
						FieldMetadata: map[string]string{
							"type":          "friend",
							"activity_type": "question",
						},
					},
					Distance: 0.8, // Above threshold
				},
			},
		}, nil)

	isSimilar, err = friendService.CheckForSimilarFriendMessages(ctx, testMessage)
	assert.NoError(t, err)
	assert.False(t, isSimilar)
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

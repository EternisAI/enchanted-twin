package friend

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
)

type MockMemoryService struct {
	mock.Mock
}

func (m *MockMemoryService) Store(ctx context.Context, documents []memory.TextDocument, progressChan chan<- memory.ProgressUpdate) error {
	args := m.Called(ctx, documents, progressChan)
	return args.Error(0)
}

func (m *MockMemoryService) Query(ctx context.Context, query string) (memory.QueryResult, error) {
	args := m.Called(ctx, query)
	return args.Get(0).(memory.QueryResult), args.Error(1)
}

func (m *MockMemoryService) QueryWithDistance(ctx context.Context, query string) (memory.QueryWithDistanceResult, error) {
	args := m.Called(ctx, query)
	return args.Get(0).(memory.QueryWithDistanceResult), args.Error(1)
}

func TestCheckForSimilarMessages(t *testing.T) {
	logger := log.New(os.Stdout)
	mockMemory := &MockMemoryService{}

	friendService := &FriendService{
		logger:        logger,
		memoryService: mockMemory,
	}

	ctx := context.Background()
	testMessage := "Hey, how are you doing today?"

	t.Run("should return true when similar message found", func(t *testing.T) {
		now := time.Now()
		similarDoc := memory.DocumentWithDistance{
			Document: memory.TextDocument{
				ID:        "test-id",
				Content:   "Hey, how are you doing?",
				Timestamp: &now,
				Metadata: map[string]string{
					"type": FriendMetadataType,
				},
			},
			Distance: 0.1, // Below threshold
		}

		mockMemory.On("QueryWithDistance", ctx, testMessage).Return(
			memory.QueryWithDistanceResult{
				Documents: []memory.DocumentWithDistance{similarDoc},
			}, nil)

		isSimilar, err := friendService.CheckForSimilarMessages(ctx, testMessage)

		assert.NoError(t, err)
		assert.True(t, isSimilar)
		mockMemory.AssertExpectations(t)
	})

	t.Run("should return false when no similar message found", func(t *testing.T) {
		mockMemory.ExpectedCalls = nil // Reset mock
		now := time.Now()
		differentDoc := memory.DocumentWithDistance{
			Document: memory.TextDocument{
				ID:        "test-id-2",
				Content:   "Completely different message about weather",
				Timestamp: &now,
				Metadata: map[string]string{
					"type": FriendMetadataType,
				},
			},
			Distance: 0.8, // Above threshold
		}

		mockMemory.On("QueryWithDistance", ctx, testMessage).Return(
			memory.QueryWithDistanceResult{
				Documents: []memory.DocumentWithDistance{differentDoc},
			}, nil)

		isSimilar, err := friendService.CheckForSimilarMessages(ctx, testMessage)

		assert.NoError(t, err)
		assert.False(t, isSimilar)
		mockMemory.AssertExpectations(t)
	})

	t.Run("should return false when no friend messages found", func(t *testing.T) {
		mockMemory.ExpectedCalls = nil // Reset mock
		now := time.Now()
		nonFriendDoc := memory.DocumentWithDistance{
			Document: memory.TextDocument{
				ID:        "test-id-3",
				Content:   "Hey, how are you doing?",
				Timestamp: &now,
				Metadata: map[string]string{
					"type": "user_memory", // Different type
				},
			},
			Distance: 0.1, // Below threshold but wrong type
		}

		mockMemory.On("QueryWithDistance", ctx, testMessage).Return(
			memory.QueryWithDistanceResult{
				Documents: []memory.DocumentWithDistance{nonFriendDoc},
			}, nil)

		isSimilar, err := friendService.CheckForSimilarMessages(ctx, testMessage)

		assert.NoError(t, err)
		assert.False(t, isSimilar)
		mockMemory.AssertExpectations(t)
	})
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

	mockMemory.On("Store", ctx, mock.MatchedBy(func(docs []memory.TextDocument) bool {
		if len(docs) != 1 {
			return false
		}
		doc := docs[0]
		return doc.Content == testMessage &&
			doc.Metadata["type"] == FriendMetadataType &&
			doc.Metadata["activity_type"] == activityType &&
			len(doc.Tags) == 2 &&
			doc.Tags[0] == "sent_message" &&
			doc.Tags[1] == activityType
	}), mock.AnythingOfType("chan<- memory.ProgressUpdate")).Return(nil)

	err := friendService.StoreSentMessage(ctx, testMessage, activityType)

	assert.NoError(t, err)
	mockMemory.AssertExpectations(t)
}

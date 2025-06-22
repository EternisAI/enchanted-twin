package main

import (
	"context"
	"time"

	"github.com/enchanted-twin/backend/golang/graph/model"
	"github.com/enchanted-twin/backend/golang/pkg/agent/memory"
	"github.com/enchanted-twin/backend/golang/pkg/agent/memory/evolvingmemory/storage"
)

// MockStorage provides a mock implementation for testing
type MockStorage struct {
	conversations map[string]*memory.ConversationDocument
	users         map[string]*model.UserProfile
}

// NewMockStorage creates a new mock storage instance
func NewMockStorage() *MockStorage {
	return &MockStorage{
		conversations: make(map[string]*memory.ConversationDocument),
		users:         make(map[string]*model.UserProfile),
	}
}

// GetConversations returns mock conversation data
func (m *MockStorage) GetConversations(ctx context.Context, userID string, limit int) ([]*memory.ConversationDocument, error) {
	var conversations []*memory.ConversationDocument
	count := 0

	for _, conv := range m.conversations {
		if count >= limit {
			break
		}
		conversations = append(conversations, conv)
		count++
	}

	return conversations, nil
}

// GetUserProfile returns mock user profile data
func (m *MockStorage) GetUserProfile(ctx context.Context, userID string) (*model.UserProfile, error) {
	if user, exists := m.users[userID]; exists {
		return user, nil
	}

	// Return default mock user
	return &model.UserProfile{
		ID:        &userID,
		Name:      stringPtr("Test User"),
		CreatedAt: time.Now().Format(time.RFC3339),
	}, nil
}

// GetDocumentReferences returns mock document references
func (m *MockStorage) GetDocumentReferences(ctx context.Context, memoryID string) ([]*storage.DocumentReference, error) {
	// Return a mock conversation document reference
	return []*storage.DocumentReference{
		{
			ID:      "mock-doc-1",
			Content: "User: I love coffee in the morning\nAssistant: That's great! Coffee is a wonderful way to start the day.",
			Type:    "conversation",
		},
	}, nil
}

// AddMockConversation adds a conversation to the mock storage
func (m *MockStorage) AddMockConversation(id string, conversation *memory.ConversationDocument) {
	m.conversations[id] = conversation
}

// AddMockUser adds a user to the mock storage
func (m *MockStorage) AddMockUser(id string, user *model.UserProfile) {
	m.users[id] = user
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}

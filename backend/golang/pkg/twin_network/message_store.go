package twin_network

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// NetworkMessage represents a single message published on the twin network.
// This struct mirrors the GraphQL type definition, but uses Go native types.
type NetworkMessage struct {
	AuthorPubKey string
	NetworkID    string
	Content      string
	CreatedAt    time.Time
	ID           int64
	ThreadID     string
	IsMine       bool
	Signature    string
}

// MessageStore is a concurrency-safe in-memory store for NetworkMessage items.
// For the first iteration we keep everything in memory; this can be replaced
// by a persistent store later if needed.
type MessageStore struct {
	mu       sync.RWMutex
	messages []NetworkMessage
	nextID   int64 // monotonically increasing counter for assigning message IDs
}

// NewMessageStore returns a ready-to-use empty MessageStore.
func NewMessageStore() *MessageStore {
	return &MessageStore{
		messages: make([]NetworkMessage, 0),
		nextID:   1,
	}
}

// Add appends a message to the store.
func (s *MessageStore) Add(msg NetworkMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	msg.ID = s.nextID
	s.nextID++
	s.messages = append(s.messages, msg)
}

// GetSince returns all messages that belong to the given networkID and were
// created strictly after the provided timestamp.
func (s *MessageStore) GetSince(networkID string, fromID int64, limit *int) []NetworkMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []NetworkMessage
	for _, m := range s.messages {
		if m.NetworkID == networkID && m.ID > fromID {
			out = append(out, m)
		}
	}

	// Sort messages by CreatedAt in descending order
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})

	if limit != nil && len(out) > *limit {
		return out[:*limit]
	}
	return out
}

// String returns a human-readable representation of the message – suitable for
// feeding directly into an LLM prompt.
func (m NetworkMessage) String() string {
	return fmt.Sprintf("Message[%d] from %s on network %s at %s: %s", m.ID, m.AuthorPubKey, m.NetworkID, m.CreatedAt.Format(time.RFC3339), m.Content)
}

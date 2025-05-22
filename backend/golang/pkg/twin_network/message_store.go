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
	AuthorPubKey       string
	NetworkID          string
	Content            string
	CreatedAt          time.Time
	ID                 int64
	ThreadID           string
	ThreadAuthorPubKey string // The author/organizer of the thread
	IsMine             bool
	Signature          string
}

// Thread represents a conversation thread in the twin network
type Thread struct {
	ID           string
	AuthorPubKey string
	UpdatedAt    time.Time
}

// MessageStore is a concurrency-safe in-memory store for NetworkMessage items.
// For the first iteration we keep everything in memory; this can be replaced
// by a persistent store later if needed.
type MessageStore struct {
	mu       sync.RWMutex
	messages []NetworkMessage
	threads  map[string]Thread // Map of ThreadID -> Thread
	nextID   int64             // monotonically increasing counter for assigning message IDs
}

// NewMessageStore returns a ready-to-use empty MessageStore.
func NewMessageStore() *MessageStore {
	return &MessageStore{
		messages: make([]NetworkMessage, 0),
		threads:  make(map[string]Thread),
		nextID:   1,
	}
}

// Add appends a message to the store.
func (s *MessageStore) Add(msg NetworkMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Assign ID to message
	msg.ID = s.nextID
	s.nextID++

	if msg.ThreadID != "" {
		thread, exists := s.threads[msg.ThreadID]
		if !exists {
			// Creating a new thread - the creator is the thread author
			s.threads[msg.ThreadID] = Thread{
				ID:           msg.ThreadID,
				AuthorPubKey: msg.AuthorPubKey,
				UpdatedAt:    msg.CreatedAt,
			}
		} else {
			// Only update the timestamp for existing threads, preserving the original author
			thread.UpdatedAt = msg.CreatedAt
			s.threads[msg.ThreadID] = thread
		}
	}

	s.messages = append(s.messages, msg)
}

// GetSince returns all messages that belong to the given networkID and were
// created strictly after the provided timestamp.
func (s *MessageStore) GetSince(networkID string, from time.Time, limit *int) []NetworkMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []NetworkMessage
	for _, m := range s.messages {
		if m.NetworkID == networkID && m.CreatedAt.After(from) {
			out = append(out, m)
		}
	}

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

// GetThread returns a thread by its ID
func (s *MessageStore) GetThread(threadID string) (Thread, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	thread, exists := s.threads[threadID]
	return thread, exists
}

// GetAllThreads returns all threads
func (s *MessageStore) GetAllThreads() []Thread {
	s.mu.RLock()
	defer s.mu.RUnlock()

	threads := make([]Thread, 0, len(s.threads))
	for _, thread := range s.threads {
		threads = append(threads, thread)
	}

	return threads
}

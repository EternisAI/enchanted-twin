// Owner: slimane@eternis.ai
package twin_network

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/db"
)

// ThreadState represents the state of a thread.
type ThreadState string

const (
	ThreadStateNone      ThreadState = ""
	ThreadStateIgnored   ThreadState = "ignored"
	ThreadStateCompleted ThreadState = "completed"
)

// ErrThreadNotFound is returned when a thread is not found.
var ErrThreadNotFound = errors.New("thread not found")

// ThreadRecord stores information about a thread state.
type ThreadRecord struct {
	ThreadID    string      `json:"thread_id"`
	State       ThreadState `json:"state"`
	LastUpdated time.Time   `json:"last_updated"`
	ChatID      string      `json:"chat_id"`
}

// ThreadStore manages thread state for the client side.
type ThreadStore struct {
	mu    sync.RWMutex
	store *db.Store
}

// NewThreadStore creates a new thread store.
func NewThreadStore(store *db.Store) *ThreadStore {
	return &ThreadStore{
		store: store,
	}
}

func (ts *ThreadStore) GetThread(ctx context.Context, threadID string) (ThreadRecord, error) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	return ts.getThreadUnlocked(ctx, threadID)
}

func (ts *ThreadStore) getThreadUnlocked(ctx context.Context, threadID string) (ThreadRecord, error) {
	key := fmt.Sprintf("thread_state_%s", threadID)
	value, err := ts.store.GetValue(ctx, key)
	if err != nil {
		return ThreadRecord{}, fmt.Errorf("failed to get thread state: %w", err)
	}

	if value == "" {
		return ThreadRecord{}, fmt.Errorf("thread not found")
	}

	var record ThreadRecord
	if err := json.Unmarshal([]byte(value), &record); err != nil {
		return ThreadRecord{}, fmt.Errorf("failed to unmarshal thread record: %w", err)
	}

	return record, nil
}

func (ts *ThreadStore) SetThreadChatID(ctx context.Context, threadID string, chatID string) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	thread, err := ts.getThreadUnlocked(ctx, threadID)
	if err != nil {
		return fmt.Errorf("failed to get thread: %w", err)
	}
	thread.ChatID = chatID

	recordJSON, err := json.Marshal(thread)
	if err != nil {
		return fmt.Errorf("failed to marshal thread record: %w", err)
	}

	key := fmt.Sprintf("thread_state_%s", threadID)
	return ts.store.SetValue(ctx, key, string(recordJSON))
}

// SetThreadState updates the state of a thread.
func (ts *ThreadStore) InitializeThread(ctx context.Context, threadID string, state ThreadState) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	record := ThreadRecord{
		ThreadID:    threadID,
		State:       state,
		LastUpdated: time.Now(),
		ChatID:      "",
	}

	recordJSON, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal thread record: %w", err)
	}

	key := fmt.Sprintf("thread_state_%s", threadID)
	return ts.store.SetValue(ctx, key, string(recordJSON))
}

func (ts *ThreadStore) SetThreadState(ctx context.Context, threadID string, state ThreadState) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	thread, err := ts.getThreadUnlocked(ctx, threadID)
	if err != nil {
		return fmt.Errorf("failed to get thread: %w", err)
	}
	thread.State = state
	thread.LastUpdated = time.Now()

	recordJSON, err := json.Marshal(thread)
	if err != nil {
		return fmt.Errorf("failed to marshal thread record: %w", err)
	}

	key := fmt.Sprintf("thread_state_%s", threadID)
	return ts.store.SetValue(ctx, key, string(recordJSON))
}

// GetThreadState retrieves the state of a thread.
func (ts *ThreadStore) GetThreadState(ctx context.Context, threadID string) (ThreadState, error) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	return ts.getThreadStateUnlocked(ctx, threadID)
}

func (ts *ThreadStore) getThreadStateUnlocked(ctx context.Context, threadID string) (ThreadState, error) {
	key := fmt.Sprintf("thread_state_%s", threadID)
	value, err := ts.store.GetValue(ctx, key)
	if err != nil {
		return ThreadStateNone, fmt.Errorf("failed to get thread state: %w", err)
	}

	if value == "" {
		return ThreadStateNone, nil
	}

	var record ThreadRecord
	if err := json.Unmarshal([]byte(value), &record); err != nil {
		return ThreadStateNone, fmt.Errorf("failed to unmarshal thread record: %w", err)
	}

	return record.State, nil
}

// GetAllThreadStates retrieves all thread states.
func (ts *ThreadStore) GetAllThreadStates(ctx context.Context) (map[string]ThreadState, error) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	keys, err := ts.store.GetAllKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get all keys: %w", err)
	}

	result := make(map[string]ThreadState)
	for _, key := range keys {
		// Only process keys that start with thread_state_
		if len(key) > 13 && key[:13] == "thread_state_" {
			threadID := key[13:]
			state, err := ts.getThreadStateUnlocked(ctx, threadID)
			if err != nil {
				// Skip errors and continue
				continue
			}
			result[threadID] = state
		}
	}

	return result, nil
}

// DeleteThreadState removes a thread state.
func (ts *ThreadStore) DeleteThreadState(ctx context.Context, threadID string) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	key := fmt.Sprintf("thread_state_%s", threadID)
	return ts.store.SetValue(ctx, key, "")
}

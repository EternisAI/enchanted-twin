package ai

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
)

type ConversationStore interface {
	GetConversationDict(conversationID string) (map[string]string, error)
	SaveConversationDict(conversationID string, dict map[string]string) error
	IsMessageAnonymized(conversationID, messageHash string) (bool, error)
	MarkMessageAnonymized(conversationID, messageHash string) error
	DeleteConversation(conversationID string) error
	ListConversations() ([]string, error)
	Close() error
}

type SQLiteConversationStore struct {
	db     *sql.DB
	logger *log.Logger
}

func NewSQLiteConversationStore(db *sql.DB, logger *log.Logger) *SQLiteConversationStore {
	return &SQLiteConversationStore{
		db:     db,
		logger: logger,
	}
}

func (s *SQLiteConversationStore) GetConversationDict(conversationID string) (map[string]string, error) {
	if conversationID == "" {
		return make(map[string]string), nil
	}

	query := `SELECT dict_data FROM conversation_dicts WHERE conversation_id = ?`

	var dictDataJSON string
	err := s.db.QueryRow(query, conversationID).Scan(&dictDataJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			// No existing dictionary, return empty map
			return make(map[string]string), nil
		}
		// Handle database corruption or other errors gracefully
		s.logger.Error("Failed to get conversation dict, returning empty dict", "conversationID", conversationID, "error", err)
		return make(map[string]string), nil
	}

	var dict map[string]string
	if err := json.Unmarshal([]byte(dictDataJSON), &dict); err != nil {
		s.logger.Error("Failed to unmarshal conversation dict", "conversationID", conversationID, "error", err)
		// Return empty dict if corrupted
		return make(map[string]string), nil
	}

	s.logger.Debug("Retrieved conversation dict", "conversationID", conversationID, "entryCount", len(dict))
	return dict, nil
}

func (s *SQLiteConversationStore) SaveConversationDict(conversationID string, dict map[string]string) error {
	if conversationID == "" {
		// Memory-only mode, no persistence
		return nil
	}

	dictDataJSON, err := json.Marshal(dict)
	if err != nil {
		return fmt.Errorf("failed to marshal conversation dict for %s: %w", conversationID, err)
	}

	query := `
		INSERT INTO conversation_dicts (conversation_id, dict_data, created_at, updated_at) 
		VALUES (?, ?, ?, ?)
		ON CONFLICT(conversation_id) DO UPDATE SET
			dict_data = excluded.dict_data,
			updated_at = excluded.updated_at
	`

	now := time.Now()
	_, err = s.db.Exec(query, conversationID, string(dictDataJSON), now, now)
	if err != nil {
		// Handle database corruption gracefully - log error but don't fail the operation
		s.logger.Error("Failed to save conversation dict, operation will continue", "conversationID", conversationID, "error", err)
		return nil
	}

	s.logger.Debug("Saved conversation dict", "conversationID", conversationID, "entryCount", len(dict))
	return nil
}

func (s *SQLiteConversationStore) IsMessageAnonymized(conversationID, messageHash string) (bool, error) {
	if conversationID == "" {
		// Memory-only mode, messages are never considered pre-anonymized
		return false, nil
	}

	query := `SELECT 1 FROM anonymized_messages WHERE conversation_id = ? AND message_hash = ?`

	var exists int
	err := s.db.QueryRow(query, conversationID, messageHash).Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		// Handle database corruption gracefully - assume message is not anonymized
		s.logger.Error("Failed to check if message is anonymized, assuming not anonymized", "conversationID", conversationID, "messageHash", messageHash, "error", err)
		return false, nil
	}

	return true, nil
}

func (s *SQLiteConversationStore) MarkMessageAnonymized(conversationID, messageHash string) error {
	if conversationID == "" {
		// Memory-only mode, no persistence
		return nil
	}

	query := `
		INSERT INTO anonymized_messages (conversation_id, message_hash, anonymized_at)
		VALUES (?, ?, ?)
		ON CONFLICT(conversation_id, message_hash) DO NOTHING
	`

	_, err := s.db.Exec(query, conversationID, messageHash, time.Now())
	if err != nil {
		// Handle database corruption gracefully - log error but don't fail the operation
		s.logger.Error("Failed to mark message as anonymized, operation will continue", "conversationID", conversationID, "messageHash", messageHash, "error", err)
		return nil
	}

	return nil
}

func (s *SQLiteConversationStore) DeleteConversation(conversationID string) error {
	if conversationID == "" {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Delete anonymized messages first (foreign key constraint)
	_, err = tx.Exec(`DELETE FROM anonymized_messages WHERE conversation_id = ?`, conversationID)
	if err != nil {
		return fmt.Errorf("failed to delete anonymized messages: %w", err)
	}

	// Delete conversation dict
	_, err = tx.Exec(`DELETE FROM conversation_dicts WHERE conversation_id = ?`, conversationID)
	if err != nil {
		return fmt.Errorf("failed to delete conversation dict: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.logger.Debug("Deleted conversation", "conversationID", conversationID)
	return nil
}

func (s *SQLiteConversationStore) ListConversations() ([]string, error) {
	query := `SELECT conversation_id FROM conversation_dicts ORDER BY updated_at DESC`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list conversations: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var conversations []string
	for rows.Next() {
		var conversationID string
		if err := rows.Scan(&conversationID); err != nil {
			return nil, fmt.Errorf("failed to scan conversation ID: %w", err)
		}
		conversations = append(conversations, conversationID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating conversations: %w", err)
	}

	return conversations, nil
}

func (s *SQLiteConversationStore) Close() error {
	// Note: We don't close the DB here as it's managed by the caller
	return nil
}

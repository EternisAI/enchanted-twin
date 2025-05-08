package repository

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/EternisAI/enchanted-twin/graph/model"
)

func (r *Repository) GetChat(ctx context.Context, id string) (model.Chat, error) {
	// Start a read transaction for consistency
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return model.Chat{}, fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Defer a rollback in case anything fails
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	query := "SELECT id, name, created_at FROM chats WHERE id = ?"

	var chatDB ChatDB
	err = tx.GetContext(ctx, &chatDB, query, id)
	if err != nil {
		r.logger.Error("Failed to get chat", "error", err)
		return model.Chat{}, fmt.Errorf("chat not found in the database: %w", err)
	}

	chat := chatDB.ToModel()

	// Get messages without starting a new transaction
	var messages []Message
	err = tx.SelectContext(ctx, &messages, `
		SELECT id, chat_id, text, role, tool_calls, tool_results, image_urls, created_at 
		FROM messages 
		WHERE chat_id = ? 
		ORDER BY created_at ASC
	`, id)
	if err != nil {
		return model.Chat{}, fmt.Errorf("failed to get messages: %w", err)
	}

	modelMessages := make([]*model.Message, len(messages))
	for i, msg := range messages {
		modelMessages[i] = msg.ToModel()
	}
	chat.Messages = modelMessages

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return model.Chat{}, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Set tx to nil so rollback won't be called
	tx = nil

	return chat, nil
}

// GetChatByName retrieves a chat by its name. If multiple chats have the same name,
// returns the most recently created one.
func (r *Repository) GetChatByName(ctx context.Context, name string) (model.Chat, error) {
	// Start a read transaction for consistency
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return model.Chat{}, fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Defer a rollback in case anything fails
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	query := "SELECT id, name, created_at FROM chats WHERE name = ? ORDER BY created_at DESC LIMIT 1"

	var chatDB ChatDB
	err = tx.GetContext(ctx, &chatDB, query, name)
	if err != nil {
		r.logger.Error("Failed to get chat by name", "name", name, "error", err)
		return model.Chat{}, fmt.Errorf("chat with name '%s' not found: %w", name, err)
	}

	chat := chatDB.ToModel()

	// Get messages without starting a new transaction
	var messages []Message
	err = tx.SelectContext(ctx, &messages, `
		SELECT id, chat_id, text, role, tool_calls, tool_results, image_urls, created_at 
		FROM messages 
		WHERE chat_id = ? 
		ORDER BY created_at ASC
	`, chat.ID)
	if err != nil {
		return model.Chat{}, fmt.Errorf("failed to get messages: %w", err)
	}

	modelMessages := make([]*model.Message, len(messages))
	for i, msg := range messages {
		modelMessages[i] = msg.ToModel()
	}
	chat.Messages = modelMessages

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return model.Chat{}, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Set tx to nil so rollback won't be called
	tx = nil

	return chat, nil
}

func (r *Repository) GetChats(ctx context.Context) ([]*model.Chat, error) {
	// Start a read transaction for consistency
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Defer a rollback in case anything fails
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	query := "SELECT id, name, created_at FROM chats ORDER BY created_at DESC"

	var chatDBs []ChatDB
	err = tx.SelectContext(ctx, &chatDBs, query)
	if err != nil {
		r.logger.Error("Failed to get chats", "error", err)
		return nil, fmt.Errorf("failed to get chats: %w", err)
	}
	chats := make([]*model.Chat, len(chatDBs))
	for i, chatDB := range chatDBs {
		chat := chatDB.ToModel()
		chats[i] = &chat
	}

	// Sort chats by createdAt in descending order (newest first)
	sort.Slice(chats, func(i, j int) bool {
		timeI, _ := time.Parse(time.RFC3339, chats[i].CreatedAt)
		timeJ, _ := time.Parse(time.RFC3339, chats[j].CreatedAt)
		return timeI.After(timeJ)
	})

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Set tx to nil so rollback won't be called
	tx = nil

	return chats, nil
}

func (r *Repository) CreateChat(ctx context.Context, name string) (model.Chat, error) {
	// Start a transaction to ensure consistency
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return model.Chat{}, fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Defer a rollback in case anything fails
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	chat := model.Chat{
		ID:        uuid.New().String(),
		Name:      name,
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO chats (id, name, created_at) VALUES (?, ?, ?)
	`, chat.ID, chat.Name, chat.CreatedAt)
	if err != nil {
		return model.Chat{}, fmt.Errorf("failed to create chat: %w", err)
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return model.Chat{}, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Set tx to nil so rollback won't be called
	tx = nil

	return chat, nil
}

func (r *Repository) DeleteChat(ctx context.Context, chatID string) error {
	// Start a transaction to ensure consistency
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Defer a rollback in case anything fails
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	_, err = tx.ExecContext(ctx, `
		DELETE FROM chats WHERE id = ?
	`, chatID)
	if err != nil {
		return fmt.Errorf("failed to delete chat: %w", err)
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Set tx to nil so rollback won't be called
	tx = nil

	return nil
}

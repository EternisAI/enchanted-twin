package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/EternisAI/enchanted-twin/graph/model"
)

func (r *Repository) AddMessageToChat(ctx context.Context, message Message) (string, error) {
	// Start a transaction to ensure consistency
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Defer a rollback in case anything fails
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	// Check if chat exists within the transaction
	var exists bool
	err = tx.GetContext(ctx, &exists, `
		SELECT 1 FROM chats WHERE id = ? LIMIT 1
	`, message.ChatID)
	if err != nil {
		return "", fmt.Errorf("chat not found for new message: %w", err)
	}

	if message.ID == "" {
		message.ID = uuid.New().String()
	}

	if message.CreatedAtStr == "" {
		message.CreatedAtStr = time.Now().Format(time.RFC3339Nano)
	}

	// Insert the message within the transaction
	_, err = tx.ExecContext(ctx, `
		INSERT INTO messages (id, chat_id, text, role, tool_calls, tool_results, image_urls, created_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, message.ID, message.ChatID, message.Text, message.Role,
		message.ToolCallsStr, message.ToolResultsStr, message.ImageURLsStr, message.CreatedAtStr)
	if err != nil {
		return "", fmt.Errorf("failed to add message: %w", err)
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return "", fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Set tx to nil so rollback won't be called
	tx = nil

	return message.ID, nil
}

func (r *Repository) GetMessagesByChatId(
	ctx context.Context,
	chatID string,
) ([]*model.Message, error) {
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

	var exists bool
	err = tx.GetContext(ctx, &exists, `
		SELECT 1 FROM chats WHERE id = ? LIMIT 1
	`, chatID)
	if err != nil {
		return nil, fmt.Errorf("chat not found: %w", err)
	}

	var messages []Message
	err = tx.SelectContext(ctx, &messages, `
		SELECT id, chat_id, text, role, tool_calls, tool_results, image_urls, created_at 
		FROM messages 
		WHERE chat_id = ? 
		ORDER BY created_at ASC
	`, chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}

	result := make([]*model.Message, len(messages))
	for i, msg := range messages {
		result[i] = msg.ToModel()
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Set tx to nil so rollback won't be called
	tx = nil

	return result, nil
}

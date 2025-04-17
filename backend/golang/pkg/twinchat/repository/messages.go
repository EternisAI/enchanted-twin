package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/EternisAI/enchanted-twin/graph/model"
	"github.com/google/uuid"
)

func (r *Repository) AddMessageToChat(ctx context.Context, message Message) (string, error) {
	var exists bool
	err := r.db.GetContext(ctx, &exists, `
		SELECT 1 FROM chats WHERE id = ? LIMIT 1
	`, message.ChatID)
	if err != nil {
		return "", fmt.Errorf("chat not found: %w", err)
	}

	if message.ID == "" {
		message.ID = uuid.New().String()
	}

	message.CreatedAtStr = time.Now().Format(time.RFC3339)

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO messages (id, chat_id, text, role, tool_calls, tool_results, image_urls, created_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, message.ID, message.ChatID, message.Text, message.Role,
		message.ToolCallsStr, message.ToolResultsStr, message.ImageURLsStr, message.CreatedAtStr)
	if err != nil {
		return "", fmt.Errorf("failed to add message: %w", err)
	}

	return message.ID, nil
}

func (r *Repository) GetMessagesByChatId(ctx context.Context, chatID string) ([]*model.Message, error) {
	var exists bool
	err := r.db.GetContext(ctx, &exists, `
		SELECT 1 FROM chats WHERE id = ? LIMIT 1
	`, chatID)
	if err != nil {
		return nil, fmt.Errorf("chat not found: %w", err)
	}

	var messages []Message
	err = r.db.SelectContext(ctx, &messages, `
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

	return result, nil
}

package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/EternisAI/enchanted-twin/graph/model"

	"github.com/google/uuid"
)

func (r *Repository) GetChat(ctx context.Context, id string) (model.Chat, error) {
	query := "SELECT id, name, created_at FROM chats WHERE id = ?"

	var chatDB ChatDB
	err := r.db.GetContext(ctx, &chatDB, query, id)
	if err != nil {
		r.logger.Error("Failed to get chat", "error", err)
		return model.Chat{}, fmt.Errorf("chat not found: %w", err)
	}

	chat := chatDB.ToModel()
	messages, err := r.GetMessagesByChatId(ctx, id)
	if err != nil {
		return model.Chat{}, fmt.Errorf("failed to get messages: %w", err)
	}
	chat.Messages = messages

	return chat, nil
}

func (r *Repository) GetChats(ctx context.Context) ([]*model.Chat, error) {
	query := "SELECT id, name, created_at FROM chats ORDER BY created_at DESC"

	var chatDBs []ChatDB
	err := r.db.SelectContext(ctx, &chatDBs, query)
	if err != nil {
		r.logger.Error("Failed to get chats", "error", err)
		return nil, fmt.Errorf("failed to get chats: %w", err)
	}
	chats := make([]*model.Chat, len(chatDBs))
	for i, chatDB := range chatDBs {
		chat := chatDB.ToModel()
		chats[i] = &chat
	}
	return chats, nil
}

func (r *Repository) CreateChat(ctx context.Context, name string) (model.Chat, error) {
	chat := model.Chat{
		ID:        uuid.New().String(),
		Name:      name,
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO chats (id, name, created_at) VALUES (?, ?, ?)
	`, chat.ID, chat.Name, chat.CreatedAt)
	if err != nil {
		return model.Chat{}, fmt.Errorf("failed to create chat: %w", err)
	}

	return chat, nil
}

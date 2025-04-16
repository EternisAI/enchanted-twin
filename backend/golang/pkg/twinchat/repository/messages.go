package repository

import (
	"context"
	"fmt"

	"github.com/EternisAI/enchanted-twin/graph/model"
)

func (r *Repository) AddMessageToChat(ctx context.Context, message Message) (string, error) {
	chat, ok := r.chats[message.ChatID]
	if !ok {
		return "", fmt.Errorf("chat not found")
	}
	chat.Messages = append(chat.Messages, message.ToModel())
	r.chats[message.ChatID] = chat
	return message.ID, nil
}

func (r *Repository) GetMessagesByChatId(ctx context.Context, chatID string) ([]*model.Message, error) {
	chat, ok := r.chats[chatID]
	if !ok {
		return nil, fmt.Errorf("chat not found")
	}
	return chat.Messages, nil
}

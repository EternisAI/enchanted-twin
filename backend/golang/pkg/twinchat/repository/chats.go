package repository

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/EternisAI/enchanted-twin/graph/model"

	"github.com/google/uuid"
)

func (r *Repository) GetChat(ctx context.Context, id string) (model.Chat, error) {
	chat, ok := r.chats[id]
	if !ok {
		return model.Chat{}, fmt.Errorf("chat not found")
	}
	return chat, nil
}

func (r *Repository) GetChats(ctx context.Context) ([]*model.Chat, error) {
	chats := make([]*model.Chat, 0, len(r.chats))
	for _, chat := range r.chats {
		chats = append(chats, &chat)
	}

	// Sort chats by createdAt in descending order (newest first)
	sort.Slice(chats, func(i, j int) bool {
		timeI, _ := time.Parse(time.RFC3339, chats[i].CreatedAt)
		timeJ, _ := time.Parse(time.RFC3339, chats[j].CreatedAt)
		return timeI.After(timeJ)
	})

	return chats, nil
}

func (r *Repository) CreateChat(ctx context.Context, name string) (model.Chat, error) {
	chat := model.Chat{
		ID:        uuid.New().String(),
		Name:      name,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	r.chats[chat.ID] = chat
	return chat, nil
}

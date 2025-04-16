package repository

import (
	"log/slog"
	"time"

	"github.com/EternisAI/enchanted-twin/graph/model"
	"github.com/EternisAI/enchanted-twin/pkg/helpers"
)

// temporary stub for before database integration
func mockChats() map[string]model.Chat {
	return map[string]model.Chat{
		"1": {
			ID:        "1",
			Name:      "Chat 1",
			CreatedAt: time.Now().Format(time.RFC3339),
			Messages: []*model.Message{
				{ID: "1", Role: model.RoleUser, Text: helpers.Ptr("Hello from Chat 1"), CreatedAt: time.Now().Format(time.RFC3339)},
				{ID: "2", Role: model.RoleAssistant, Text: helpers.Ptr("Another message in Chat 1"), CreatedAt: time.Now().Format(time.RFC3339)},
			},
		},
		"2": {
			ID:        "2",
			Name:      "Chat 2",
			CreatedAt: time.Now().Format(time.RFC3339),
			Messages: []*model.Message{
				{ID: "3", Role: model.RoleUser, Text: helpers.Ptr("Hello from Chat 2"), CreatedAt: time.Now().Format(time.RFC3339)},
				{ID: "4", Role: model.RoleAssistant, Text: helpers.Ptr("Another message in Chat 2"), CreatedAt: time.Now().Format(time.RFC3339)},
			},
		},
	}
}

type Repository struct {
	logger *slog.Logger

	// temporary stub
	chats map[string]model.Chat
}

func NewRepository(logger *slog.Logger) *Repository {
	return &Repository{
		logger: logger,
		chats:  mockChats(),
	}
}

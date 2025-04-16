package twinchat

import (
	"context"

	"github.com/EternisAI/enchanted-twin/graph/model"
	"github.com/EternisAI/enchanted-twin/pkg/twinchat/repository"
)

type Storage interface {
	GetChat(ctx context.Context, id string) (model.Chat, error)
	GetChats(ctx context.Context) ([]*model.Chat, error)
	GetMessagesByChatId(ctx context.Context, chatId string) ([]*model.Message, error)
	AddMessageToChat(ctx context.Context, message repository.Message) (string, error)
	CreateChat(ctx context.Context, name string) (model.Chat, error)
}

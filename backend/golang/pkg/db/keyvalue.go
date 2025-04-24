package db

import (
	"context"

	_ "github.com/mattn/go-sqlite3"
)

type TelegramChat struct {
	ID       int    `db:"id"`
	UUID     string `db:"uuid"`
	ChatID   int    `db:"chat_id"`
	Username string `db:"username"`
}

func (s *Store) GetTelegramChat(ctx context.Context) (*TelegramChat, error) {
	var chat TelegramChat
	err := s.db.GetContext(ctx, &chat, "SELECT * FROM config WHERE key = ?", "telegram_chat_uuid")
	if err != nil {
		return nil, err
	}
	return &chat, nil
}

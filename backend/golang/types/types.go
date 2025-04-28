package types

const (
	// TelegramChatUUIDKey allows to identifies the chat with a specific user, after the first message
	TelegramChatUUIDKey = "telegram_chat_uuid"
	// TelegramLastUpdateIDKey is used to track the last update ID for Telegram messages
	TelegramLastUpdateIDKey = "telegram_last_update_id"
	// TelegramBotName is the telegram bot name to be used for sending messages
	TelegramBotName = "MyTwinSlimBot"
	// TelegramAPIBase is the base url for the telegram api
	TelegramAPIBase = "https://api.telegram.org"
)

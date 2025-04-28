package helpers

import "fmt"

func GetChatURL(botName string, chatUUID string) string {
	return fmt.Sprintf("https://t.me/%s?start=%s", botName, chatUUID)
}

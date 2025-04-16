package x

type DMConversation struct {
	DMConversation struct {
		ConversationID string `json:"conversationId"`
		Messages       []struct {
			MessageCreate struct {
				SenderID    string `json:"senderId"`
				RecipientID string `json:"recipientId"`
				Text        string `json:"text"`
				CreatedAt   string `json:"createdAt"`
			} `json:"messageCreate"`
		} `json:"messages"`
	} `json:"dmConversation"`
}

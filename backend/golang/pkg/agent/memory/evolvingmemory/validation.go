package evolvingmemory

import (
	"encoding/json"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
)

// CreateExampleConversationDocument creates an example ConversationDocument for testing/documentation.
func CreateExampleConversationDocument() *memory.ConversationDocument {
	now := time.Now()
	return &memory.ConversationDocument{
		FieldID:     "example_conversation_001",
		FieldSource: "chat_app",
		People:      []string{"Alice", "Bob"},
		User:        "Alice",
		Conversation: []memory.ConversationMessage{
			{
				Speaker: "Alice",
				Content: "Hey Bob, I just tried this amazing new pizza place downtown!",
				Time:    now.Add(-10 * time.Minute),
			},
			{
				Speaker: "Bob",
				Content: "Oh really? What kind of pizza did you get?",
				Time:    now.Add(-9 * time.Minute),
			},
			{
				Speaker: "Alice",
				Content: "I got their signature margherita with extra basil. It was incredible! I'm definitely going back next week.",
				Time:    now.Add(-8 * time.Minute),
			},
			{
				Speaker: "Bob",
				Content: "That sounds great! I love margherita pizza. What's the name of the place?",
				Time:    now.Add(-7 * time.Minute),
			},
			{
				Speaker: "Alice",
				Content: "It's called 'Nonna's Kitchen' on 5th Street. You should definitely check it out!",
				Time:    now.Add(-6 * time.Minute),
			},
		},
		FieldTags: []string{"food", "recommendations", "casual"},
		FieldMetadata: map[string]string{
			"session_type": "casual_chat",
			"platform":     "messaging_app",
		},
	}
}

// ConversationDocumentToJSON converts a ConversationDocument to JSON string.
func ConversationDocumentToJSON(doc *memory.ConversationDocument) ([]byte, error) {
	return json.MarshalIndent(doc, "", "  ")
}

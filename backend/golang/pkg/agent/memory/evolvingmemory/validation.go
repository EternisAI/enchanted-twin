package evolvingmemory

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
)

// ParseStructuredConversationFromJSON parses a JSON string into a ConversationDocument.
func ParseStructuredConversationFromJSON(jsonData []byte) (*memory.ConversationDocument, error) {
	var doc memory.ConversationDocument
	if err := json.Unmarshal(jsonData, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse structured conversation JSON: %w", err)
	}

	if err := ValidateConversationDocument(&doc); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return &doc, nil
}

// ValidateConversationDocument validates a ConversationDocument for required fields and consistency.
func ValidateConversationDocument(doc *memory.ConversationDocument) error {
	if doc.ID() == "" {
		return fmt.Errorf("document ID is required")
	}

	if doc.FieldSource == "" {
		return fmt.Errorf("conversation source is required")
	}

	if len(doc.People) == 0 {
		return fmt.Errorf("at least one person must be specified in the conversation")
	}

	if doc.User == "" {
		return fmt.Errorf("user field is required to identify the primary user")
	}

	// Validate that the user is in the people list
	userFound := false
	for _, person := range doc.People {
		if person == doc.User {
			userFound = true
			break
		}
	}
	if !userFound {
		return fmt.Errorf("user '%s' must be included in the people list", doc.User)
	}

	if len(doc.Conversation) == 0 {
		return fmt.Errorf("conversation must contain at least one message")
	}

	// Validate each message
	for i, msg := range doc.Conversation {
		if msg.Speaker == "" {
			return fmt.Errorf("message %d: speaker is required", i)
		}
		if msg.Content == "" {
			return fmt.Errorf("message %d: content is required", i)
		}
		if msg.Time.IsZero() {
			return fmt.Errorf("message %d: time is required", i)
		}

		// Validate that the speaker is in the people list
		speakerFound := false
		for _, person := range doc.People {
			if person == msg.Speaker {
				speakerFound = true
				break
			}
		}
		if !speakerFound {
			return fmt.Errorf("message %d: speaker '%s' must be included in the people list", i, msg.Speaker)
		}
	}

	return nil
}

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

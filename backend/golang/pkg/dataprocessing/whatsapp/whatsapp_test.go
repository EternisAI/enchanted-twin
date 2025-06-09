package whatsapp

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
	"github.com/charmbracelet/log"
)

func TestToDocuments(t *testing.T) {
	processor := NewWhatsappProcessor(nil, nil)

	baseTime := time.Date(2025, 6, 8, 15, 59, 3, 0, time.UTC)

	testRecords := []types.Record{
		{
			Data: map[string]interface{}{
				"_pk":         1,
				"chatsession": 32,
				"isfromme":    0,
				"text":        "Cool",
				"fromname":    "Group Chat",
				"toname":      "",
			},
			Timestamp: baseTime,
			Source:    "whatsapp",
		},
		{
			Data: map[string]interface{}{
				"_pk":         2,
				"chatsession": 32,
				"isfromme":    1,
				"text":        "Thanks! Let me know if you need anything else",
				"fromname":    "",
				"toname":      "Group Chat",
			},
			Timestamp: baseTime.Add(5 * time.Minute),
			Source:    "whatsapp",
		},

		{
			Data: map[string]interface{}{
				"_pk":         3,
				"chatsession": 100,
				"isfromme":    1,
				"text":        "no pasa nada, encontre uno !",
				"fromname":    "",
				"toname":      "Contact A",
			},
			Timestamp: baseTime.Add(10 * time.Minute),
			Source:    "whatsapp",
		},
		{
			Data: map[string]interface{}{
				"_pk":         4,
				"chatsession": 100,
				"isfromme":    0,
				"text":        "Perfecto! Me alegro que lo hayas solucionado",
				"fromname":    "Contact A",
				"toname":      "",
			},
			Timestamp: baseTime.Add(15 * time.Minute),
			Source:    "whatsapp",
		},

		{
			Data: map[string]interface{}{
				"_pk":         5,
				"chatsession": 200,
				"isfromme":    1,
				"text":        "https://googleapp.io/",
				"fromname":    "",
				"toname":      "Contact B",
			},
			Timestamp: baseTime.Add(20 * time.Minute),
			Source:    "whatsapp",
		},
		{
			Data: map[string]interface{}{
				"_pk":         6,
				"chatsession": 200,
				"isfromme":    0,
				"text":        "Interesting project! I'll check it out",
				"fromname":    "Contact B",
				"toname":      "",
			},
			Timestamp: baseTime.Add(25 * time.Minute),
			Source:    "whatsapp",
		},
	}

	documents, err := processor.ToDocuments(context.Background(), testRecords)
	require.NoError(t, err)

	assert.Len(t, documents, 3, "Expected 3 conversation documents")

	var conversations []*memory.ConversationDocument
	for _, doc := range documents {
		if convDoc, ok := doc.(*memory.ConversationDocument); ok {
			conversations = append(conversations, convDoc)
		}
	}
	assert.Len(t, conversations, 3, "All documents should be ConversationDocuments")

	var chat32, chat100, chat200 *memory.ConversationDocument
	for _, conv := range conversations {
		switch conv.FieldMetadata["chat_session"] {
		case "32":
			chat32 = conv
		case "100":
			chat100 = conv
		case "200":
			chat200 = conv
		}
	}

	require.NotNil(t, chat32, "Chat session 32 should exist")
	assert.Equal(t, "whatsapp-chat-32", chat32.ID())
	assert.Equal(t, "whatsapp", chat32.Source())
	assert.Equal(t, []string{"whatsapp", "conversation", "chat"}, chat32.Tags())
	assert.Equal(t, "me", chat32.User)
	assert.Len(t, chat32.Conversation, 2, "Chat 32 should have 2 messages")

	assert.Contains(t, chat32.People, "me")
	assert.Contains(t, chat32.People, "Group Chat")

	assert.Equal(t, "Group Chat", chat32.Conversation[0].Speaker)
	assert.Equal(t, "Cool", chat32.Conversation[0].Content)
	assert.Equal(t, "me", chat32.Conversation[1].Speaker)
	assert.Equal(t, "Thanks! Let me know if you need anything else", chat32.Conversation[1].Content)

	expectedContent32 := "Group Chat: Cool\nme: Thanks! Let me know if you need anything else"
	assert.Equal(t, expectedContent32, chat32.Content())

	require.NotNil(t, chat100, "Chat session 100 should exist")
	assert.Equal(t, "whatsapp-chat-100", chat100.ID())
	assert.Len(t, chat100.Conversation, 2, "Chat 100 should have 2 messages")
	assert.Contains(t, chat100.People, "me")
	assert.Contains(t, chat100.People, "Contact A")

	assert.Equal(t, "me", chat100.Conversation[0].Speaker)
	assert.Equal(t, "no pasa nada, encontre uno !", chat100.Conversation[0].Content)
	assert.Equal(t, "Contact A", chat100.Conversation[1].Speaker)
	assert.Equal(t, "Perfecto! Me alegro que lo hayas solucionado", chat100.Conversation[1].Content)

	require.NotNil(t, chat200, "Chat session 200 should exist")
	assert.Equal(t, "whatsapp-chat-200", chat200.ID())
	assert.Len(t, chat200.Conversation, 2, "Chat 200 should have 2 messages")
	assert.Contains(t, chat200.People, "me")
	assert.Contains(t, chat200.People, "Contact B")

	assert.Equal(t, "https://googleapp.io/", chat200.Conversation[0].Content)
	assert.Equal(t, "Interesting project! I'll check it out", chat200.Conversation[1].Content)

	assert.Equal(t, "conversation", chat32.FieldMetadata["type"])
	assert.Equal(t, "32", chat32.FieldMetadata["chat_session"])
}

func TestToDocumentsEdgeCases(t *testing.T) {
	processor := NewWhatsappProcessor(nil, nil)
	baseTime := time.Date(2025, 6, 8, 15, 59, 3, 0, time.UTC)

	t.Run("EmptyTextMessages", func(t *testing.T) {
		records := []types.Record{
			{
				Data: map[string]interface{}{
					"chatsession": 1,
					"isfromme":    1,
					"text":        "",
					"fromname":    "",
					"toname":      "Contact",
				},
				Timestamp: baseTime,
				Source:    "whatsapp",
			},
			{
				Data: map[string]interface{}{
					"chatsession": 1,
					"isfromme":    1,
					"text":        "   ",
					"fromname":    "",
					"toname":      "Contact",
				},
				Timestamp: baseTime,
				Source:    "whatsapp",
			},
		}

		documents, err := processor.ToDocuments(context.Background(), records)
		require.NoError(t, err)
		assert.Empty(t, documents, "Should not create documents for empty text messages")
	})

	t.Run("MissingRequiredFields", func(t *testing.T) {
		records := []types.Record{
			{
				Data: map[string]interface{}{
					"isfromme": 1,
					"text":     "Hello",
				},
				Timestamp: baseTime,
				Source:    "whatsapp",
			},
			{
				Data: map[string]interface{}{
					"chatsession": 1,
					"text":        "Hello",
				},
				Timestamp: baseTime,
				Source:    "whatsapp",
			},
		}

		documents, err := processor.ToDocuments(context.Background(), records)
		require.NoError(t, err)
		assert.Empty(t, documents, "Should not create documents when required fields are missing")
	})

	t.Run("DifferentChatSessionTypes", func(t *testing.T) {
		records := []types.Record{
			{
				Data: map[string]interface{}{
					"chatsession": "123",
					"isfromme":    1,
					"text":        "String session",
					"toname":      "Contact",
				},
				Timestamp: baseTime,
				Source:    "whatsapp",
			},
			{
				Data: map[string]interface{}{
					"chatsession": 456,
					"isfromme":    0,
					"text":        "Int session",
					"fromname":    "Contact",
				},
				Timestamp: baseTime,
				Source:    "whatsapp",
			},
			{
				Data: map[string]interface{}{
					"chatsession": 789.0,
					"isfromme":    1,
					"text":        "Float session",
					"toname":      "Contact",
				},
				Timestamp: baseTime,
				Source:    "whatsapp",
			},
		}

		documents, err := processor.ToDocuments(context.Background(), records)
		require.NoError(t, err)
		assert.Len(t, documents, 3, "Should handle different chatsession types")

		sessionIDs := make(map[string]bool)
		for _, doc := range documents {
			if convDoc, ok := doc.(*memory.ConversationDocument); ok {
				sessionIDs[convDoc.FieldMetadata["chat_session"]] = true
			}
		}

		assert.True(t, sessionIDs["123"], "String session should be preserved")
		assert.True(t, sessionIDs["456"], "Int session should be converted to string")
		assert.True(t, sessionIDs["789"], "Float session should be converted to string")
	})

	t.Run("UnknownSender", func(t *testing.T) {
		records := []types.Record{
			{
				Data: map[string]interface{}{
					"chatsession": 1,
					"isfromme":    0,
					"text":        "Message from unknown sender",
					"fromname":    "",
					"toname":      "",
				},
				Timestamp: baseTime,
				Source:    "whatsapp",
			},
		}

		documents, err := processor.ToDocuments(context.Background(), records)
		require.NoError(t, err)
		assert.Len(t, documents, 1, "Should create document even with unknown sender")

		convDoc, ok := documents[0].(*memory.ConversationDocument)
		require.True(t, ok)
		assert.Equal(t, "unknown", convDoc.Conversation[0].Speaker)
		assert.Contains(t, convDoc.People, "unknown")
	})

	t.Run("SingleMessageConversation", func(t *testing.T) {
		records := []types.Record{
			{
				Data: map[string]interface{}{
					"chatsession": 1,
					"isfromme":    1,
					"text":        "Hello there!",
					"toname":      "Solo Contact",
				},
				Timestamp: baseTime,
				Source:    "whatsapp",
			},
		}

		documents, err := processor.ToDocuments(context.Background(), records)
		require.NoError(t, err)
		assert.Len(t, documents, 1, "Should create document for single message")

		convDoc, ok := documents[0].(*memory.ConversationDocument)
		require.True(t, ok)
		assert.Len(t, convDoc.Conversation, 1, "Should have one message")
		assert.Equal(t, "me", convDoc.Conversation[0].Speaker)
		assert.Equal(t, "Hello there!", convDoc.Conversation[0].Content)
		assert.Contains(t, convDoc.People, "Solo Contact")
	})
}

func TestToDocumentsParticipantDetection(t *testing.T) {
	logger := log.New(os.Stdout)
	processor := NewWhatsappProcessor(nil, logger)
	baseTime := time.Date(2025, 6, 8, 15, 59, 3, 0, time.UTC)

	records := []types.Record{
		{
			Data: map[string]interface{}{
				"chatsession": 1,
				"isfromme":    1,
				"text":        "Hi from me",
				"fromname":    "",
				"toname":      "Alice",
			},
			Timestamp: baseTime,
			Source:    "whatsapp",
		},

		{
			Data: map[string]interface{}{
				"chatsession": 1,
				"isfromme":    0,
				"text":        "Hi back to you",
				"fromname":    "Alice",
				"toname":      "",
			},
			Timestamp: baseTime.Add(time.Minute),
			Source:    "whatsapp",
		},

		{
			Data: map[string]interface{}{
				"chatsession": 1,
				"isfromme":    1,
				"text":        "Hey Bob",
				"fromname":    "",
				"toname":      "Bob",
			},
			Timestamp: baseTime.Add(2 * time.Minute),
			Source:    "whatsapp",
		},

		{
			Data: map[string]interface{}{
				"chatsession": 1,
				"isfromme":    0,
				"text":        "Hey there!",
				"fromname":    "Bob",
				"toname":      "",
			},
			Timestamp: baseTime.Add(3 * time.Minute),
			Source:    "whatsapp",
		},
	}

	documents, err := processor.ToDocuments(context.Background(), records)
	require.NoError(t, err)
	assert.Len(t, documents, 1, "All messages should be grouped in one conversation")

	convDoc, ok := documents[0].(*memory.ConversationDocument)
	require.True(t, ok)

	expectedParticipants := []string{"me", "Alice", "Bob"}
	for _, participant := range expectedParticipants {
		assert.Contains(t, convDoc.People, participant, "Should contain participant: %s", participant)
	}

	expectedSpeakers := []string{"me", "Alice", "me", "Bob"}
	assert.Len(t, convDoc.Conversation, 4, "Should have 4 messages")

	for i, expectedSpeaker := range expectedSpeakers {
		assert.Equal(t, expectedSpeaker, convDoc.Conversation[i].Speaker, "Message %d speaker should be %s", i, expectedSpeaker)
	}

	content := convDoc.Content()
	assert.Contains(t, content, "me: Hi from me")
	assert.Contains(t, content, "Alice: Hi back to you")
	assert.Contains(t, content, "me: Hey Bob")
	assert.Contains(t, content, "Bob: Hey there!")
}

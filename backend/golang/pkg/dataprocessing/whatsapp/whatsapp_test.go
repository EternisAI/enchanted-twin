package whatsapp

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
)

func TestToDocuments(t *testing.T) {
	processor := NewWhatsappProcessor(nil, nil)

	baseTime := time.Date(2025, 6, 8, 15, 59, 3, 0, time.UTC)

	// Test records now contain full conversations
	testRecords := []types.Record{
		{
			Data: map[string]interface{}{
				"id":           "whatsapp-chat-32",
				"source":       "whatsapp",
				"chat_session": "32",
				"people":       []string{"me", "Group Chat"},
				"user":         "me",
				"type":         "conversation",
				"conversation": []map[string]interface{}{
					{
						"speaker": "Group Chat",
						"content": "Cool",
						"time":    baseTime,
					},
					{
						"speaker": "me",
						"content": "Thanks! Let me know if you need anything else",
						"time":    baseTime.Add(5 * time.Minute),
					},
				},
			},
			Timestamp: baseTime,
			Source:    "whatsapp",
		},
		{
			Data: map[string]interface{}{
				"id":           "whatsapp-chat-100",
				"source":       "whatsapp",
				"chat_session": "100",
				"people":       []string{"me", "Contact A"},
				"user":         "me",
				"type":         "conversation",
				"conversation": []map[string]interface{}{
					{
						"speaker": "me",
						"content": "no pasa nada, encontre uno !",
						"time":    baseTime.Add(10 * time.Minute),
					},
					{
						"speaker": "Contact A",
						"content": "Perfecto! Me alegro que lo hayas solucionado",
						"time":    baseTime.Add(15 * time.Minute),
					},
				},
			},
			Timestamp: baseTime.Add(10 * time.Minute),
			Source:    "whatsapp",
		},
		{
			Data: map[string]interface{}{
				"id":           "whatsapp-chat-200",
				"source":       "whatsapp",
				"chat_session": "200",
				"people":       []string{"me", "Contact B"},
				"user":         "me",
				"type":         "conversation",
				"conversation": []map[string]interface{}{
					{
						"speaker": "me",
						"content": "https://googleapp.io/",
						"time":    baseTime.Add(20 * time.Minute),
					},
					{
						"speaker": "Contact B",
						"content": "Interesting project! I'll check it out",
						"time":    baseTime.Add(25 * time.Minute),
					},
				},
			},
			Timestamp: baseTime.Add(20 * time.Minute),
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
	assert.Equal(t, []string{"conversation", "chat"}, chat32.Tags())
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

	require.NotNil(t, chat200, "Chat session 200 should exist")
	assert.Equal(t, "whatsapp-chat-200", chat200.ID())
	assert.Len(t, chat200.Conversation, 2, "Chat 200 should have 2 messages")
	assert.Equal(t, "me", chat200.Conversation[0].Speaker)
	assert.Equal(t, "https://googleapp.io/", chat200.Conversation[0].Content)
}

func TestToDocumentsEdgeCases(t *testing.T) {
	processor := NewWhatsappProcessor(nil, nil)
	baseTime := time.Date(2025, 6, 8, 15, 59, 3, 0, time.UTC)

	t.Run("EmptyConversation", func(t *testing.T) {
		records := []types.Record{
			{
				Data: map[string]interface{}{
					"id":           "whatsapp-chat-1",
					"source":       "whatsapp",
					"chat_session": "1",
					"people":       []string{"me"},
					"user":         "me",
					"type":         "conversation",
					"conversation": []map[string]interface{}{},
				},
				Timestamp: baseTime,
				Source:    "whatsapp",
			},
		}

		documents, err := processor.ToDocuments(context.Background(), records)
		require.NoError(t, err)
		assert.Empty(t, documents, "Should not create documents for empty conversations")
	})

	t.Run("MissingConversationField", func(t *testing.T) {
		records := []types.Record{
			{
				Data: map[string]interface{}{
					"id":           "whatsapp-chat-1",
					"source":       "whatsapp",
					"chat_session": "1",
					"people":       []string{"me"},
					"user":         "me",
					"type":         "conversation",
					// Missing conversation field
				},
				Timestamp: baseTime,
				Source:    "whatsapp",
			},
		}

		documents, err := processor.ToDocuments(context.Background(), records)
		require.NoError(t, err)
		assert.Empty(t, documents, "Should not create documents when conversation field is missing")
	})

	t.Run("EmptyMessages", func(t *testing.T) {
		records := []types.Record{
			{
				Data: map[string]interface{}{
					"id":           "whatsapp-chat-1",
					"source":       "whatsapp",
					"chat_session": "1",
					"people":       []string{"me", "Contact"},
					"user":         "me",
					"type":         "conversation",
					"conversation": []map[string]interface{}{
						{
							"speaker": "",
							"content": "Hello",
							"time":    baseTime,
						},
						{
							"speaker": "me",
							"content": "",
							"time":    baseTime.Add(time.Minute),
						},
					},
				},
				Timestamp: baseTime,
				Source:    "whatsapp",
			},
		}

		documents, err := processor.ToDocuments(context.Background(), records)
		require.NoError(t, err)
		assert.Empty(t, documents, "Should not create documents when all messages are invalid")
	})

	t.Run("SingleValidMessage", func(t *testing.T) {
		records := []types.Record{
			{
				Data: map[string]interface{}{
					"id":           "whatsapp-chat-1",
					"source":       "whatsapp",
					"chat_session": "1",
					"people":       []string{"me", "Solo Contact"},
					"user":         "me",
					"type":         "conversation",
					"conversation": []map[string]interface{}{
						{
							"speaker": "me",
							"content": "Hello there!",
							"time":    baseTime,
						},
					},
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
	// This test is no longer needed as participants are determined during ReadWhatsAppDB
	t.Skip("Participant detection now happens in ReadWhatsAppDB")
}

func TestToDocumentsMessageOrdering(t *testing.T) {
	// This test is no longer needed as message ordering happens in ReadWhatsAppDB
	t.Skip("Message ordering now happens in ReadWhatsAppDB")
}

func TestToDocumentsFullConversation(t *testing.T) {
	// This test is no longer needed as full conversations are created in ReadWhatsAppDB
	t.Skip("Full conversations are now created in ReadWhatsAppDB")
}

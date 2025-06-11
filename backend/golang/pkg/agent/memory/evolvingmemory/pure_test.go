package evolvingmemory

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
)

func TestDocumentChunking(t *testing.T) {
	t.Run("conversation document chunking", func(t *testing.T) {
		doc := &memory.ConversationDocument{
			FieldID:     "conv1",
			FieldSource: "whatsapp",
			User:        "alice",
			People:      []string{"alice", "bob"},
			Conversation: []memory.ConversationMessage{
				{Speaker: "alice", Content: "Hello", Time: time.Now()},
				{Speaker: "bob", Content: "Hi there", Time: time.Now()},
			},
		}

		chunks := doc.Chunk()
		assert.Len(t, chunks, 1) // Small conversation should not be chunked

		// Verify it's the same document
		assert.Equal(t, doc.ID(), chunks[0].ID())
	})

	t.Run("text document chunking", func(t *testing.T) {
		doc := &memory.TextDocument{
			FieldID:      "text1",
			FieldContent: "Some text content",
			FieldSource:  "email",
		}

		chunks := doc.Chunk()
		assert.Len(t, chunks, 1) // Small text should not be chunked

		// Verify it's the same document
		assert.Equal(t, doc.ID(), chunks[0].ID())
	})
}

func TestDistributeWork(t *testing.T) {
	// Create test documents
	docs := make([]memory.Document, 10)
	for i := range docs {
		docs[i] = &memory.TextDocument{
			FieldID:      fmt.Sprintf("doc-%d", i),
			FieldContent: "test content",
		}
	}

	// Test with 3 workers
	chunks := DistributeWork(docs, 3)

	assert.Len(t, chunks, 3)
	assert.Len(t, chunks[0], 4) // 0, 3, 6, 9
	assert.Len(t, chunks[1], 3) // 1, 4, 7
	assert.Len(t, chunks[2], 3) // 2, 5, 8

	// Test with 0 workers (should default to 1)
	chunks = DistributeWork(docs, 0)
	assert.Len(t, chunks, 1)
	assert.Len(t, chunks[0], 10)
}

func TestMarshalMetadata(t *testing.T) {
	metadata := map[string]string{
		"source": "telegram",
	}

	result := marshalMetadata(metadata)

	// Should contain keys
	assert.Contains(t, result, `"source":"telegram"`)
	assert.True(t, result[0] == '{' && result[len(result)-1] == '}')
}

func TestDocumentSizeValidation(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name           string
		doc            memory.Document
		expectedLength int
		shouldTruncate bool
	}{
		{
			name: "small document within limits",
			doc: &memory.TextDocument{
				FieldID:      "small-doc",
				FieldContent: "This is a small document that should not be truncated.",
				FieldSource:  "test",
			},
			expectedLength: 54,
			shouldTruncate: false,
		},
		{
			name: "large_document_exceeding_limits",
			doc: &memory.TextDocument{
				FieldID:      "large-doc",
				FieldContent: strings.Repeat("x", memory.MaxProcessableContentChars+25000), // Exceed limit by 25k
				FieldSource:  "test",
			},
			expectedLength: memory.MaxProcessableContentChars + 25000, // Should be chunked, not truncated
			shouldTruncate: false,                                     // Now chunking instead of truncating
		},
		{
			name: "conversation document exceeding limits",
			doc: &memory.ConversationDocument{
				FieldID:     "large-conversation",
				FieldSource: "test",
				User:        "testuser",
				Conversation: []memory.ConversationMessage{
					{
						Speaker: "user",
						Content: strings.Repeat("x", memory.MaxProcessableContentChars/5), // Each message ~1/5 of limit
						Time:    now,
					},
					{
						Speaker: "user",
						Content: strings.Repeat("y", memory.MaxProcessableContentChars/5),
						Time:    now,
					},
					{
						Speaker: "user",
						Content: strings.Repeat("z", memory.MaxProcessableContentChars/5),
						Time:    now,
					},
					{
						Speaker: "user",
						Content: strings.Repeat("w", memory.MaxProcessableContentChars/5),
						Time:    now,
					},
				},
			},
			expectedLength: 16216, // 4 messages * 4000 chars each + formatting overhead
			shouldTruncate: false, // Should NOT chunk with current sizes
		},
		{
			name: "document at exact limit",
			doc: &memory.TextDocument{
				FieldID:      "exact-limit-doc",
				FieldContent: strings.Repeat("x", memory.MaxProcessableContentChars), // Exactly at the limit
				FieldSource:  "test",
			},
			expectedLength: memory.MaxProcessableContentChars,
			shouldTruncate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test chunking behavior
			chunks := tt.doc.Chunk()

			if len(tt.doc.Content()) > memory.MaxProcessableContentChars {
				// Large documents should be chunked
				assert.True(t, len(chunks) > 1, "Expected large document to be chunked into multiple documents")

				// Verify total content is preserved across chunks
				var combinedContent strings.Builder
				for _, chunk := range chunks {
					combinedContent.WriteString(chunk.Content())
					if _, isConv := chunk.(*memory.ConversationDocument); isConv {
						combinedContent.WriteString("\n") // Conversation chunks have newlines between them
					}
				}

				// For conversation documents, trim the final newline
				fullCombinedContent := combinedContent.String()
				if _, isConv := tt.doc.(*memory.ConversationDocument); isConv {
					fullCombinedContent = strings.TrimSpace(fullCombinedContent)
					assert.Equal(t, tt.doc.Content(), fullCombinedContent, "Combined content of chunks should match original")
				} else {
					// For text documents, verify length
					assert.Equal(t, tt.expectedLength, combinedContent.Len(),
						"Combined content length should be %d, got %d", tt.expectedLength, combinedContent.Len())
				}

				// Each chunk should be the same type as the original
				for _, chunk := range chunks {
					assert.IsType(t, tt.doc, chunk, "Chunk should be same type as original document")
				}
			} else {
				// Small documents should not be chunked
				require.Len(t, chunks, 1)
				actualLength := len(chunks[0].Content())
				assert.Equal(t, tt.expectedLength, actualLength,
					"Content length should be %d, got %d", tt.expectedLength, actualLength)
				assert.Equal(t, tt.doc.Content(), chunks[0].Content(),
					"Small document content should be the same")

				// Verify other properties are preserved for small documents
				assert.Equal(t, tt.doc.ID(), chunks[0].ID())
				assert.Equal(t, tt.doc.Tags(), chunks[0].Tags())
				assert.Equal(t, tt.doc.Metadata(), chunks[0].Metadata())
				assert.Equal(t, tt.doc.Source(), chunks[0].Source())
			}
		})
	}
}

func TestDocumentChunking_Large(t *testing.T) {
	now := time.Now()

	// Create a long conversation document that should be chunked
	longConversation := make([]memory.ConversationMessage, 0)
	// Create messages that will exceed MaxProcessableContentChars when combined
	messageCount := 10
	messageSize := (memory.MaxProcessableContentChars / messageCount) * 2 // Each message is 2x the equal share, ensuring we exceed the limit
	for i := 0; i < messageCount; i++ {
		longConversation = append(longConversation, memory.ConversationMessage{
			Speaker: "alice",
			Content: strings.Repeat("x", messageSize),
			Time:    now.Add(time.Duration(i) * time.Second),
		})
	}

	convDoc := &memory.ConversationDocument{
		FieldID:      "conv-long",
		User:         "alice",
		Conversation: longConversation,
		FieldTags:    []string{"project-x"},
	}

	// Debug: Check content length
	contentLength := len(convDoc.Content())
	t.Logf("Conversation content length: %d (MaxProcessableContentChars: %d)", contentLength, memory.MaxProcessableContentChars)
	t.Logf("Should be chunked: %t", contentLength >= memory.MaxProcessableContentChars)

	// Get chunks
	chunks := convDoc.Chunk()

	t.Logf("Number of chunks: %d", len(chunks))

	// We expect multiple chunks because the total size exceeds ConversationChunkMaxChars
	assert.True(t, len(chunks) > 1, "Expected document to be split into multiple chunks, but got %d", len(chunks))

	// Check the first chunk
	firstChunk, ok := chunks[0].(*memory.ConversationDocument)
	require.True(t, ok, "Expected first chunk to be a ConversationDocument")
	assert.Equal(t, "conv-long-chunk-1", firstChunk.ID())
	assert.Equal(t, []string{"project-x"}, firstChunk.Tags())
	assert.Contains(t, firstChunk.Metadata(), "_enchanted_chunk_number")
	assert.Contains(t, firstChunk.Metadata(), "_enchanted_original_document_id")
	assert.Equal(t, "1", firstChunk.Metadata()["_enchanted_chunk_number"])
	assert.Equal(t, "conv-long", firstChunk.Metadata()["_enchanted_original_document_id"])
	assert.NotEmpty(t, firstChunk.Conversation)

	// Check the last chunk
	lastChunk, ok := chunks[len(chunks)-1].(*memory.ConversationDocument)
	require.True(t, ok)
	assert.Contains(t, lastChunk.Metadata(), "_enchanted_chunk_number")
	expectedChunkNum := fmt.Sprintf("%d", len(chunks))
	assert.Equal(t, expectedChunkNum, lastChunk.Metadata()["_enchanted_chunk_number"])
}

func TestDocumentChunking_NoChunking(t *testing.T) {
	now := time.Now()

	// A short conversation that should not be chunked
	convDoc := &memory.ConversationDocument{
		FieldID: "conv-short",
		User:    "bob",
		Conversation: []memory.ConversationMessage{
			{Speaker: "bob", Content: "A short message.", Time: now},
		},
	}

	chunks := convDoc.Chunk()

	// Should not be chunked
	assert.Len(t, chunks, 1)
	singleDoc, ok := chunks[0].(*memory.ConversationDocument)
	require.True(t, ok)

	// ID should be the original ID since it's not a chunk
	assert.Equal(t, "conv-short", singleDoc.ID())
	assert.NotContains(t, singleDoc.Metadata(), "_enchanted_chunk_number", "Short conversation should not have chunk metadata")
}

func TestSplitOversizedMessage(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name              string
		messageContent    string
		expectedParts     int
		expectPartMarkers bool
	}{
		{
			name:              "normal message not split",
			messageContent:    "This is a normal message",
			expectedParts:     1,
			expectPartMarkers: false,
		},
		{
			name:              "very long message split into parts",
			messageContent:    strings.Repeat("This is a long sentence that will be repeated many times. ", (memory.MaxProcessableContentChars/60)+100), // Exceed limit
			expectedParts:     2,                                                                                                                        // Should be split into at least 2 parts
			expectPartMarkers: true,
		},
		{
			name:              "extremely long message",
			messageContent:    strings.Repeat("X", memory.MaxProcessableContentChars*3), // 3x the limit
			expectedParts:     3,                                                        // Should be split into at least 3 parts
			expectPartMarkers: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := memory.ConversationMessage{
				Speaker: "testuser",
				Content: tt.messageContent,
				Time:    now,
			}

			convDoc := &memory.ConversationDocument{}
			splitMessages := convDoc.SplitOversizedMessage(msg)
			assert.GreaterOrEqual(t, len(splitMessages), tt.expectedParts,
				"Should split into at least %d parts", tt.expectedParts)

			// Verify each split message respects size limits
			for i, splitMsg := range splitMessages {
				msgContent := fmt.Sprintf("%s: %s\n", splitMsg.Speaker, splitMsg.Content)
				assert.LessOrEqual(t, len(msgContent), memory.MaxProcessableContentChars,
					"Split message %d should not exceed MaxProcessableContentChars", i)

				// Verify speaker and timestamp preservation
				assert.Equal(t, msg.Speaker, splitMsg.Speaker)
				assert.Equal(t, msg.Time, splitMsg.Time)

				// Check for part markers if expected
				if tt.expectPartMarkers && len(splitMessages) > 1 {
					assert.Contains(t, splitMsg.Content, "[Part ",
						"Multi-part message should have part markers")
				}
			}

			// Verify content preservation (ignoring markers)
			var reconstructed strings.Builder
			for _, splitMsg := range splitMessages {
				content := splitMsg.Content
				// Remove part markers and continuation indicators for comparison
				content = strings.ReplaceAll(content, " [continued...]", "")
				if strings.Contains(content, "[Part ") {
					// Remove "[Part X] " prefix
					if idx := strings.Index(content, "] "); idx != -1 {
						content = content[idx+2:]
					}
				}
				reconstructed.WriteString(content)
			}

			// The reconstructed content should contain the essential parts of the original
			originalWords := strings.Fields(tt.messageContent)
			reconstructedContent := reconstructed.String()

			if len(originalWords) > 0 {
				// Check that we preserved the beginning and end
				assert.Contains(t, reconstructedContent, originalWords[0],
					"Should preserve beginning of message")
				if len(originalWords) > 1 {
					assert.Contains(t, reconstructedContent, originalWords[len(originalWords)-1],
						"Should preserve end of message")
				}
			}
		})
	}
}

func TestOversizedMessageEdgeCases(t *testing.T) {
	now := time.Now()

	t.Run("message exactly at limit", func(t *testing.T) {
		// Create a message that's exactly at the limit (accounting for speaker prefix and newline)
		speakerPrefix := "user: "
		exactContent := strings.Repeat("x", memory.MaxProcessableContentChars-len(speakerPrefix)-1)

		msg := memory.ConversationMessage{
			Speaker: "user",
			Content: exactContent,
			Time:    now,
		}

		convDoc := &memory.ConversationDocument{}
		splitMessages := convDoc.SplitOversizedMessage(msg)
		assert.Len(t, splitMessages, 1, "Message exactly at limit should not be split")
		assert.Equal(t, exactContent, splitMessages[0].Content)
	})

	t.Run("empty message", func(t *testing.T) {
		msg := memory.ConversationMessage{
			Speaker: "user",
			Content: "",
			Time:    now,
		}

		convDoc := &memory.ConversationDocument{}
		splitMessages := convDoc.SplitOversizedMessage(msg)
		assert.Len(t, splitMessages, 1, "Empty message should result in single message")
		assert.Equal(t, "", splitMessages[0].Content)
	})

	t.Run("message with very long speaker name", func(t *testing.T) {
		longSpeaker := strings.Repeat("verylongspeakername", 100)                  // Very long speaker name
		content := strings.Repeat("content ", memory.MaxProcessableContentChars/8) // Large content that will exceed limit

		msg := memory.ConversationMessage{
			Speaker: longSpeaker,
			Content: content,
			Time:    now,
		}

		convDoc := &memory.ConversationDocument{}
		splitMessages := convDoc.SplitOversizedMessage(msg)

		// Should handle gracefully even with long speaker names
		for i, splitMsg := range splitMessages {
			msgContent := fmt.Sprintf("%s: %s\n", splitMsg.Speaker, splitMsg.Content)
			assert.LessOrEqual(t, len(msgContent), memory.MaxProcessableContentChars+100, // Small tolerance for edge cases
				"Split message %d should roughly respect size limits even with long speaker name", i)
			assert.Equal(t, longSpeaker, splitMsg.Speaker, "Speaker name should be preserved")
		}
	})
}

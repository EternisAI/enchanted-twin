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

func TestPrepareDocuments(t *testing.T) {
	now := time.Now()

	// Create test documents
	convDoc := &memory.ConversationDocument{
		FieldID: "conv-1",
		User:    "alice",
		Conversation: []memory.ConversationMessage{
			{Speaker: "alice", Content: "Hello", Time: now},
		},
	}

	textDoc := &memory.TextDocument{
		FieldID:      "text-1",
		FieldContent: "Some text content",
	}

	docs := []memory.Document{convDoc, textDoc}

	// Test preparation
	prepared, errors := PrepareDocuments(docs, now)

	assert.Empty(t, errors)
	assert.Len(t, prepared, 2)

	// Check conversation document
	assert.Equal(t, DocumentTypeConversation, prepared[0].Type)
	assert.Equal(t, "alice", prepared[0].SpeakerID)
	// With chunking, the original doc is not the same instance, but a new chunk for long docs.
	// For a short doc like this, it should NOT be chunked.
	preparedConv, ok := prepared[0].Original.(*memory.ConversationDocument)
	require.True(t, ok)
	assert.Equal(t, "conv-1", preparedConv.ID()) // It's a short doc, should not be chunked
	assert.Equal(t, "alice", preparedConv.User)
	assert.Equal(t, convDoc.Conversation, preparedConv.Conversation)

	// Check text document
	assert.Equal(t, DocumentTypeText, prepared[1].Type)
	assert.Empty(t, prepared[1].SpeakerID) // Text docs have no speaker
	assert.Equal(t, textDoc, prepared[1].Original)
}

func TestDistributeWork(t *testing.T) {
	// Create test documents
	docs := make([]PreparedDocument, 10)
	for i := range docs {
		docs[i] = PreparedDocument{
			DateString: "test",
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

func TestValidateMemoryOperation(t *testing.T) {
	tests := []struct {
		name    string
		rule    ValidationRule
		wantErr bool
		errMsg  string
	}{
		{
			name: "speaker can update own memory",
			rule: ValidationRule{
				CurrentSpeakerID: "alice",
				IsDocumentLevel:  false,
				TargetSpeakerID:  "alice",
				Action:           UPDATE,
			},
			wantErr: false,
		},
		{
			name: "speaker cannot update other's memory",
			rule: ValidationRule{
				CurrentSpeakerID: "alice",
				IsDocumentLevel:  false,
				TargetSpeakerID:  "bob",
				Action:           UPDATE,
			},
			wantErr: true,
			errMsg:  "speaker alice cannot UPDATE memory belonging to speaker bob",
		},
		{
			name: "document-level cannot update speaker memory",
			rule: ValidationRule{
				CurrentSpeakerID: "",
				IsDocumentLevel:  true,
				TargetSpeakerID:  "alice",
				Action:           DELETE,
			},
			wantErr: true,
			errMsg:  "document-level context cannot DELETE speaker-specific memory",
		},
		{
			name: "document-level can update document-level memory",
			rule: ValidationRule{
				CurrentSpeakerID: "",
				IsDocumentLevel:  true,
				TargetSpeakerID:  "",
				Action:           UPDATE,
			},
			wantErr: false,
		},
		{
			name: "ADD action always allowed",
			rule: ValidationRule{
				CurrentSpeakerID: "alice",
				IsDocumentLevel:  false,
				TargetSpeakerID:  "bob",
				Action:           ADD,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMemoryOperation(tt.rule)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, tt.errMsg, err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMarshalMetadata(t *testing.T) {
	metadata := map[string]string{
		"speakerID": "alice",
		"source":    "telegram",
	}

	result := marshalMetadata(metadata)

	// Should contain both keys
	assert.Contains(t, result, `"speakerID":"alice"`)
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
						Content: strings.Repeat("This is a long message. ", 200), // ~5,800 chars
						Time:    now,
					},
					{
						Speaker: "user",
						Content: strings.Repeat("This is another long message. ", 200),
						Time:    now,
					},
					{
						Speaker: "user",
						Content: strings.Repeat("This is a third long message. ", 200),
						Time:    now,
					},
					{
						Speaker: "user",
						Content: strings.Repeat("This is a fourth long message. ", 200),
						Time:    now,
					},
				},
			},
			expectedLength: 20000, // This is not used for conversations anymore
			shouldTruncate: true,  // This signals it should be chunked
		},
		{
			name: "document at exact limit",
			doc: &memory.TextDocument{
				FieldID:      "exact-limit-doc",
				FieldContent: strings.Repeat("x", 20000), // Exactly 20,000 characters
				FieldSource:  "test",
			},
			expectedLength: 20000,
			shouldTruncate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Because of chunking, we need to handle conversation documents differently now
			if convDoc, isConv := tt.doc.(*memory.ConversationDocument); isConv {
				prepared, err := PrepareDocuments([]memory.Document{convDoc}, now)
				require.NoError(t, err)

				// It should be chunked if it's large enough
				if len(convDoc.Content()) > memory.MaxProcessableContentChars {
					assert.True(t, len(prepared) > 1, "Expected large conversation to be chunked into multiple documents")

					// A large conversation should be chunked, not truncated.
					// The total content of chunks should equal the original content.
					var combinedContent strings.Builder
					for _, p := range prepared {
						combinedContent.WriteString(p.Original.Content())
						combinedContent.WriteString("\n") // The .Content() method joins with newline
					}
					// Trim the final newline
					fullCombinedContent := strings.TrimSpace(combinedContent.String())
					assert.Equal(t, convDoc.Content(), fullCombinedContent, "Combined content of chunks should match original")
				} else {
					assert.Len(t, prepared, 1, "Expected short conversation not to be chunked")
					assert.Equal(t, convDoc.Content(), prepared[0].Original.Content())
				}

				// Each chunk should be a conversation doc
				for _, p := range prepared {
					_, isConvChunk := p.Original.(*memory.ConversationDocument)
					assert.True(t, isConvChunk, "Chunk of a conversation should be a ConversationDocument")
				}
				return // Skip the rest of the test logic for this conversation document
			}

			// Test the clean function for text documents
			prepared, err := PrepareDocuments([]memory.Document{tt.doc}, now)
			require.NoError(t, err)

			if len(tt.doc.Content()) > memory.MaxProcessableContentChars {
				// Large text documents should now be chunked
				assert.True(t, len(prepared) > 1, "Expected large text document to be chunked into multiple documents")

				// Verify total content is preserved across chunks
				var combinedContent strings.Builder
				for _, p := range prepared {
					combinedContent.WriteString(p.Original.Content())
				}
				assert.Equal(t, tt.expectedLength, combinedContent.Len(),
					"Combined content length should be %d, got %d", tt.expectedLength, combinedContent.Len())

				// Each chunk should be a text document
				for _, p := range prepared {
					_, isTextChunk := p.Original.(*memory.TextDocument)
					assert.True(t, isTextChunk, "Chunk of a text document should be a TextDocument")
				}
			} else {
				// Small documents should not be chunked
				require.Len(t, prepared, 1)
				validatedDoc := prepared[0].Original
				actualLength := len(validatedDoc.Content())
				assert.Equal(t, tt.expectedLength, actualLength,
					"Content length should be %d, got %d", tt.expectedLength, actualLength)
				assert.Equal(t, tt.doc.Content(), validatedDoc.Content(),
					"Small document content should be the same")

				// Verify other properties are preserved for small documents
				assert.Equal(t, tt.doc.ID(), validatedDoc.ID())
				assert.Equal(t, tt.doc.Tags(), validatedDoc.Tags())
				assert.Equal(t, tt.doc.Metadata(), validatedDoc.Metadata())
				assert.Equal(t, tt.doc.Source(), validatedDoc.Source())
			}
		})
	}
}

func TestPrepareDocuments_Chunking(t *testing.T) {
	now := time.Now()

	// Create a long conversation document that should be chunked
	longConversation := make([]memory.ConversationMessage, 0)
	// Create a conversation > 50000 chars to force chunking (new limit)
	for i := 0; i < 25; i++ { // Increased from 10 to 25 messages
		longConversation = append(longConversation, memory.ConversationMessage{
			Speaker: "alice",
			Content: strings.Repeat("This is a long test message to ensure chunking happens with the new 50k limit. ", 50), // Approx 4000 chars per message
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

	// Prepare documents
	prepared, errors := PrepareDocuments([]memory.Document{convDoc}, now)
	require.Empty(t, errors)

	t.Logf("Number of prepared documents: %d", len(prepared))

	// We expect multiple chunks because the total size exceeds ConversationChunkMaxChars
	assert.True(t, len(prepared) > 1, "Expected document to be split into multiple chunks, but got %d", len(prepared))

	// Check the first chunk
	firstChunk, ok := prepared[0].Original.(*memory.ConversationDocument)
	require.True(t, ok, "Expected first chunk to be a ConversationDocument")
	assert.Equal(t, "alice", prepared[0].SpeakerID)
	assert.Equal(t, DocumentTypeConversation, prepared[0].Type)
	assert.Equal(t, "conv-long-chunk-1", firstChunk.ID())
	assert.Equal(t, []string{"project-x"}, firstChunk.Tags())
	assert.Contains(t, firstChunk.Metadata(), "chunk_number")
	assert.Contains(t, firstChunk.Metadata(), "original_document_id")
	assert.Equal(t, "1", firstChunk.Metadata()["chunk_number"])
	assert.Equal(t, "conv-long", firstChunk.Metadata()["original_document_id"])
	assert.NotEmpty(t, firstChunk.Conversation)

	// Check the last chunk
	lastChunk, ok := prepared[len(prepared)-1].Original.(*memory.ConversationDocument)
	require.True(t, ok)
	assert.Equal(t, "alice", prepared[len(prepared)-1].SpeakerID)
	assert.Contains(t, lastChunk.Metadata(), "chunk_number")
	expectedChunkNum := fmt.Sprintf("%d", len(prepared))
	assert.Equal(t, expectedChunkNum, lastChunk.Metadata()["chunk_number"])
}

func TestPrepareDocuments_NoChunking(t *testing.T) {
	now := time.Now()

	// A short conversation that should not be chunked
	convDoc := &memory.ConversationDocument{
		FieldID: "conv-short",
		User:    "bob",
		Conversation: []memory.ConversationMessage{
			{Speaker: "bob", Content: "A short message.", Time: now},
		},
	}

	prepared, errors := PrepareDocuments([]memory.Document{convDoc}, now)
	require.Empty(t, errors)

	// Should not be chunked
	assert.Len(t, prepared, 1)
	singleDoc, ok := prepared[0].Original.(*memory.ConversationDocument)
	require.True(t, ok)

	// ID should be the original ID since it's not a chunk
	assert.Equal(t, "conv-short", singleDoc.ID())
	assert.NotContains(t, singleDoc.Metadata(), "chunk_number", "Short conversation should not have chunk metadata")
}

func TestPrepareDocuments_OversizedMessageHandling(t *testing.T) {
	now := time.Now()

	// Create a single message that exceeds MaxProcessableContentChars
	// Calculate how many repetitions we need to exceed the limit
	sampleText := "This is a very long message that will exceed the maximum character limit. "
	repetitionsNeeded := (memory.MaxProcessableContentChars / len(sampleText)) + 100 // Add extra to ensure we exceed
	oversizedContent := strings.Repeat(sampleText, repetitionsNeeded)

	convDoc := &memory.ConversationDocument{
		FieldID: "conv-oversized-msg",
		User:    "alice",
		Conversation: []memory.ConversationMessage{
			{
				Speaker: "alice",
				Content: "Normal message before",
				Time:    now,
			},
			{
				Speaker: "bob",
				Content: oversizedContent, // This exceeds MaxProcessableContentChars
				Time:    now.Add(time.Second),
			},
			{
				Speaker: "alice",
				Content: "Normal message after",
				Time:    now.Add(2 * time.Second),
			},
		},
		FieldTags: []string{"test"},
	}

	// Prepare documents
	prepared, errors := PrepareDocuments([]memory.Document{convDoc}, now)
	require.Empty(t, errors)

	// Should be chunked into multiple parts due to the oversized message
	assert.True(t, len(prepared) > 1, "Expected conversation with oversized message to be chunked")

	// Verify that all content is preserved
	var totalContent strings.Builder
	for _, p := range prepared {
		chunk, ok := p.Original.(*memory.ConversationDocument)
		require.True(t, ok, "Expected chunk to be a ConversationDocument")
		for _, msg := range chunk.Conversation {
			totalContent.WriteString(fmt.Sprintf("%s: %s\n", msg.Speaker, msg.Content))
		}
	}

	// The total should contain all original messages (accounting for [Part X] and [continued...] markers)
	combinedContent := totalContent.String()
	assert.Contains(t, combinedContent, "Normal message before", "Should preserve message before oversized one")
	assert.Contains(t, combinedContent, "Normal message after", "Should preserve message after oversized one")

	// Should contain parts of the oversized content
	assert.Contains(t, combinedContent, "This is a very long message", "Should contain beginning of oversized message")

	// Check for part markers indicating message splitting
	partCount := strings.Count(combinedContent, "[Part ")
	assert.True(t, partCount > 1, "Should have multiple parts for oversized message, got %d parts", partCount)

	// Verify each chunk respects size limits
	for i, p := range prepared {
		chunk, ok := p.Original.(*memory.ConversationDocument)
		require.True(t, ok, "Expected chunk to be a ConversationDocument")
		chunkContent := chunk.Content()
		assert.LessOrEqual(t, len(chunkContent), memory.MaxProcessableContentChars,
			"Chunk %d should not exceed MaxProcessableContentChars. Length: %d", i, len(chunkContent))

		// Each chunk should be a valid ConversationDocument
		assert.Equal(t, DocumentTypeConversation, p.Type)
		assert.Contains(t, chunk.Metadata(), "chunk_number")
		assert.Contains(t, chunk.Metadata(), "original_document_id")
		assert.Equal(t, "conv-oversized-msg", chunk.Metadata()["original_document_id"])
	}
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

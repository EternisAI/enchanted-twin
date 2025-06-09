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
			name: "large document exceeding limits",
			doc: &memory.TextDocument{
				FieldID:      "large-doc",
				FieldContent: strings.Repeat("This is a long document. ", 1000), // ~25,000 characters
				FieldSource:  "test",
			},
			expectedLength: 20000,
			shouldTruncate: true,
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
				if len(convDoc.Content()) > ConversationChunkMaxChars {
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

			// Original logic for non-conversation documents
			validatedDoc := validateAndTruncateDocument(tt.doc)

			// Check that content length is as expected
			actualLength := len(validatedDoc.Content())
			assert.Equal(t, tt.expectedLength, actualLength,
				"Content length should be %d, got %d", tt.expectedLength, actualLength)

			// Check if document was truncated or not
			if tt.shouldTruncate {
				// For truncated documents, verify content is actually truncated
				originalLength := len(tt.doc.Content())
				assert.Greater(t, originalLength, actualLength,
					"Original content (%d chars) should be longer than truncated content (%d chars)",
					originalLength, actualLength)
			} else {
				// For non-truncated documents, verify we have the same instance
				assert.Equal(t, tt.doc.Content(), validatedDoc.Content(),
					"Non-truncated content should be the same")
			}

			// Verify other properties are preserved
			assert.Equal(t, tt.doc.ID(), validatedDoc.ID())
			assert.Equal(t, tt.doc.Tags(), validatedDoc.Tags())
			assert.Equal(t, tt.doc.Metadata(), validatedDoc.Metadata())
			assert.Equal(t, tt.doc.Source(), validatedDoc.Source())
		})
	}
}

func TestPrepareDocuments_Chunking(t *testing.T) {
	now := time.Now()

	// Create a long conversation document that should be chunked
	longConversation := make([]memory.ConversationMessage, 0)
	// Create a conversation > 20000 chars to force chunking
	for i := 0; i < 10; i++ {
		longConversation = append(longConversation, memory.ConversationMessage{
			Speaker: "alice",
			Content: strings.Repeat("This is a long test message to ensure chunking. ", 50), // Approx 2500 chars
			Time:    now.Add(time.Duration(i) * time.Second),
		})
	}

	convDoc := &memory.ConversationDocument{
		FieldID:      "conv-long",
		User:         "alice",
		Conversation: longConversation,
		FieldTags:    []string{"project-x"},
	}

	// Prepare documents
	prepared, errors := PrepareDocuments([]memory.Document{convDoc}, now)
	require.Empty(t, errors)

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

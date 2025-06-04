package evolvingmemory

import (
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
	assert.Equal(t, convDoc, prepared[0].Original)

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
						Content: strings.Repeat("This is a very long message. ", 800), // ~24,000 characters
						Time:    now,
					},
				},
			},
			expectedLength: 20000,
			shouldTruncate: true,
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
			prepared, err := PrepareDocuments([]memory.Document{tt.doc}, now)
			require.NoError(t, err)
			require.Len(t, prepared, 1)

			prepDoc := prepared[0]

			// Check that content length is as expected
			actualLength := len(prepDoc.Original.Content())
			assert.Equal(t, tt.expectedLength, actualLength,
				"Content length should be %d, got %d", tt.expectedLength, actualLength)

			// Check if document was truncated or not
			if tt.shouldTruncate {
				// For truncated documents, verify content is actually truncated
				originalLength := len(tt.doc.Content())
				assert.Greater(t, originalLength, actualLength,
					"Original content (%d chars) should be longer than truncated content (%d chars)",
					originalLength, actualLength)

				// For conversation documents that are truncated, they become TextDocument instances
				if _, isConv := tt.doc.(*memory.ConversationDocument); isConv {
					_, isTextDoc := prepDoc.Original.(*memory.TextDocument)
					assert.True(t, isTextDoc, "Truncated conversation should become TextDocument")
				}
			} else {
				// For non-truncated documents, verify we have the same instance
				assert.Same(t, tt.doc, prepDoc.Original, "Non-truncated document should be the same instance")

				// Verify content matches original
				assert.Equal(t, tt.doc.Content(), prepDoc.Original.Content(),
					"Non-truncated content should match original")
			}

			// Verify other properties are preserved
			assert.Equal(t, tt.doc.ID(), prepDoc.Original.ID())
			assert.Equal(t, tt.doc.Tags(), prepDoc.Original.Tags())
			assert.Equal(t, tt.doc.Metadata(), prepDoc.Original.Metadata())
			assert.Equal(t, tt.doc.Source(), prepDoc.Original.Source())
		})
	}
}

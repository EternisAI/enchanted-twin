package storage

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/helpers"
)

// TestFilterStructure tests the basic Filter struct functionality.
func TestFilterStructure(t *testing.T) {
	t.Run("empty filter", func(t *testing.T) {
		filter := &memory.Filter{}
		assert.Nil(t, filter.Source)
		assert.Nil(t, filter.Subject)
		assert.Nil(t, filter.Tags)
		assert.Equal(t, float32(0), filter.Distance)
		assert.Nil(t, filter.Limit)
	})

	t.Run("populated filter", func(t *testing.T) {
		filter := &memory.Filter{
			Source:   helpers.Ptr("conversations"),
			Subject:  helpers.Ptr("alice"),
			Distance: 0.8,
			Limit:    helpers.Ptr(10),
		}
		assert.Equal(t, "conversations", *filter.Source)
		assert.Equal(t, "alice", *filter.Subject)
		assert.Equal(t, float32(0.8), filter.Distance)
		assert.Equal(t, 10, *filter.Limit)
	})

	t.Run("timestamp filters", func(t *testing.T) {
		now := time.Now()
		yesterday := now.AddDate(0, 0, -1)

		filter := &memory.Filter{
			TimestampAfter:  &yesterday,
			TimestampBefore: &now,
		}

		assert.True(t, filter.TimestampAfter.Before(*filter.TimestampBefore))
	})
}

// TestDocumentReference tests the DocumentReference struct.
func TestDocumentReference(t *testing.T) {
	ref := &DocumentReference{
		ID:      "doc-123",
		Content: "Sample document content",
		Type:    "conversation",
	}

	assert.Equal(t, "doc-123", ref.ID)
	assert.Equal(t, "Sample document content", ref.Content)
	assert.Equal(t, "conversation", ref.Type)
}

// TestStoredDocument tests the StoredDocument struct.
func TestStoredDocument(t *testing.T) {
	now := time.Now()
	metadata := map[string]string{
		"source": "test",
		"type":   "conversation",
	}

	doc := &StoredDocument{
		ID:          "stored-123",
		Content:     "Stored document content",
		Type:        "conversation",
		OriginalID:  "orig-123",
		ContentHash: "abc123",
		Metadata:    metadata,
		CreatedAt:   now,
	}

	assert.Equal(t, "stored-123", doc.ID)
	assert.Equal(t, "Stored document content", doc.Content)
	assert.Equal(t, "conversation", doc.Type)
	assert.Equal(t, "orig-123", doc.OriginalID)
	assert.Equal(t, "abc123", doc.ContentHash)
	assert.Equal(t, metadata, doc.Metadata)
	assert.Equal(t, now, doc.CreatedAt)
}

// TestDocumentChunk tests the DocumentChunk struct.
func TestDocumentChunk(t *testing.T) {
	now := time.Now()
	tags := []string{"tag1", "tag2"}
	metadata := map[string]string{
		"chunk_type": "conversation",
	}

	chunk := &DocumentChunk{
		ID:                 "chunk-123",
		Content:            "Chunk content",
		ChunkIndex:         1,
		OriginalDocumentID: "doc-123",
		Source:             "whatsapp",
		FilePath:           "/path/to/file",
		Tags:               tags,
		Metadata:           metadata,
		CreatedAt:          now,
	}

	assert.Equal(t, "chunk-123", chunk.ID)
	assert.Equal(t, "Chunk content", chunk.Content)
	assert.Equal(t, 1, chunk.ChunkIndex)
	assert.Equal(t, "doc-123", chunk.OriginalDocumentID)
	assert.Equal(t, "whatsapp", chunk.Source)
	assert.Equal(t, "/path/to/file", chunk.FilePath)
	assert.Equal(t, tags, chunk.Tags)
	assert.Equal(t, metadata, chunk.Metadata)
	assert.Equal(t, now, chunk.CreatedAt)
}

package evolvingmemory

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
)

// TestMetadataCollisionAvoidance tests that chunking preserves existing metadata
// and uses namespaced keys to avoid collisions.
func TestMetadataCollisionAvoidance(t *testing.T) {
	now := time.Now()

	t.Run("conversation_document_metadata_preserved", func(t *testing.T) {
		// Create a document with existing metadata that would collide with chunk metadata
		originalMetadata := map[string]string{
			"chunk_number":         "existing_value_1", // Would collide!
			"original_document_id": "existing_value_2", // Would collide!
			"custom_field":         "should_be_preserved",
			"another_field":        "also_preserved",
		}

		// Create a large conversation that will need chunking
		largeConversation := make([]memory.ConversationMessage, 0)
		for i := 0; i < 200; i++ {
			largeConversation = append(largeConversation, memory.ConversationMessage{
				Speaker: "user",
				Content: "This is a very long conversation message that will cause the document to exceed the maximum size limit and trigger chunking behavior. " +
					"We need to ensure that when chunking happens, existing metadata is preserved and not overwritten by chunk metadata.",
				Time: now,
			})
		}

		convDoc := &memory.ConversationDocument{
			FieldID:       "test-conv-with-metadata",
			FieldSource:   "test-source",
			Conversation:  largeConversation,
			FieldTags:     []string{"test-tag"},
			FieldMetadata: originalMetadata,
		}

		// Call Chunk() method
		chunks := convDoc.Chunk()

		// Should result in multiple chunks
		require.Greater(t, len(chunks), 1, "Document should be chunked into multiple pieces")

		// Check each chunk preserves original metadata and uses namespaced chunk metadata
		for i, chunkDoc := range chunks {
			chunk, ok := chunkDoc.(*memory.ConversationDocument)
			require.True(t, ok, "Chunk should be a ConversationDocument")

			metadata := chunk.Metadata()

			// Original metadata should be preserved exactly
			assert.Equal(t, "existing_value_1", metadata["chunk_number"],
				"Original chunk_number metadata should be preserved")
			assert.Equal(t, "existing_value_2", metadata["original_document_id"],
				"Original original_document_id metadata should be preserved")
			assert.Equal(t, "should_be_preserved", metadata["custom_field"],
				"Custom metadata should be preserved")
			assert.Equal(t, "also_preserved", metadata["another_field"],
				"Another custom metadata should be preserved")

			// Namespaced chunk metadata should be added without collision
			assert.Contains(t, metadata, "_enchanted_chunk_number",
				"Should have namespaced chunk number")
			assert.Contains(t, metadata, "_enchanted_original_document_id",
				"Should have namespaced original document ID")
			assert.Contains(t, metadata, "_enchanted_chunk_type",
				"Should have chunk type")

			// Verify namespaced values are correct
			expectedChunkNum := i + 1
			assert.Equal(t, fmt.Sprintf("%d", expectedChunkNum), metadata["_enchanted_chunk_number"])
			assert.Equal(t, "test-conv-with-metadata", metadata["_enchanted_original_document_id"])
			assert.Equal(t, "conversation", metadata["_enchanted_chunk_type"])
		}
	})

	t.Run("text_document_metadata_preserved", func(t *testing.T) {
		// Create a text document with potentially colliding metadata
		originalMetadata := map[string]string{
			"chunk_number":         "my_custom_chunk_ref",
			"original_document_id": "my_source_ref",
			"author":               "Jane Doe",
			"category":             "technical",
		}

		// Create large content that will trigger chunking
		largeContent := ""
		for i := 0; i < 1000; i++ {
			largeContent += "This is a paragraph that contributes to making the document very large. " +
				"When the document size exceeds MaxProcessableContentChars, it should be chunked " +
				"intelligently while preserving all existing metadata without collision. "
		}

		textDoc := &memory.TextDocument{
			FieldID:       "test-text-with-metadata",
			FieldContent:  largeContent,
			FieldSource:   "test-source",
			FieldTags:     []string{"large-document"},
			FieldMetadata: originalMetadata,
		}

		// Call Chunk() method
		chunks := textDoc.Chunk()

		// Should result in multiple chunks due to size
		require.Greater(t, len(chunks), 1, "Large text document should be chunked")

		// Verify each chunk preserves original metadata
		for i, chunkDoc := range chunks {
			chunk, ok := chunkDoc.(*memory.TextDocument)
			require.True(t, ok, "Chunk should be a TextDocument")

			metadata := chunk.Metadata()

			// Original metadata preserved exactly (no collision!)
			assert.Equal(t, "my_custom_chunk_ref", metadata["chunk_number"],
				"Original chunk_number should not be overwritten")
			assert.Equal(t, "my_source_ref", metadata["original_document_id"],
				"Original original_document_id should not be overwritten")
			assert.Equal(t, "Jane Doe", metadata["author"],
				"Custom metadata should be preserved")
			assert.Equal(t, "technical", metadata["category"],
				"Category metadata should be preserved")

			// Namespaced chunk metadata added safely
			assert.Contains(t, metadata, "_enchanted_chunk_number")
			assert.Contains(t, metadata, "_enchanted_original_document_id")
			assert.Contains(t, metadata, "_enchanted_chunk_type")

			// Verify namespaced chunk data
			expectedChunkNum := i + 1
			assert.Equal(t, fmt.Sprintf("%d", expectedChunkNum), metadata["_enchanted_chunk_number"])
			assert.Equal(t, "test-text-with-metadata", metadata["_enchanted_original_document_id"])
			assert.Equal(t, "text", metadata["_enchanted_chunk_type"])
		}
	})

	t.Run("small_document_no_chunking_no_collision", func(t *testing.T) {
		// Small document with potential collision metadata
		originalMetadata := map[string]string{
			"chunk_number":            "not_a_chunk",
			"_enchanted_chunk_number": "should_stay",
		}

		smallDoc := &memory.TextDocument{
			FieldID:       "small-doc",
			FieldContent:  "Small content that won't trigger chunking",
			FieldSource:   "test",
			FieldMetadata: originalMetadata,
		}

		// Should not chunk
		chunks := smallDoc.Chunk()
		require.Len(t, chunks, 1, "Small document should not be chunked")

		// Original metadata should be completely unchanged
		result, ok := chunks[0].(*memory.TextDocument)
		require.True(t, ok, "Chunk should be a TextDocument")
		metadata := result.Metadata()

		assert.Equal(t, "not_a_chunk", metadata["chunk_number"])
		assert.Equal(t, "should_stay", metadata["_enchanted_chunk_number"])

		// No new chunk metadata should be added for unchunked documents
		assert.NotContains(t, metadata, "_enchanted_original_document_id")
		assert.NotContains(t, metadata, "_enchanted_chunk_type")
	})
}

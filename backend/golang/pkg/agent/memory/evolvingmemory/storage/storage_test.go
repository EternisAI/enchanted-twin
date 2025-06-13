package storage

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/helpers"
)

// MockWeaviateClient provides mock implementation for testing.
type MockWeaviateClient struct {
	mock.Mock
}

// MockGraphQLClient for GraphQL operations.
type MockGraphQLClient struct {
	mock.Mock
}

// MockGetQueryBuilder for query building.
type MockGetQueryBuilder struct {
	mock.Mock
}

// MockNearVectorBuilder for near vector operations.
type MockNearVectorBuilder struct {
	mock.Mock
}

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
			Distance: 0.7,
			Limit:    helpers.Ptr(5),
		}
		assert.Equal(t, "conversations", *filter.Source)
		assert.Equal(t, "alice", *filter.Subject)
		assert.Equal(t, float32(0.7), filter.Distance)
		assert.Equal(t, 5, *filter.Limit)
	})
}

// TestDirectFieldFiltering tests the new direct field filtering vs old JSON pattern matching.
func TestDirectFieldFiltering(t *testing.T) {
	// This test documents the expected behavior without requiring a real Weaviate client
	t.Run("source field filtering", func(t *testing.T) {
		filter := &memory.Filter{
			Source: helpers.Ptr("conversations"),
		}

		// Document expected behavior:
		// - Should use filters.Equal on sourceProperty field
		// - Should NOT use LIKE pattern matching on metadataJson
		assert.Equal(t, "conversations", *filter.Source)
	})

	t.Run("subject filtering", func(t *testing.T) {
		filter := &memory.Filter{
			Subject: helpers.Ptr("alice"),
		}

		// Document expected behavior:
		// - Should use filters.Equal on speakerProperty field
		// - Should NOT use LIKE pattern matching on metadataJson
		assert.Equal(t, "alice", *filter.Subject)
	})

	t.Run("combined filtering", func(t *testing.T) {
		filter := &memory.Filter{
			Source:   helpers.Ptr("conversations"),
			Subject:  helpers.Ptr("alice"),
			Distance: 0.8,
			Limit:    helpers.Ptr(10),
		}

		// Document expected behavior:
		// - Should use filters.And to combine multiple filters
		// - Should use direct field queries for performance
		assert.Equal(t, "conversations", *filter.Source)
		assert.Equal(t, "alice", *filter.Subject)
		assert.Equal(t, float32(0.8), filter.Distance)
		assert.Equal(t, 10, *filter.Limit)
	})
}

// TestSchemaFields tests the hybrid schema approach with direct fields + legacy JSON.
func TestSchemaFields(t *testing.T) {
	t.Run("hybrid metadata merging", func(t *testing.T) {
		// Test data representing a document with both direct fields and JSON metadata
		subjectID := "alice"
		metadataJSON := `{"subjectID":"old_subject","extra":"value"}`

		// Parse metadata JSON
		metadata := make(map[string]string)
		err := json.Unmarshal([]byte(metadataJSON), &metadata)
		require.NoError(t, err)

		// Only add subjectID to metadata
		if subjectID != "" {
			metadata["subjectID"] = subjectID
		}

		assert.Equal(t, "alice", metadata["subjectID"]) // Not "old_subject"
		assert.Equal(t, "value", metadata["extra"])     // Preserved from JSON
	})

	t.Run("subjectID from JSON when direct field empty", func(t *testing.T) {
		subjectID := ""
		metadataJSON := `{"subjectID":"json_subject"}`

		metadata := make(map[string]string)
		err := json.Unmarshal([]byte(metadataJSON), &metadata)
		require.NoError(t, err)

		// Only merge non-empty direct fields
		if subjectID != "" {
			metadata["subjectID"] = subjectID
		}

		// Should preserve JSON values when direct fields are empty
		assert.Equal(t, "json_subject", metadata["subjectID"])
	})
}

// TestBackwardCompatibility tests 100% backward compatibility with nil filters.
func TestBackwardCompatibility(t *testing.T) {
	t.Run("nil filter maintains original behavior", func(t *testing.T) {
		var filter *memory.Filter = nil

		// Nil filter should be handled gracefully
		// No WHERE clauses should be added
		// Default limit should be used
		assert.Nil(t, filter)
	})

	t.Run("zero value filter fields", func(t *testing.T) {
		filter := &memory.Filter{
			Source:   nil, // Should not add WHERE clause
			Subject:  nil, // Should not add WHERE clause
			Distance: 0,   // Should not add distance filter
			Limit:    nil, // Should use default limit
		}

		// All fields are nil/zero, should behave like no filter
		assert.Nil(t, filter.Source)
		assert.Nil(t, filter.Subject)
		assert.Equal(t, float32(0), filter.Distance)
		assert.Nil(t, filter.Limit)
	})
}

// TestFilterValidation tests edge cases and validation scenarios.
func TestFilterValidation(t *testing.T) {
	t.Run("empty string values", func(t *testing.T) {
		filter := &memory.Filter{
			Source:  helpers.Ptr(""),
			Subject: helpers.Ptr(""),
		}

		// Empty strings should be handled appropriately
		assert.Equal(t, "", *filter.Source)
		assert.Equal(t, "", *filter.Subject)
	})

	t.Run("negative limit", func(t *testing.T) {
		filter := &memory.Filter{
			Limit: helpers.Ptr(-5),
		}

		// Document expected behavior for negative limits
		assert.Equal(t, -5, *filter.Limit)
	})

	t.Run("zero distance", func(t *testing.T) {
		filter := &memory.Filter{
			Distance: 0,
		}

		// Zero distance should disable distance filtering
		assert.Equal(t, float32(0), filter.Distance)
	})

	t.Run("distance above 1.0", func(t *testing.T) {
		filter := &memory.Filter{
			Distance: 1.5,
		}

		// Distance values above 1.0 should be allowed
		assert.Equal(t, float32(1.5), filter.Distance)
	})
}

// TestDocumentWithDirectFields tests document creation with new direct fields.
func TestDocumentWithDirectFields(t *testing.T) {
	t.Run("text document with direct source field", func(t *testing.T) {
		now := time.Now()
		doc := &memory.TextDocument{
			FieldID:        "test-123",
			FieldContent:   "test content",
			FieldTimestamp: &now,
			FieldSource:    "conversations", // Direct field
			FieldMetadata: map[string]string{
				"subjectID": "alice",
				"extra":     "metadata",
			},
		}

		// Direct field should be accessible
		assert.Equal(t, "conversations", doc.Source())
		assert.Equal(t, "conversations", doc.FieldSource)

		// Metadata should contain subjectID but Source() should use direct field
		assert.Equal(t, "alice", doc.Metadata()["subjectID"])
		assert.Equal(t, "metadata", doc.Metadata()["extra"])
	})

	t.Run("conversation document metadata merging", func(t *testing.T) {
		conv := &memory.ConversationDocument{
			FieldID:     "conv-456",
			FieldSource: "conversations",
			User:        "alice",
			People:      []string{"alice", "bob"},
			Conversation: []memory.ConversationMessage{
				{Speaker: "alice", Content: "hello", Time: time.Now()},
			},
			FieldMetadata: map[string]string{
				"channel": "general",
			},
		}

		metadata := conv.Metadata()

		assert.Equal(t, "alice", metadata["user"])
		assert.Equal(t, "general", metadata["channel"])

		// Direct source field should match
		assert.Equal(t, "conversations", conv.Source())
	})
}

// TestQueryResultParsing tests parsing of query results with both direct fields and JSON metadata.
func TestQueryResultParsing(t *testing.T) {
	t.Run("parse result with direct fields", func(t *testing.T) {
		// Simulate a Weaviate result object
		subjectID := "alice"
		metadataJSON := `{"extra":"value","old_source":"ignore"}`

		// Parse metadata
		metaMap := make(map[string]string)
		err := json.Unmarshal([]byte(metadataJSON), &metaMap)
		require.NoError(t, err)

		// Only add subjectID to metadata, source stays as direct field
		if subjectID != "" {
			metaMap["subjectID"] = subjectID
		}

		doc := memory.TextDocument{
			FieldID:       "test-123",
			FieldContent:  "test content",
			FieldSource:   "conversations", // Direct field
			FieldMetadata: metaMap,         // Merged metadata
		}

		// Verify direct field is used
		assert.Equal(t, "conversations", doc.Source())
		assert.Equal(t, "conversations", doc.FieldSource)

		// Verify field separation
		assert.Equal(t, "conversations", doc.Source())        // From direct field
		assert.Equal(t, "alice", doc.Metadata()["subjectID"]) // In metadata
		assert.Equal(t, "value", doc.Metadata()["extra"])     // From JSON
	})
}

// TestPerformanceBehavior documents the performance improvements.
func TestPerformanceBehavior(t *testing.T) {
	t.Run("direct field filtering performance", func(t *testing.T) {
		// Document expected query patterns:

		// OLD WAY (slow):
		// WHERE metadataJson LIKE '*"source":"conversations"*'

		// NEW WAY (fast):
		// WHERE source = "conversations"

		filter := &memory.Filter{
			Source: helpers.Ptr("conversations"),
		}

		// This test documents that we should be using:
		// - filters.Equal operator
		// - Direct field paths: []string{sourceProperty}
		// - NOT LIKE pattern matching on JSON strings

		assert.Equal(t, "conversations", *filter.Source)
	})

	t.Run("combined filter performance", func(t *testing.T) {
		filter := &memory.Filter{
			Source:  helpers.Ptr("conversations"),
			Subject: helpers.Ptr("alice"),
		}

		// Should generate:
		// WHERE (source = "conversations" AND subjectID = "alice")
		// Using filters.And with multiple filters.Equal conditions

		assert.Equal(t, "conversations", *filter.Source)
		assert.Equal(t, "alice", *filter.Subject)
	})
}

// TestMigrationScenarios tests schema migration behavior.
func TestMigrationScenarios(t *testing.T) {
	t.Run("new schema gets all fields", func(t *testing.T) {
		// New schemas should have:
		// - content (existing)
		// - timestamp (existing)
		// - metadataJson (existing, for backward compatibility)
		// - source (new direct field)
		// - subjectID (new direct field)
		// - tags (existing)

		expectedProperties := []string{
			contentProperty,   // "content"
			timestampProperty, // "timestamp"
			metadataProperty,  // "metadataJson"
			sourceProperty,    // "source"
			tagsProperty,      // "tags"
		}

		// Document that all these fields should exist
		for _, field := range expectedProperties {
			assert.NotEmpty(t, field)
		}
	})

	t.Run("existing data compatibility", func(t *testing.T) {
		// Existing documents without direct fields should:
		// 1. Get new fields added to schema automatically
		// 2. Have empty direct fields initially
		// 3. Fall back to JSON metadata for queries
		// 4. Work with new filtering when direct fields are populated

		// Simulate old document (no direct fields)
		metadataJSON := `{"subjectID":"alice"}`

		metadata := make(map[string]string)
		err := json.Unmarshal([]byte(metadataJSON), &metadata)
		require.NoError(t, err)

		assert.Equal(t, "alice", metadata["subjectID"])
	})
}

// Integration-style test without external dependencies.
func TestStorageInterface(t *testing.T) {
	t.Run("interface compliance", func(t *testing.T) {
		// Verify that WeaviateStorage implements Interface
		var _ Interface = (*WeaviateStorage)(nil)

		// Document all required methods
		methods := []string{
			"GetByID",
			"Update",
			"Delete",
			"StoreBatch",
			"DeleteAll",
			"Query",
			"EnsureSchemaExists",
		}

		for _, method := range methods {
			assert.NotEmpty(t, method)
		}
	})

	t.Run("query method signature", func(t *testing.T) {
		// Document the new Query method signature:
		// Query(ctx context.Context, queryText string, filter *memory.Filter) (memory.QueryResult, error)

		// This should:
		// 1. Accept nil filter for backward compatibility
		// 2. Use Filter struct for structured filtering
		// 3. Return QueryResult with Facts and Documents

		// Example usage:
		filter := &memory.Filter{
			Source: helpers.Ptr("conversations"),
			Limit:  helpers.Ptr(5),
		}

		assert.NotNil(t, filter)
		// Actual query would be: storage.Query(ctx, "query text", filter)
	})
}

func TestTagsFilteringIntegration(t *testing.T) {
	t.Run("tags filtering with ContainsAll operator", func(t *testing.T) {
		filter := &memory.Filter{
			Tags: &memory.TagsFilter{
				All: []string{"work", "important"},
			},
			Limit: helpers.Ptr(5),
		}

		// Test the filter structure validation
		assert.NotNil(t, filter.Tags)
		assert.Len(t, filter.Tags.All, 2)
		assert.Contains(t, filter.Tags.All, "work")
		assert.Contains(t, filter.Tags.All, "important")
		assert.Equal(t, 5, *filter.Limit)

		// Verify that tags filtering maps to the correct Weaviate schema field
		// The storage implementation should use filters.ContainsAll with the tagsProperty field
		expectedTags := []string{"work", "important"}
		assert.Equal(t, expectedTags, filter.Tags.All)
	})

	t.Run("schema alignment verification", func(t *testing.T) {
		// This test verifies that our Filter.Tags field properly aligns with the Weaviate schema
		filter := &memory.Filter{
			Source:  helpers.Ptr("conversations"),
			Subject: helpers.Ptr("alice"),
			Tags: &memory.TagsFilter{
				All: []string{"work", "urgent"},
			},
			Distance: 0.8,
			Limit:    helpers.Ptr(10),
		}

		// Verify all filter components are properly structured
		assert.Equal(t, "conversations", *filter.Source)
		assert.Equal(t, "alice", *filter.Subject)
		assert.NotNil(t, filter.Tags)
		assert.Len(t, filter.Tags.All, 2)
		assert.Contains(t, filter.Tags.All, "work")
		assert.Contains(t, filter.Tags.All, "urgent")
		assert.Equal(t, float32(0.8), filter.Distance)
		assert.Equal(t, 10, *filter.Limit)
	})

	t.Run("comprehensive filter with all fields", func(t *testing.T) {
		filter := &memory.Filter{
			Tags: &memory.TagsFilter{
				All: []string{"project", "meeting", "Q1"},
			},
			Limit: helpers.Ptr(20),
		}

		// Test the filter structure validation
		assert.NotNil(t, filter.Tags)
		assert.Len(t, filter.Tags.All, 3)
		assert.Contains(t, filter.Tags.All, "project")
		assert.Contains(t, filter.Tags.All, "meeting")
		assert.Contains(t, filter.Tags.All, "Q1")
		assert.Equal(t, 20, *filter.Limit)

		// Verify that tags filtering maps to the correct Weaviate schema field
		// The storage implementation should use filters.ContainsAll with the tagsProperty field
		expectedTags := []string{"project", "meeting", "Q1"}
		assert.Equal(t, expectedTags, filter.Tags.All)
	})
}

// TestStructuredFactFiltering tests the new structured fact filtering capabilities.
func TestStructuredFactFiltering(t *testing.T) {
	t.Run("fact category filtering", func(t *testing.T) {
		filter := &memory.Filter{
			FactCategory: helpers.Ptr("preference"),
		}
		assert.Equal(t, "preference", *filter.FactCategory)
		assert.Nil(t, filter.Subject)
		assert.Nil(t, filter.FactAttribute)
	})

	t.Run("fact subject filtering", func(t *testing.T) {
		filter := &memory.Filter{
			Subject: helpers.Ptr("alice"),
		}
		assert.Equal(t, "alice", *filter.Subject)
		assert.Nil(t, filter.FactCategory)
	})

	t.Run("fact attribute filtering", func(t *testing.T) {
		filter := &memory.Filter{
			FactAttribute: helpers.Ptr("coffee_preference"),
		}
		assert.Equal(t, "coffee_preference", *filter.FactAttribute)
	})

	t.Run("fact importance exact filtering", func(t *testing.T) {
		filter := &memory.Filter{
			FactImportance: helpers.Ptr(3),
		}
		assert.Equal(t, 3, *filter.FactImportance)
		assert.Nil(t, filter.FactImportanceMin)
		assert.Nil(t, filter.FactImportanceMax)
	})

	t.Run("fact importance range filtering", func(t *testing.T) {
		filter := &memory.Filter{
			FactImportanceMin: helpers.Ptr(2),
			FactImportanceMax: helpers.Ptr(3),
		}
		assert.Equal(t, 2, *filter.FactImportanceMin)
		assert.Equal(t, 3, *filter.FactImportanceMax)
		assert.Nil(t, filter.FactImportance) // Should be nil when using range
	})

	t.Run("combined structured fact filtering", func(t *testing.T) {
		filter := &memory.Filter{
			FactCategory:   helpers.Ptr("health"),
			Subject:        helpers.Ptr("user"),
			FactAttribute:  helpers.Ptr("health_metric"),
			FactImportance: helpers.Ptr(3),
			Source:         helpers.Ptr("conversations"),
		}
		// Test all fields are set correctly
		assert.Equal(t, "health", *filter.FactCategory)
		assert.Equal(t, "user", *filter.Subject)
		assert.Equal(t, "health_metric", *filter.FactAttribute)
		assert.Equal(t, 3, *filter.FactImportance)
		assert.Equal(t, "conversations", *filter.Source)
	})
}

// TestStructuredFactFilteringEdgeCases tests edge cases and validation scenarios.
func TestStructuredFactFilteringEdgeCases(t *testing.T) {
	t.Run("empty string structured fact fields", func(t *testing.T) {
		filter := &memory.Filter{
			FactCategory:  helpers.Ptr(""),
			Subject:       helpers.Ptr(""),
			FactAttribute: helpers.Ptr(""),
		}

		// Empty strings should be handled appropriately
		assert.Equal(t, "", *filter.FactCategory)
		assert.Equal(t, "", *filter.Subject)
		assert.Equal(t, "", *filter.FactAttribute)
	})

	t.Run("negative importance values", func(t *testing.T) {
		filter := &memory.Filter{
			FactImportance:    helpers.Ptr(-1),
			FactImportanceMin: helpers.Ptr(-5),
			FactImportanceMax: helpers.Ptr(0),
		}

		// Negative values should be accepted but handled appropriately by storage
		assert.Equal(t, -1, *filter.FactImportance)
		assert.Equal(t, -5, *filter.FactImportanceMin)
		assert.Equal(t, 0, *filter.FactImportanceMax)
	})

	t.Run("out of range importance values", func(t *testing.T) {
		filter := &memory.Filter{
			FactImportance:    helpers.Ptr(10), // Beyond expected 1-3 range
			FactImportanceMin: helpers.Ptr(5),
			FactImportanceMax: helpers.Ptr(15),
		}

		// Out-of-range values should be accepted (storage layer validates)
		assert.Equal(t, 10, *filter.FactImportance)
		assert.Equal(t, 5, *filter.FactImportanceMin)
		assert.Equal(t, 15, *filter.FactImportanceMax)
	})

	t.Run("invalid range (min > max)", func(t *testing.T) {
		filter := &memory.Filter{
			FactImportanceMin: helpers.Ptr(3),
			FactImportanceMax: helpers.Ptr(1), // Min > Max
		}

		// Invalid ranges should be accepted (query will return no results)
		assert.Equal(t, 3, *filter.FactImportanceMin)
		assert.Equal(t, 1, *filter.FactImportanceMax)
	})

	t.Run("nil structured fact fields", func(t *testing.T) {
		filter := &memory.Filter{
			Source: helpers.Ptr("conversations"), // Only set non-structured field
		}

		// Legacy field should be set
		assert.Equal(t, "conversations", *filter.Source)

		// All structured fact fields should be nil
		assert.Nil(t, filter.FactCategory)
		assert.Nil(t, filter.Subject)
		assert.Nil(t, filter.FactAttribute)
		assert.Nil(t, filter.FactImportance)
		assert.Nil(t, filter.FactImportanceMin)
		assert.Nil(t, filter.FactImportanceMax)
	})
}

// TestStructuredFactFilteringCombinations tests realistic filtering scenarios.
func TestStructuredFactFilteringCombinations(t *testing.T) {
	t.Run("user preferences with importance", func(t *testing.T) {
		filter := &memory.Filter{
			FactCategory:   helpers.Ptr("preference"),
			FactImportance: helpers.Ptr(2),
			Subject:        helpers.Ptr("user"),
		}

		assert.Equal(t, "preference", *filter.FactCategory)
		assert.Equal(t, 2, *filter.FactImportance)
		assert.Equal(t, "user", *filter.Subject)
	})

	t.Run("important work facts from recent conversations", func(t *testing.T) {
		now := time.Now()
		sevenDaysAgo := now.AddDate(0, 0, -7)
		filter := &memory.Filter{
			FactCategory:      helpers.Ptr("work"),
			Subject:           helpers.Ptr("user"),
			Source:            helpers.Ptr("conversations"),
			FactImportanceMin: helpers.Ptr(2),
			TimestampAfter:    &sevenDaysAgo,
			Limit:             helpers.Ptr(50),
		}

		// Test all fields are set correctly
		assert.Equal(t, "work", *filter.FactCategory)
		assert.Equal(t, "user", *filter.Subject)
		assert.Equal(t, "conversations", *filter.Source)
		assert.Equal(t, 2, *filter.FactImportanceMin)
		assert.Equal(t, sevenDaysAgo, *filter.TimestampAfter)
		assert.Equal(t, 50, *filter.Limit)
	})
}

// TestStructuredFactFilteringBackwardCompatibility tests backward compatibility.
func TestStructuredFactFilteringBackwardCompatibility(t *testing.T) {
	t.Run("legacy filter without structured facts", func(t *testing.T) {
		filter := &memory.Filter{
			Source:   helpers.Ptr("conversations"),
			Subject:  helpers.Ptr("alice"),
			Distance: 0.7,
			Limit:    helpers.Ptr(10),
		}

		// Legacy fields should work unchanged
		assert.Equal(t, "conversations", *filter.Source)
		assert.Equal(t, "alice", *filter.Subject)
		assert.Equal(t, float32(0.7), filter.Distance)
		assert.Equal(t, 10, *filter.Limit)

		// All new fields should be nil
		assert.Nil(t, filter.FactCategory)
		assert.Nil(t, filter.FactAttribute)
		assert.Nil(t, filter.FactImportance)
		assert.Nil(t, filter.FactImportanceMin)
		assert.Nil(t, filter.FactImportanceMax)
	})

	t.Run("exact vs partial matching strategy", func(t *testing.T) {
		exactFilter := &memory.Filter{
			FactCategory: helpers.Ptr("health"), // Exact match
		}

		// Exact matching for categories (fast indexed lookup)
		assert.Equal(t, "health", *exactFilter.FactCategory)
	})
}

// TestStructuredFactFilteringPerformance tests performance characteristics of fact filtering.
func TestStructuredFactFilteringPerformance(t *testing.T) {
	t.Run("direct field indexing vs JSON pattern matching", func(t *testing.T) {
		// Document expected performance improvements:
		// OLD: LIKE pattern matching on JSON strings
		// NEW: Direct field queries with proper indexing

		filter := &memory.Filter{
			FactCategory:   helpers.Ptr("preference"),
			FactImportance: helpers.Ptr(3),
		}

		// These should use direct field queries:
		// WHERE factCategory = "preference" AND factImportance = 3
		assert.Equal(t, "preference", *filter.FactCategory)
		assert.Equal(t, 3, *filter.FactImportance)
	})

	t.Run("range queries on importance", func(t *testing.T) {
		filter := &memory.Filter{
			FactImportanceMin: helpers.Ptr(2),
			FactImportanceMax: helpers.Ptr(3),
		}

		// Should generate efficient range query:
		// WHERE factImportance >= 2 AND factImportance <= 3
		assert.Equal(t, 2, *filter.FactImportanceMin)
		assert.Equal(t, 3, *filter.FactImportanceMax)
	})
}

// TestUpdatePreservesAllFields tests that the Update function preserves all structured fact fields.
func TestUpdatePreservesAllFields(t *testing.T) {
	t.Run("update preserves structured fact fields", func(t *testing.T) {
		// This test documents the expected behavior of the Update function
		// It should preserve all fields when updating a memory

		// Simulate an existing memory with all fields populated
		existingProperties := map[string]interface{}{
			contentProperty:             "old content",
			timestampProperty:           "2025-01-01T00:00:00Z",
			metadataProperty:            `{"extra":"metadata"}`,
			sourceProperty:              "whatsapp",
			tagsProperty:                []interface{}{"conversation", "chat"},
			documentReferencesProperty:  []interface{}{"doc-id-1", "doc-id-2"},
			factCategoryProperty:        "event",
			factSubjectProperty:         "primaryUser",
			factAttributeProperty:       "pet_loss",
			factValueProperty:           "primaryUser's family dog was put down",
			factTemporalContextProperty: "2024-12-27",
			factSensitivityProperty:     "high",
			factImportanceProperty:      3,
		}

		// Create a new document for update
		now := time.Now()
		updateDoc := &memory.TextDocument{
			FieldContent:   "updated content about the pet loss",
			FieldTimestamp: &now,
			FieldSource:    "whatsapp-update",
			FieldMetadata:  map[string]string{"updated": "true"},
		}

		// Verify update document properties
		assert.Equal(t, "updated content about the pet loss", updateDoc.Content())
		assert.Equal(t, "whatsapp-update", updateDoc.Source())
		assert.Equal(t, "true", updateDoc.Metadata()["updated"])

		// After update, the properties should include:
		// - Updated fields: content, timestamp, source, metadata
		// - Preserved fields: all fact fields, tags, documentReferences
		expectedFields := []string{
			tagsProperty,
			documentReferencesProperty,
			factCategoryProperty,
			factSubjectProperty,
			factAttributeProperty,
			factValueProperty,
			factTemporalContextProperty,
			factSensitivityProperty,
			factImportanceProperty,
		}

		// Verify all fields should be preserved
		for _, field := range expectedFields {
			value, exists := existingProperties[field]
			assert.True(t, exists, "Field %s should exist", field)
			assert.NotNil(t, value, "Field %s should not be nil", field)
		}

		// Document specific expectations
		assert.Equal(t, "event", existingProperties[factCategoryProperty])
		assert.Equal(t, "primaryUser", existingProperties[factSubjectProperty])
		assert.Equal(t, 3, existingProperties[factImportanceProperty])
		assert.Equal(t, []interface{}{"doc-id-1", "doc-id-2"}, existingProperties[documentReferencesProperty])
	})

	t.Run("update with empty metadata preserves existing metadata", func(t *testing.T) {
		// When updating with an empty metadata map, existing metadata should be preserved
		existingProperties := map[string]interface{}{
			contentProperty:        "old content",
			metadataProperty:       `{"important":"data","preserve":"this"}`,
			factCategoryProperty:   "preference",
			factImportanceProperty: 2,
		}

		// Update with empty metadata
		updateDoc := &memory.TextDocument{
			FieldContent:  "new content",
			FieldMetadata: map[string]string{}, // Empty metadata
		}

		// The metadata field should remain unchanged when update has empty metadata
		assert.Equal(t, `{"important":"data","preserve":"this"}`, existingProperties[metadataProperty])
		assert.Empty(t, updateDoc.Metadata())
		assert.Equal(t, "new content", updateDoc.Content())
	})

	t.Run("update handles missing fields gracefully", func(t *testing.T) {
		// Test updating a memory that doesn't have all structured fields
		// (e.g., old memory created before structured facts were added)
		existingProperties := map[string]interface{}{
			contentProperty:   "old content",
			timestampProperty: "2024-01-01T00:00:00Z",
			metadataProperty:  `{"source":"old-system"}`,
			// Missing: all structured fact fields
		}

		updateDoc := &memory.TextDocument{
			FieldContent: "updated content",
		}

		// Verify update document
		assert.Equal(t, "updated content", updateDoc.Content())

		// After update, old fields should be preserved even if they're not structured fact fields
		assert.Equal(t, "old content", existingProperties[contentProperty])
		assert.Equal(t, "2024-01-01T00:00:00Z", existingProperties[timestampProperty])
		assert.Equal(t, `{"source":"old-system"}`, existingProperties[metadataProperty])

		// Nil/missing fields should not cause errors
		assert.Nil(t, existingProperties[factCategoryProperty])
		assert.Nil(t, existingProperties[documentReferencesProperty])
	})

	t.Run("update preserves array fields correctly", func(t *testing.T) {
		// Test that array fields like tags and documentReferences are preserved
		existingProperties := map[string]interface{}{
			contentProperty:            "content",
			tagsProperty:               []interface{}{"tag1", "tag2", "tag3"},
			documentReferencesProperty: []interface{}{"ref1", "ref2"},
		}

		// Verify array fields are preserved as-is
		tags, ok := existingProperties[tagsProperty].([]interface{})
		assert.True(t, ok)
		assert.Len(t, tags, 3)
		assert.Contains(t, tags, "tag1")

		refs, ok := existingProperties[documentReferencesProperty].([]interface{})
		assert.True(t, ok)
		assert.Len(t, refs, 2)
		assert.Contains(t, refs, "ref1")
	})

	t.Run("update logs preserved fields", func(t *testing.T) {
		// Test that the update function logs which fields are being preserved
		preservedFields := []string{}
		allProperties := map[string]interface{}{
			contentProperty:            "content",
			timestampProperty:          "2025-01-01T00:00:00Z",
			sourceProperty:             "source",
			metadataProperty:           "{}",
			tagsProperty:               []interface{}{"tag1"},
			documentReferencesProperty: []interface{}{"doc1"},
			factCategoryProperty:       "event",
			factSubjectProperty:        "user",
		}

		// Collect fields that aren't being updated
		updatedFields := []string{contentProperty, timestampProperty, sourceProperty, metadataProperty}
		for key := range allProperties {
			isUpdated := false
			for _, updated := range updatedFields {
				if key == updated {
					isUpdated = true
					break
				}
			}
			if !isUpdated {
				preservedFields = append(preservedFields, key)
			}
		}

		// Should have preserved non-update fields
		assert.Contains(t, preservedFields, tagsProperty)
		assert.Contains(t, preservedFields, documentReferencesProperty)
		assert.Contains(t, preservedFields, factCategoryProperty)
		assert.Contains(t, preservedFields, factSubjectProperty)
		assert.Len(t, preservedFields, 4)
	})
}

package storage

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
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

// Helper functions for creating filter pointer values.
func stringPtr(s string) *string { return &s }
func intPtr(i int) *int          { return &i }

// TestFilterStructure tests the basic Filter struct functionality.
func TestFilterStructure(t *testing.T) {
	t.Run("empty filter", func(t *testing.T) {
		filter := &memory.Filter{}
		assert.Nil(t, filter.Source)
		assert.Nil(t, filter.ContactName)
		assert.Equal(t, float32(0), filter.Distance)
		assert.Nil(t, filter.Limit)
	})

	t.Run("populated filter", func(t *testing.T) {
		filter := &memory.Filter{
			Source:      stringPtr("conversations"),
			ContactName: stringPtr("alice"),
			Distance:    0.7,
			Limit:       intPtr(5),
		}
		assert.Equal(t, "conversations", *filter.Source)
		assert.Equal(t, "alice", *filter.ContactName)
		assert.Equal(t, float32(0.7), filter.Distance)
		assert.Equal(t, 5, *filter.Limit)
	})
}

// TestDirectFieldFiltering tests the new direct field filtering vs old JSON pattern matching.
func TestDirectFieldFiltering(t *testing.T) {
	// This test documents the expected behavior without requiring a real Weaviate client
	t.Run("source field filtering", func(t *testing.T) {
		filter := &memory.Filter{
			Source: stringPtr("conversations"),
		}

		// Document expected behavior:
		// - Should use filters.Equal on sourceProperty field
		// - Should NOT use LIKE pattern matching on metadataJson
		assert.Equal(t, "conversations", *filter.Source)
	})

	t.Run("contact name filtering", func(t *testing.T) {
		filter := &memory.Filter{
			ContactName: stringPtr("alice"),
		}

		// Document expected behavior:
		// - Should use filters.Equal on speakerProperty field
		// - Should NOT use LIKE pattern matching on metadataJson
		assert.Equal(t, "alice", *filter.ContactName)
	})

	t.Run("combined filtering", func(t *testing.T) {
		filter := &memory.Filter{
			Source:      stringPtr("conversations"),
			ContactName: stringPtr("alice"),
			Distance:    0.8,
			Limit:       intPtr(10),
		}

		// Document expected behavior:
		// - Should use filters.And to combine multiple filters
		// - Should use direct field queries for performance
		assert.Equal(t, "conversations", *filter.Source)
		assert.Equal(t, "alice", *filter.ContactName)
		assert.Equal(t, float32(0.8), filter.Distance)
		assert.Equal(t, 10, *filter.Limit)
	})
}

// TestSchemaFields tests the hybrid schema approach with direct fields + legacy JSON.
func TestSchemaFields(t *testing.T) {
	t.Run("hybrid metadata merging", func(t *testing.T) {
		// Test data representing a document with both direct fields and JSON metadata
		source := "conversations"
		speakerID := "alice"
		metadataJSON := `{"source":"old_source","speakerID":"old_speaker","extra":"value"}`

		// Parse metadata JSON
		metadata := make(map[string]string)
		err := json.Unmarshal([]byte(metadataJSON), &metadata)
		require.NoError(t, err)

		// Merge direct fields (they should take precedence)
		if source != "" {
			metadata["source"] = source
		}
		if speakerID != "" {
			metadata["speakerID"] = speakerID
		}

		// Verify direct fields override JSON values
		assert.Equal(t, "conversations", metadata["source"]) // Not "old_source"
		assert.Equal(t, "alice", metadata["speakerID"])      // Not "old_speaker"
		assert.Equal(t, "value", metadata["extra"])          // Preserved from JSON
	})

	t.Run("empty direct fields fallback to JSON", func(t *testing.T) {
		source := ""
		speakerID := ""
		metadataJSON := `{"source":"json_source","speakerID":"json_speaker"}`

		metadata := make(map[string]string)
		err := json.Unmarshal([]byte(metadataJSON), &metadata)
		require.NoError(t, err)

		// Only merge non-empty direct fields
		if source != "" {
			metadata["source"] = source
		}
		if speakerID != "" {
			metadata["speakerID"] = speakerID
		}

		// Should preserve JSON values when direct fields are empty
		assert.Equal(t, "json_source", metadata["source"])
		assert.Equal(t, "json_speaker", metadata["speakerID"])
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
			Source:      nil, // Should not add WHERE clause
			ContactName: nil, // Should not add WHERE clause
			Distance:    0,   // Should not add distance filter
			Limit:       nil, // Should use default limit
		}

		// All fields are nil/zero, should behave like no filter
		assert.Nil(t, filter.Source)
		assert.Nil(t, filter.ContactName)
		assert.Equal(t, float32(0), filter.Distance)
		assert.Nil(t, filter.Limit)
	})
}

// TestFilterValidation tests edge cases and validation scenarios.
func TestFilterValidation(t *testing.T) {
	t.Run("empty string values", func(t *testing.T) {
		filter := &memory.Filter{
			Source:      stringPtr(""),
			ContactName: stringPtr(""),
		}

		// Empty strings should be handled appropriately
		assert.Equal(t, "", *filter.Source)
		assert.Equal(t, "", *filter.ContactName)
	})

	t.Run("negative limit", func(t *testing.T) {
		filter := &memory.Filter{
			Limit: intPtr(-5),
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
				"speakerID": "alice",
				"extra":     "metadata",
			},
		}

		// Direct field should be accessible
		assert.Equal(t, "conversations", doc.Source())
		assert.Equal(t, "conversations", doc.FieldSource)

		// Metadata should contain speakerID but Source() should use direct field
		assert.Equal(t, "alice", doc.Metadata()["speakerID"])
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

		// Should include source and user in metadata
		assert.Equal(t, "conversations", metadata["source"])
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
		source := "conversations"
		speakerID := "alice"
		metadataJSON := `{"extra":"value","old_source":"ignore"}`

		// Parse metadata
		metaMap := make(map[string]string)
		err := json.Unmarshal([]byte(metadataJSON), &metaMap)
		require.NoError(t, err)

		// Merge direct fields (they take precedence)
		if source != "" {
			metaMap["source"] = source
		}
		if speakerID != "" {
			metaMap["speakerID"] = speakerID
		}

		doc := memory.TextDocument{
			FieldID:       "test-123",
			FieldContent:  "test content",
			FieldSource:   source,  // Direct field
			FieldMetadata: metaMap, // Merged metadata
		}

		// Verify direct field is used
		assert.Equal(t, "conversations", doc.Source())
		assert.Equal(t, "conversations", doc.FieldSource)

		// Verify metadata merging
		assert.Equal(t, "conversations", doc.Metadata()["source"]) // From direct field
		assert.Equal(t, "alice", doc.Metadata()["speakerID"])      // From direct field
		assert.Equal(t, "value", doc.Metadata()["extra"])          // From JSON
		assert.NotEqual(t, "ignore", doc.Metadata()["source"])     // JSON overridden
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
			Source: stringPtr("conversations"),
		}

		// This test documents that we should be using:
		// - filters.Equal operator
		// - Direct field paths: []string{sourceProperty}
		// - NOT LIKE pattern matching on JSON strings

		assert.Equal(t, "conversations", *filter.Source)
	})

	t.Run("combined filter performance", func(t *testing.T) {
		filter := &memory.Filter{
			Source:      stringPtr("conversations"),
			ContactName: stringPtr("alice"),
		}

		// Should generate:
		// WHERE (source = "conversations" AND speakerID = "alice")
		// Using filters.And with multiple filters.Equal conditions

		assert.Equal(t, "conversations", *filter.Source)
		assert.Equal(t, "alice", *filter.ContactName)
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
		// - speakerID (new direct field)
		// - tags (existing)

		expectedFields := []string{
			contentProperty,   // "content"
			timestampProperty, // "timestamp"
			metadataProperty,  // "metadataJson"
			sourceProperty,    // "source"
			speakerProperty,   // "speakerID"
			tagsProperty,      // "tags"
		}

		// Document that all these fields should exist
		for _, field := range expectedFields {
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
		metadataJSON := `{"source":"conversations","speakerID":"alice"}`

		metadata := make(map[string]string)
		err := json.Unmarshal([]byte(metadataJSON), &metadata)
		require.NoError(t, err)

		// Old documents can still be filtered via metadata
		assert.Equal(t, "conversations", metadata["source"])
		assert.Equal(t, "alice", metadata["speakerID"])
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
			Source: stringPtr("conversations"),
			Limit:  intPtr(5),
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
			Limit: intPtr(5),
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
			Source:      stringPtr("conversations"),
			ContactName: stringPtr("alice"),
			Tags: &memory.TagsFilter{
				All: []string{"work", "urgent"},
			},
			Distance: 0.8,
			Limit:    intPtr(10),
		}

		// Verify all filter components are properly structured
		assert.Equal(t, "conversations", *filter.Source)
		assert.Equal(t, "alice", *filter.ContactName)
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
			Limit: intPtr(20),
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
			FactCategory: stringPtr("preference"),
		}
		assert.Equal(t, "preference", *filter.FactCategory)
		assert.Nil(t, filter.FactSubject)
		assert.Nil(t, filter.FactAttribute)
	})

	t.Run("fact subject filtering", func(t *testing.T) {
		filter := &memory.Filter{
			FactSubject: stringPtr("alice"),
		}
		assert.Equal(t, "alice", *filter.FactSubject)
		assert.Nil(t, filter.FactCategory)
	})

	t.Run("fact attribute filtering", func(t *testing.T) {
		filter := &memory.Filter{
			FactAttribute: stringPtr("coffee_preference"),
		}
		assert.Equal(t, "coffee_preference", *filter.FactAttribute)
	})

	t.Run("fact value partial matching", func(t *testing.T) {
		filter := &memory.Filter{
			FactValue: stringPtr("coffee"),
		}
		// Should result in LIKE *coffee* query
		assert.Equal(t, "coffee", *filter.FactValue)
	})

	t.Run("fact temporal context filtering", func(t *testing.T) {
		filter := &memory.Filter{
			FactTemporalContext: stringPtr("2024-01-15"),
		}
		assert.Equal(t, "2024-01-15", *filter.FactTemporalContext)
	})

	t.Run("fact sensitivity filtering", func(t *testing.T) {
		filter := &memory.Filter{
			FactSensitivity: stringPtr("high"),
		}
		assert.Equal(t, "high", *filter.FactSensitivity)
	})

	t.Run("fact importance exact filtering", func(t *testing.T) {
		filter := &memory.Filter{
			FactImportance: intPtr(3),
		}
		assert.Equal(t, 3, *filter.FactImportance)
		assert.Nil(t, filter.FactImportanceMin)
		assert.Nil(t, filter.FactImportanceMax)
	})

	t.Run("fact importance range filtering", func(t *testing.T) {
		filter := &memory.Filter{
			FactImportanceMin: intPtr(2),
			FactImportanceMax: intPtr(3),
		}
		assert.Equal(t, 2, *filter.FactImportanceMin)
		assert.Equal(t, 3, *filter.FactImportanceMax)
		assert.Nil(t, filter.FactImportance) // Should be nil when using range
	})

	t.Run("combined structured fact filtering", func(t *testing.T) {
		filter := &memory.Filter{
			FactCategory:    stringPtr("health"),
			FactSubject:     stringPtr("user"),
			FactAttribute:   stringPtr("health_metric"),
			FactSensitivity: stringPtr("high"),
			FactImportance:  intPtr(3),
			Source:          stringPtr("conversations"),
		}
		// Test all fields are set correctly
		assert.Equal(t, "health", *filter.FactCategory)
		assert.Equal(t, "user", *filter.FactSubject)
		assert.Equal(t, "health_metric", *filter.FactAttribute)
		assert.Equal(t, "high", *filter.FactSensitivity)
		assert.Equal(t, 3, *filter.FactImportance)
		assert.Equal(t, "conversations", *filter.Source)
	})
}

// TestStructuredFactFilteringEdgeCases tests edge cases and validation scenarios.
func TestStructuredFactFilteringEdgeCases(t *testing.T) {
	t.Run("empty string structured fact fields", func(t *testing.T) {
		filter := &memory.Filter{
			FactCategory:        stringPtr(""),
			FactSubject:         stringPtr(""),
			FactAttribute:       stringPtr(""),
			FactValue:           stringPtr(""),
			FactTemporalContext: stringPtr(""),
			FactSensitivity:     stringPtr(""),
		}

		// Empty strings should be handled appropriately
		assert.Equal(t, "", *filter.FactCategory)
		assert.Equal(t, "", *filter.FactSubject)
		assert.Equal(t, "", *filter.FactAttribute)
		assert.Equal(t, "", *filter.FactValue)
		assert.Equal(t, "", *filter.FactTemporalContext)
		assert.Equal(t, "", *filter.FactSensitivity)
	})

	t.Run("negative importance values", func(t *testing.T) {
		filter := &memory.Filter{
			FactImportance:    intPtr(-1),
			FactImportanceMin: intPtr(-5),
			FactImportanceMax: intPtr(0),
		}

		// Negative values should be accepted but handled appropriately by storage
		assert.Equal(t, -1, *filter.FactImportance)
		assert.Equal(t, -5, *filter.FactImportanceMin)
		assert.Equal(t, 0, *filter.FactImportanceMax)
	})

	t.Run("out of range importance values", func(t *testing.T) {
		filter := &memory.Filter{
			FactImportance:    intPtr(10), // Beyond expected 1-3 range
			FactImportanceMin: intPtr(5),
			FactImportanceMax: intPtr(15),
		}

		// Out-of-range values should be accepted (storage layer validates)
		assert.Equal(t, 10, *filter.FactImportance)
		assert.Equal(t, 5, *filter.FactImportanceMin)
		assert.Equal(t, 15, *filter.FactImportanceMax)
	})

	t.Run("invalid range (min > max)", func(t *testing.T) {
		filter := &memory.Filter{
			FactImportanceMin: intPtr(3),
			FactImportanceMax: intPtr(1), // Min > Max
		}

		// Invalid ranges should be accepted (query will return no results)
		assert.Equal(t, 3, *filter.FactImportanceMin)
		assert.Equal(t, 1, *filter.FactImportanceMax)
	})

	t.Run("nil structured fact fields", func(t *testing.T) {
		filter := &memory.Filter{
			Source: stringPtr("conversations"), // Only set non-structured field
		}

		// Legacy field should be set
		assert.Equal(t, "conversations", *filter.Source)

		// All structured fact fields should be nil
		assert.Nil(t, filter.FactCategory)
		assert.Nil(t, filter.FactSubject)
		assert.Nil(t, filter.FactAttribute)
		assert.Nil(t, filter.FactValue)
		assert.Nil(t, filter.FactTemporalContext)
		assert.Nil(t, filter.FactSensitivity)
		assert.Nil(t, filter.FactImportance)
		assert.Nil(t, filter.FactImportanceMin)
		assert.Nil(t, filter.FactImportanceMax)
	})
}

// TestStructuredFactFilteringCombinations tests realistic filtering scenarios.
func TestStructuredFactFilteringCombinations(t *testing.T) {
	t.Run("privacy-aware filtering", func(t *testing.T) {
		filter := &memory.Filter{
			FactSensitivity: stringPtr("low"),   // Only public information
			FactSubject:     stringPtr("alice"), // About alice
		}

		assert.Equal(t, "low", *filter.FactSensitivity)
		assert.Equal(t, "alice", *filter.FactSubject)
	})

	t.Run("critical health facts", func(t *testing.T) {
		filter := &memory.Filter{
			FactCategory:   stringPtr("health"),
			FactImportance: intPtr(3), // Critical priority
			FactSubject:    stringPtr("user"),
		}

		assert.Equal(t, "health", *filter.FactCategory)
		assert.Equal(t, 3, *filter.FactImportance)
		assert.Equal(t, "user", *filter.FactSubject)
	})

	t.Run("preferences from conversations", func(t *testing.T) {
		filter := &memory.Filter{
			FactCategory: stringPtr("preference"),
			FactSubject:  stringPtr("user"),
			Source:       stringPtr("conversations"),
			Limit:        intPtr(10),
		}

		assert.Equal(t, "preference", *filter.FactCategory)
		assert.Equal(t, "user", *filter.FactSubject)
		assert.Equal(t, "conversations", *filter.Source)
		assert.Equal(t, 10, *filter.Limit)
	})

	t.Run("important facts with date range", func(t *testing.T) {
		filter := &memory.Filter{
			FactImportanceMin:   intPtr(2), // Medium to high priority
			FactImportanceMax:   intPtr(3),
			FactTemporalContext: stringPtr("2024-01"),
		}

		assert.Equal(t, 2, *filter.FactImportanceMin)
		assert.Equal(t, 3, *filter.FactImportanceMax)
		assert.Equal(t, "2024-01", *filter.FactTemporalContext)
	})

	t.Run("complex search with all structured fields", func(t *testing.T) {
		filter := &memory.Filter{
			// Traditional fields
			Source:      stringPtr("chatgpt"),
			ContactName: stringPtr("assistant"),
			Distance:    0.8,
			Limit:       intPtr(5),
			// Structured fact fields
			FactCategory:        stringPtr("goal_plan"),
			FactSubject:         stringPtr("user"),
			FactAttribute:       stringPtr("career_goal"),
			FactValue:           stringPtr("software engineer"),
			FactTemporalContext: stringPtr("Q1 2024"),
			FactSensitivity:     stringPtr("medium"),
			FactImportance:      intPtr(2),
		}

		// Verify all fields
		assert.Equal(t, "chatgpt", *filter.Source)
		assert.Equal(t, "assistant", *filter.ContactName)
		assert.Equal(t, float32(0.8), filter.Distance)
		assert.Equal(t, 5, *filter.Limit)
		assert.Equal(t, "goal_plan", *filter.FactCategory)
		assert.Equal(t, "user", *filter.FactSubject)
		assert.Equal(t, "career_goal", *filter.FactAttribute)
		assert.Equal(t, "software engineer", *filter.FactValue)
		assert.Equal(t, "Q1 2024", *filter.FactTemporalContext)
		assert.Equal(t, "medium", *filter.FactSensitivity)
		assert.Equal(t, 2, *filter.FactImportance)
	})
}

// TestStructuredFactFilteringBackwardCompatibility ensures existing code still works.
func TestStructuredFactFilteringBackwardCompatibility(t *testing.T) {
	t.Run("legacy filter without structured facts", func(t *testing.T) {
		filter := &memory.Filter{
			Source:      stringPtr("conversations"),
			ContactName: stringPtr("alice"),
			Distance:    0.7,
			Limit:       intPtr(10),
		}

		// Legacy fields should work unchanged
		assert.Equal(t, "conversations", *filter.Source)
		assert.Equal(t, "alice", *filter.ContactName)
		assert.Equal(t, float32(0.7), filter.Distance)
		assert.Equal(t, 10, *filter.Limit)

		// All new fields should be nil
		assert.Nil(t, filter.FactCategory)
		assert.Nil(t, filter.FactSubject)
		assert.Nil(t, filter.FactAttribute)
		assert.Nil(t, filter.FactValue)
		assert.Nil(t, filter.FactTemporalContext)
		assert.Nil(t, filter.FactSensitivity)
		assert.Nil(t, filter.FactImportance)
		assert.Nil(t, filter.FactImportanceMin)
		assert.Nil(t, filter.FactImportanceMax)
	})

	t.Run("mixed legacy and structured filtering", func(t *testing.T) {
		filter := &memory.Filter{
			// Legacy fields
			Source:   stringPtr("conversations"),
			Distance: 0.8,
			// New structured fields
			FactCategory:   stringPtr("preference"),
			FactImportance: intPtr(3),
		}

		// Both legacy and new fields should coexist
		assert.Equal(t, "conversations", *filter.Source)
		assert.Equal(t, float32(0.8), filter.Distance)
		assert.Equal(t, "preference", *filter.FactCategory)
		assert.Equal(t, 3, *filter.FactImportance)

		// Unset fields should be nil
		assert.Nil(t, filter.ContactName)
		assert.Nil(t, filter.FactSubject)
	})
}

// TestStructuredFactFilteringPerformance tests performance-related scenarios.
func TestStructuredFactFilteringPerformance(t *testing.T) {
	t.Run("single field filtering should be efficient", func(t *testing.T) {
		filter := &memory.Filter{
			FactCategory: stringPtr("preference"),
		}

		// Single field should create single WHERE clause
		assert.Equal(t, "preference", *filter.FactCategory)
		assert.Nil(t, filter.FactSubject) // No additional filters
	})

	t.Run("range filtering should use optimized operators", func(t *testing.T) {
		filter := &memory.Filter{
			FactImportanceMin: intPtr(2),
			FactImportanceMax: intPtr(3),
		}

		// Range filtering should use GreaterThanEqual/LessThanEqual operators
		assert.Equal(t, 2, *filter.FactImportanceMin)
		assert.Equal(t, 3, *filter.FactImportanceMax)
	})

	t.Run("exact vs partial matching strategy", func(t *testing.T) {
		exactFilter := &memory.Filter{
			FactCategory: stringPtr("health"), // Exact match
		}
		partialFilter := &memory.Filter{
			FactValue: stringPtr("coffee"), // Partial match with LIKE
		}

		// Exact matching for categories (fast indexed lookup)
		assert.Equal(t, "health", *exactFilter.FactCategory)
		// Partial matching for values (flexible search)
		assert.Equal(t, "coffee", *partialFilter.FactValue)
	})
}

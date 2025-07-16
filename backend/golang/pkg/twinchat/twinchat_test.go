package twinchat

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreatePrivacyDictFromReplacementRules(t *testing.T) {
	tests := []struct {
		name             string
		chatID           string
		replacementRules map[string]string
		wantContains     map[string]string
		wantMetadata     bool
	}{
		{
			name:   "basic replacement rules",
			chatID: "test-chat-123",
			replacementRules: map[string]string{
				"PERSON_001":    "John Smith",
				"PERSON_002":    "Jane Doe",
				"LOCATION_001":  "New York",
				"SENSITIVE_001": "2025-07-13",
			},
			wantContains: map[string]string{
				"John Smith": "PERSON_001",
				"Jane Doe":   "PERSON_002",
				"New York":   "LOCATION_001",
				"2025-07-13": "SENSITIVE_001",
			},
			wantMetadata: true,
		},
		{
			name:             "empty replacement rules",
			chatID:           "test-chat-456",
			replacementRules: map[string]string{},
			wantContains:     map[string]string{},
			wantMetadata:     true,
		},
		{
			name:   "single replacement rule",
			chatID: "test-chat-789",
			replacementRules: map[string]string{
				"PERSON_001": "Alice Johnson",
			},
			wantContains: map[string]string{
				"Alice Johnson": "PERSON_001",
			},
			wantMetadata: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := createPrivacyDictFromReplacementRules(tt.chatID, tt.replacementRules)
			require.NoError(t, err)
			require.NotNil(t, result)

			// Parse the JSON result
			var privacyDict map[string]interface{}
			err = json.Unmarshal([]byte(*result), &privacyDict)
			require.NoError(t, err)

			// Check that all expected mappings are present
			for originalText, expectedToken := range tt.wantContains {
				assert.Equal(t, expectedToken, privacyDict[originalText], "Expected %s to map to %s", originalText, expectedToken)
			}

			// Check metadata if expected
			if tt.wantMetadata {
				metadata, exists := privacyDict["_metadata"]
				assert.True(t, exists, "Expected _metadata to exist")

				metadataMap, ok := metadata.(map[string]interface{})
				assert.True(t, ok, "Expected _metadata to be a map")

				// Check required metadata fields
				assert.Equal(t, tt.chatID, metadataMap["chat_id"])
				assert.Equal(t, "v1", metadataMap["version"])
				assert.Equal(t, "real_anonymization", metadataMap["type"])
				assert.Equal(t, float64(len(tt.replacementRules)), metadataMap["total_rules"])

				// Check that last_updated is a valid timestamp
				lastUpdated, exists := metadataMap["last_updated"]
				assert.True(t, exists, "Expected last_updated to exist")

				// Parse the timestamp to ensure it's valid
				timestampStr, ok := lastUpdated.(string)
				assert.True(t, ok, "Expected last_updated to be a string")

				_, err := time.Parse(time.RFC3339, timestampStr)
				assert.NoError(t, err, "Expected last_updated to be a valid RFC3339 timestamp")
			}

			// Ensure no extra keys beyond replacement rules and metadata
			expectedKeyCount := len(tt.replacementRules)
			if tt.wantMetadata {
				expectedKeyCount++ // +1 for _metadata
			}
			assert.Equal(t, expectedKeyCount, len(privacyDict))
		})
	}
}

func TestCreatePrivacyDictFromReplacementRules_EdgeCases(t *testing.T) {
	t.Run("nil replacement rules", func(t *testing.T) {
		result, err := createPrivacyDictFromReplacementRules("test-chat", nil)
		require.NoError(t, err)
		require.NotNil(t, result)

		var privacyDict map[string]interface{}
		err = json.Unmarshal([]byte(*result), &privacyDict)
		require.NoError(t, err)

		// Should only contain metadata
		assert.Equal(t, 1, len(privacyDict))
		assert.Contains(t, privacyDict, "_metadata")
	})

	t.Run("special characters in replacement rules", func(t *testing.T) {
		replacementRules := map[string]string{
			"PERSON_001": "José María García-López",
			"EMAIL_001":  "test@example.com",
			"PHONE_001":  "+1-555-123-4567",
		}

		result, err := createPrivacyDictFromReplacementRules("test-chat", replacementRules)
		require.NoError(t, err)
		require.NotNil(t, result)

		var privacyDict map[string]interface{}
		err = json.Unmarshal([]byte(*result), &privacyDict)
		require.NoError(t, err)

		assert.Equal(t, "PERSON_001", privacyDict["José María García-López"])
		assert.Equal(t, "EMAIL_001", privacyDict["test@example.com"])
		assert.Equal(t, "PHONE_001", privacyDict["+1-555-123-4567"])
	})

	t.Run("empty chat ID", func(t *testing.T) {
		replacementRules := map[string]string{
			"PERSON_001": "John Doe",
		}

		result, err := createPrivacyDictFromReplacementRules("", replacementRules)
		require.NoError(t, err)
		require.NotNil(t, result)

		var privacyDict map[string]interface{}
		err = json.Unmarshal([]byte(*result), &privacyDict)
		require.NoError(t, err)

		metadata, ok := privacyDict["_metadata"].(map[string]interface{})
		assert.True(t, ok, "Expected _metadata to be a map")
		assert.Equal(t, "", metadata["chat_id"])
	})
}

func TestCreatePrivacyDictFromReplacementRules_JSONStructure(t *testing.T) {
	replacementRules := map[string]string{
		"PERSON_001":    "Alice Smith",
		"LOCATION_001":  "San Francisco",
		"SENSITIVE_001": "2025-07-13T10:30:00Z",
	}

	result, err := createPrivacyDictFromReplacementRules("test-chat-json", replacementRules)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify it's valid JSON
	var privacyDict map[string]interface{}
	err = json.Unmarshal([]byte(*result), &privacyDict)
	require.NoError(t, err)

	// Verify structure matches expected format
	expectedStructure := map[string]interface{}{
		"Alice Smith":          "PERSON_001",
		"San Francisco":        "LOCATION_001",
		"2025-07-13T10:30:00Z": "SENSITIVE_001",
		"_metadata": map[string]interface{}{
			"chat_id":      "test-chat-json",
			"total_rules":  float64(3),
			"version":      "v1",
			"type":         "real_anonymization",
			"last_updated": "2025-07-13T19:00:00Z", // This will be different, we'll check separately
		},
	}

	// Check all non-metadata fields
	for key, expectedValue := range expectedStructure {
		if key != "_metadata" {
			assert.Equal(t, expectedValue, privacyDict[key])
		}
	}

	// Check metadata structure
	metadata, exists := privacyDict["_metadata"]
	assert.True(t, exists)
	metadataMap, ok := metadata.(map[string]interface{})
	assert.True(t, ok, "Expected _metadata to be a map")

	assert.Equal(t, "test-chat-json", metadataMap["chat_id"])
	assert.Equal(t, float64(3), metadataMap["total_rules"])
	assert.Equal(t, "v1", metadataMap["version"])
	assert.Equal(t, "real_anonymization", metadataMap["type"])
	assert.Contains(t, metadataMap, "last_updated")
}

func TestCreatePrivacyDictFromReplacementRules_ThreadSafety(t *testing.T) {
	// Test concurrent access to the function
	replacementRules := map[string]string{
		"PERSON_001": "John Doe",
		"PERSON_002": "Jane Smith",
	}

	// Run multiple goroutines concurrently
	const numGoroutines = 10
	results := make(chan *string, numGoroutines)
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			result, err := createPrivacyDictFromReplacementRules("test-chat", replacementRules)
			if err != nil {
				errors <- err
				return
			}
			results <- result
		}(i)
	}

	// Collect results
	for i := 0; i < numGoroutines; i++ {
		select {
		case result := <-results:
			require.NotNil(t, result)

			var privacyDict map[string]interface{}
			err := json.Unmarshal([]byte(*result), &privacyDict)
			require.NoError(t, err)

			assert.Equal(t, "PERSON_001", privacyDict["John Doe"])
			assert.Equal(t, "PERSON_002", privacyDict["Jane Smith"])

		case err := <-errors:
			t.Fatalf("Unexpected error: %v", err)
		}
	}
}

func TestCreatePrivacyDictFromReplacementRules_LargeDataset(t *testing.T) {
	// Test with a large number of replacement rules
	replacementRules := make(map[string]string)

	// Generate 1000 replacement rules
	for i := 0; i < 1000; i++ {
		token := fmt.Sprintf("PERSON_%03d", i)
		name := fmt.Sprintf("User %d", i)
		replacementRules[token] = name
	}

	result, err := createPrivacyDictFromReplacementRules("test-chat-large", replacementRules)
	require.NoError(t, err)
	require.NotNil(t, result)

	var privacyDict map[string]interface{}
	err = json.Unmarshal([]byte(*result), &privacyDict)
	require.NoError(t, err)

	// Should have 1000 mappings + 1 metadata
	assert.Equal(t, 1001, len(privacyDict))

	// Check a few random mappings
	assert.Equal(t, "PERSON_000", privacyDict["User 0"])
	assert.Equal(t, "PERSON_500", privacyDict["User 500"])
	assert.Equal(t, "PERSON_999", privacyDict["User 999"])

	// Check metadata
	metadata, ok := privacyDict["_metadata"].(map[string]interface{})
	assert.True(t, ok, "Expected _metadata to be a map")
	assert.Equal(t, float64(1000), metadata["total_rules"])
}

// Add benchmark tests.
func BenchmarkCreatePrivacyDictFromReplacementRules(b *testing.B) {
	replacementRules := map[string]string{
		"PERSON_001":    "John Smith",
		"PERSON_002":    "Jane Doe",
		"LOCATION_001":  "New York",
		"SENSITIVE_001": "2025-07-13",
		"EMAIL_001":     "test@example.com",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := createPrivacyDictFromReplacementRules("test-chat", replacementRules)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCreatePrivacyDictFromReplacementRules_Large(b *testing.B) {
	// Create a large dataset for benchmarking
	replacementRules := make(map[string]string)
	for i := 0; i < 100; i++ {
		token := fmt.Sprintf("PERSON_%03d", i)
		name := fmt.Sprintf("User Name %d", i)
		replacementRules[token] = name
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := createPrivacyDictFromReplacementRules("test-chat", replacementRules)
		if err != nil {
			b.Fatal(err)
		}
	}
}

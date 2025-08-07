package memory

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryFactsJSONExportImport(t *testing.T) {
	// Create test data
	testFacts := []MemoryFact{
		{
			ID:          uuid.New().String(),
			Content:     "Test fact 1 about health and fitness",
			Category:    "health",
			Subject:     "Physical Health & Fitness",
			Attribute:   "exercise_routine",
			Value:       "Daily morning runs",
			Importance:  1,
			Sensitivity: "low",
			Timestamp:   time.Now(),
			Source:      "test",
			Metadata: map[string]string{
				"test": "data",
			},
		},
		{
			ID:          uuid.New().String(),
			Content:     "Test fact 2 about career progression",
			Category:    "career",
			Subject:     "Career & Professional Life",
			Attribute:   "job_title",
			Value:       "Senior Software Engineer",
			Importance:  2,
			Sensitivity: "medium",
			Timestamp:   time.Now().Add(-24 * time.Hour),
			Source:      "test",
			Metadata: map[string]string{
				"company": "Tech Corp",
			},
		},
		{
			ID:          uuid.New().String(),
			Content:     "Test fact 3 about personal preferences",
			Category:    "preferences",
			Subject:     "Media & Culture Tastes",
			Attribute:   "music_genre",
			Value:       "Jazz and Classical",
			Importance:  1,
			Sensitivity: "low",
			Timestamp:   time.Now().Add(-48 * time.Hour),
			Source:      "test",
			Metadata:    map[string]string{},
		},
	}

	// Create temporary file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test_facts.jsonl")

	t.Run("ExportMemoryFactsJSON", func(t *testing.T) {
		err := ExportMemoryFactsJSON(testFacts, testFile)
		require.NoError(t, err)

		// Verify file exists and has content
		info, err := os.Stat(testFile)
		require.NoError(t, err)
		assert.Greater(t, info.Size(), int64(0))
	})

	t.Run("LoadMemoryFactsFromJSON", func(t *testing.T) {
		loadedFacts, err := LoadMemoryFactsFromJSON(testFile)
		require.NoError(t, err)

		// Verify same number of facts
		assert.Len(t, loadedFacts, len(testFacts))

		// Verify facts match (order might be different)
		factMap := make(map[string]MemoryFact)
		for _, fact := range loadedFacts {
			factMap[fact.ID] = fact
		}

		for _, originalFact := range testFacts {
			loadedFact, exists := factMap[originalFact.ID]
			require.True(t, exists, "Fact with ID %s not found", originalFact.ID)

			assert.Equal(t, originalFact.Content, loadedFact.Content)
			assert.Equal(t, originalFact.Category, loadedFact.Category)
			assert.Equal(t, originalFact.Subject, loadedFact.Subject)
			assert.Equal(t, originalFact.Attribute, loadedFact.Attribute)
			assert.Equal(t, originalFact.Value, loadedFact.Value)
			assert.Equal(t, originalFact.Importance, loadedFact.Importance)
			assert.Equal(t, originalFact.Sensitivity, loadedFact.Sensitivity)
			assert.Equal(t, originalFact.Source, loadedFact.Source)

			// Timestamps should be close (JSONL might lose some precision)
			assert.WithinDuration(t, originalFact.Timestamp, loadedFact.Timestamp, time.Second)
		}
	})

	t.Run("EmptyFactsExport", func(t *testing.T) {
		emptyFile := filepath.Join(tempDir, "empty_facts.jsonl")
		err := ExportMemoryFactsJSON([]MemoryFact{}, emptyFile)
		require.NoError(t, err)

		// File should exist but be empty
		info, err := os.Stat(emptyFile)
		require.NoError(t, err)
		assert.Equal(t, int64(0), info.Size())
	})

	t.Run("LoadFromNonexistentFile", func(t *testing.T) {
		nonexistentFile := filepath.Join(tempDir, "nonexistent.jsonl")
		_, err := LoadMemoryFactsFromJSON(nonexistentFile)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to open JSONL file")
	})

	t.Run("ExportToInvalidPath", func(t *testing.T) {
		invalidPath := "/nonexistent/directory/test.jsonl"
		err := ExportMemoryFactsJSON(testFacts, invalidPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create JSONL file")
	})
}

func TestMemoryFactsJSONRoundTrip(t *testing.T) {
	// Test various edge cases and data types
	testFacts := []MemoryFact{
		{
			ID:       uuid.New().String(),
			Content:  "Fact with special characters: áéíóú ñ ç 中文 🎉",
			Category: "test-category",
			Subject:  "Test Subject",
			Metadata: map[string]string{
				"test_type": "special_characters",
			},
		},
		{
			ID:          uuid.New().String(),
			Content:     "",                  // Empty content
			Category:    "",                  // Empty category
			Subject:     "",                  // Empty subject
			Attribute:   "",                  // Empty attribute
			Value:       "",                  // Empty value
			Importance:  1,                   // Min importance
			Sensitivity: "high",              // Max sensitivity
			Metadata:    map[string]string{}, // Empty metadata
		},
	}

	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "roundtrip_test.jsonl")

	// Export
	err := ExportMemoryFactsJSON(testFacts, testFile)
	require.NoError(t, err)

	// Import
	loadedFacts, err := LoadMemoryFactsFromJSON(testFile)
	require.NoError(t, err)

	// Verify round-trip integrity
	require.Len(t, loadedFacts, len(testFacts))

	for i, originalFact := range testFacts {
		loadedFact := loadedFacts[i]

		assert.Equal(t, originalFact.ID, loadedFact.ID)
		assert.Equal(t, originalFact.Content, loadedFact.Content)
		assert.Equal(t, originalFact.Category, loadedFact.Category)
		assert.Equal(t, originalFact.Subject, loadedFact.Subject)
		assert.Equal(t, originalFact.Attribute, loadedFact.Attribute)
		assert.Equal(t, originalFact.Value, loadedFact.Value)
		assert.Equal(t, originalFact.Importance, loadedFact.Importance)
		assert.Equal(t, originalFact.Sensitivity, loadedFact.Sensitivity)

		// Verify metadata preservation
		assert.Equal(t, len(originalFact.Metadata), len(loadedFact.Metadata))
		for key, originalValue := range originalFact.Metadata {
			loadedValue, exists := loadedFact.Metadata[key]
			assert.True(t, exists, "Metadata key %s not found", key)
			assert.Equal(t, originalValue, loadedValue, "Metadata value mismatch for key %s", key)
		}
	}
}

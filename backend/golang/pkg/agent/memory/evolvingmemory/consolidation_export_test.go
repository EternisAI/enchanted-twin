package evolvingmemory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
)

func TestConsolidationReportExportJSON(t *testing.T) {
	// Create test consolidation report
	testReport := &ConsolidationReport{
		Topic:           "Physical Health & Fitness",
		Summary:         "User maintains an active lifestyle with regular exercise routines and healthy eating habits.",
		SourceFactCount: 5,
		ConsolidatedFacts: []*ConsolidationFact{
			{
				MemoryFact: memory.MemoryFact{
					ID:          uuid.New().String(),
					Content:     "User consistently exercises 4-5 times per week, focusing on cardio and strength training",
					Category:    "health",
					Subject:     "Physical Health & Fitness",
					Attribute:   "exercise_consistency",
					Value:       "4-5 times per week",
					Importance:  2,
					Sensitivity: "low",
					Timestamp:   time.Now(),
					Source:      "consolidation",
					Metadata: map[string]string{
						"consolidation_type": "pattern_synthesis",
					},
				},
				ConsolidatedFrom: []string{
					uuid.New().String(),
					uuid.New().String(),
				},
			},
			{
				MemoryFact: memory.MemoryFact{
					ID:          uuid.New().String(),
					Content:     "User prefers outdoor activities and shows interest in new fitness challenges",
					Category:    "preferences",
					Subject:     "Physical Health & Fitness",
					Attribute:   "activity_preferences",
					Value:       "outdoor activities, fitness challenges",
					Importance:  1,
					Sensitivity: "medium",
					Timestamp:   time.Now().Add(-24 * time.Hour),
					Source:      "consolidation",
					Metadata: map[string]string{
						"consolidation_type": "preference_analysis",
					},
				},
				ConsolidatedFrom: []string{
					uuid.New().String(),
				},
			},
		},
	}

	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test_consolidation_report.json")

	t.Run("ExportJSON_Success", func(t *testing.T) {
		err := testReport.ExportJSON(testFile)
		require.NoError(t, err)

		// Verify file exists and has content
		info, err := os.Stat(testFile)
		require.NoError(t, err)
		assert.Greater(t, info.Size(), int64(0))

		// Verify file content by reading and parsing
		fileContent, err := os.ReadFile(testFile)
		require.NoError(t, err)

		var loadedReport ConsolidationReport
		err = json.Unmarshal(fileContent, &loadedReport)
		require.NoError(t, err)

		// Verify basic fields
		assert.Equal(t, testReport.Topic, loadedReport.Topic)
		assert.Equal(t, testReport.Summary, loadedReport.Summary)
		assert.Equal(t, testReport.SourceFactCount, loadedReport.SourceFactCount)
		assert.Len(t, loadedReport.ConsolidatedFacts, len(testReport.ConsolidatedFacts))

		// Verify consolidated facts
		for i, originalFact := range testReport.ConsolidatedFacts {
			loadedFact := loadedReport.ConsolidatedFacts[i]

			assert.Equal(t, originalFact.ID, loadedFact.ID)
			assert.Equal(t, originalFact.Content, loadedFact.Content)
			assert.Equal(t, originalFact.Category, loadedFact.Category)
			assert.Equal(t, originalFact.Subject, loadedFact.Subject)
			assert.Equal(t, originalFact.Attribute, loadedFact.Attribute)
			assert.Equal(t, originalFact.Value, loadedFact.Value)
			assert.Equal(t, originalFact.Importance, loadedFact.Importance)
			assert.Equal(t, originalFact.Sensitivity, loadedFact.Sensitivity)
			assert.Equal(t, originalFact.Source, loadedFact.Source)
			assert.Equal(t, originalFact.ConsolidatedFrom, loadedFact.ConsolidatedFrom)

			// Verify metadata
			assert.Equal(t, len(originalFact.Metadata), len(loadedFact.Metadata))
			for key, originalValue := range originalFact.Metadata {
				loadedValue, exists := loadedFact.Metadata[key]
				assert.True(t, exists, "Metadata key %s not found", key)
				assert.Equal(t, originalValue, loadedValue)
			}
		}
	})

	t.Run("ExportJSON_EmptyReport", func(t *testing.T) {
		emptyReport := &ConsolidationReport{
			Topic:             "Empty Topic",
			Summary:           "",
			SourceFactCount:   0,
			ConsolidatedFacts: []*ConsolidationFact{},
		}

		emptyFile := filepath.Join(tempDir, "empty_report.json")
		err := emptyReport.ExportJSON(emptyFile)
		require.NoError(t, err)

		// Verify file content
		fileContent, err := os.ReadFile(emptyFile)
		require.NoError(t, err)

		var loadedReport ConsolidationReport
		err = json.Unmarshal(fileContent, &loadedReport)
		require.NoError(t, err)

		assert.Equal(t, emptyReport.Topic, loadedReport.Topic)
		assert.Equal(t, emptyReport.Summary, loadedReport.Summary)
		assert.Equal(t, emptyReport.SourceFactCount, loadedReport.SourceFactCount)
		assert.Len(t, loadedReport.ConsolidatedFacts, 0)
	})

	t.Run("ExportJSON_InvalidPath", func(t *testing.T) {
		invalidPath := "/nonexistent/directory/test.json"
		err := testReport.ExportJSON(invalidPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to write JSON file")
	})

	t.Run("ExportJSON_PrettyFormat", func(t *testing.T) {
		prettyFile := filepath.Join(tempDir, "pretty_report.json")
		err := testReport.ExportJSON(prettyFile)
		require.NoError(t, err)

		// Read the file content to verify it's pretty-printed
		fileContent, err := os.ReadFile(prettyFile)
		require.NoError(t, err)

		// Pretty-printed JSON should contain newlines and indentation
		contentStr := string(fileContent)
		assert.Contains(t, contentStr, "\n") // Should have newlines
		assert.Contains(t, contentStr, "  ") // Should have 2-space indentation

		// Should be valid JSON
		var testObj interface{}
		err = json.Unmarshal(fileContent, &testObj)
		require.NoError(t, err)
	})
}

func TestConsolidationReportJSONRoundTrip(t *testing.T) {
	// Test with various edge cases and data types
	testReport := &ConsolidationReport{
		Topic:           "Test Topic with Special Characters: áéíóú ñ ç 中文 🎉",
		Summary:         "Summary with\nmultiple\nlines and \"quotes\" and 'apostrophes'",
		SourceFactCount: 100,
		ConsolidatedFacts: []*ConsolidationFact{
			{
				MemoryFact: memory.MemoryFact{
					ID:       uuid.New().String(),
					Content:  "Fact with unicode: ∑∫∂ƒ∆ and emojis: 🚀🎯🧠",
					Category: "test",
					Subject:  "Test Subject",
					Metadata: map[string]string{
						"test_type": "unicode",
					},
				},
				ConsolidatedFrom: []string{
					uuid.New().String(),
					uuid.New().String(),
					uuid.New().String(),
				},
			},
		},
	}

	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "roundtrip_test.json")

	// Export
	err := testReport.ExportJSON(testFile)
	require.NoError(t, err)

	// Import
	fileContent, err := os.ReadFile(testFile)
	require.NoError(t, err)

	var loadedReport ConsolidationReport
	err = json.Unmarshal(fileContent, &loadedReport)
	require.NoError(t, err)

	// Verify round-trip integrity
	assert.Equal(t, testReport.Topic, loadedReport.Topic)
	assert.Equal(t, testReport.Summary, loadedReport.Summary)
	assert.Equal(t, testReport.SourceFactCount, loadedReport.SourceFactCount)
	require.Len(t, loadedReport.ConsolidatedFacts, len(testReport.ConsolidatedFacts))

	originalFact := testReport.ConsolidatedFacts[0]
	loadedFact := loadedReport.ConsolidatedFacts[0]

	assert.Equal(t, originalFact.ID, loadedFact.ID)
	assert.Equal(t, originalFact.Content, loadedFact.Content)
	assert.Equal(t, originalFact.ConsolidatedFrom, loadedFact.ConsolidatedFrom)

	// Verify complex metadata preservation
	assert.Equal(t, len(originalFact.Metadata), len(loadedFact.Metadata))
	for key, originalValue := range originalFact.Metadata {
		loadedValue, exists := loadedFact.Metadata[key]
		assert.True(t, exists, "Metadata key %s not found", key)
		assert.Equal(t, originalValue, loadedValue)
	}
}

// Benchmark the export functionality.
func BenchmarkConsolidationReportExportJSON(b *testing.B) {
	// Create a large report for benchmarking
	facts := make([]*ConsolidationFact, 1000)
	for i := 0; i < 1000; i++ {
		facts[i] = &ConsolidationFact{
			MemoryFact: memory.MemoryFact{
				ID:       uuid.New().String(),
				Content:  "Benchmark test fact with some content that is reasonably long to simulate real data",
				Category: "benchmark",
				Subject:  "Benchmark Subject",
				Metadata: map[string]string{
					"data": "some metadata",
				},
			},
			ConsolidatedFrom: []string{uuid.New().String(), uuid.New().String()},
		}
	}

	report := &ConsolidationReport{
		Topic:             "Benchmark Topic",
		Summary:           "This is a benchmark summary for testing export performance",
		SourceFactCount:   1000,
		ConsolidatedFacts: facts,
	}

	tempDir := b.TempDir()
	testFile := filepath.Join(tempDir, "benchmark_report.json")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := report.ExportJSON(testFile)
		if err != nil {
			b.Fatalf("Export failed: %v", err)
		}
	}
}

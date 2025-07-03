package helpers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
)

func TestReadJSONLBatch(t *testing.T) {
	testRecords := []types.Record{
		{
			Data: map[string]interface{}{
				"content": "First record",
				"id":      "1",
			},
			Timestamp: time.Date(2024, 1, 3, 10, 0, 0, 0, time.UTC),
			Source:    "test",
		},
		{
			Data: map[string]interface{}{
				"content": "Second record",
				"id":      "2",
			},
			Timestamp: time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC), // Earlier timestamp
			Source:    "test",
		},
		{
			Data: map[string]interface{}{
				"content": "Third record",
				"id":      "3",
			},
			Timestamp: time.Date(2024, 1, 5, 10, 0, 0, 0, time.UTC),
			Source:    "test",
		},
		{
			Data: map[string]interface{}{
				"content": "Fourth record",
				"id":      "4",
			},
			Timestamp: time.Date(2024, 1, 2, 10, 0, 0, 0, time.UTC),
			Source:    "test",
		},
	}

	tempFile := createTestJSONLFile(t, testRecords)
	defer os.Remove(tempFile) //nolint:errcheck

	t.Run("Normal operation - read all records", func(t *testing.T) {
		records, err := ReadJSONLBatch(tempFile, 0, 10)
		require.NoError(t, err)
		assert.Len(t, records, 4)

		assert.Equal(t, "2", records[0].Data["id"]) // 2024-01-01
		assert.Equal(t, "4", records[1].Data["id"]) // 2024-01-02
		assert.Equal(t, "1", records[2].Data["id"]) // 2024-01-03
		assert.Equal(t, "3", records[3].Data["id"]) // 2024-01-05
	})

	t.Run("Batch reading - first batch", func(t *testing.T) {
		records, err := ReadJSONLBatch(tempFile, 0, 2)
		require.NoError(t, err)
		assert.Len(t, records, 2)

		assert.Equal(t, "Second record", records[0].Data["content"]) // 2024-01-01
		assert.Equal(t, "First record", records[1].Data["content"])  // 2024-01-03
	})

	t.Run("Batch reading - second batch", func(t *testing.T) {
		records, err := ReadJSONLBatch(tempFile, 2, 2)
		require.NoError(t, err)
		assert.Len(t, records, 2)

		assert.Equal(t, "Fourth record", records[0].Data["content"]) // 2024-01-02
		assert.Equal(t, "Third record", records[1].Data["content"])  // 2024-01-05
	})

	t.Run("Batch reading - partial last batch", func(t *testing.T) {
		records, err := ReadJSONLBatch(tempFile, 3, 5)
		require.NoError(t, err)
		assert.Len(t, records, 1)

		assert.Equal(t, "Fourth record", records[0].Data["content"])
	})

	t.Run("Start index beyond file", func(t *testing.T) {
		records, err := ReadJSONLBatch(tempFile, 10, 2)
		require.NoError(t, err)
		assert.Empty(t, records)
	})

	t.Run("Batch size larger than remaining records", func(t *testing.T) {
		records, err := ReadJSONLBatch(tempFile, 2, 10)
		require.NoError(t, err)
		assert.Len(t, records, 2) // Only 2 records left from index 2
	})

	t.Run("Read batch with very large lines", func(t *testing.T) {
		// Create a test file with very large lines
		file, err := os.CreateTemp("", "test_large_batch_*.jsonl")
		require.NoError(t, err)
		defer func() {
			_ = os.Remove(file.Name())
		}()

		// Create test records with large data
		baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

		for i := 0; i < 3; i++ {
			// Create a large conversation array
			var messages []map[string]string
			for j := 0; j < 500; j++ {
				messages = append(messages, map[string]string{
					"speaker": fmt.Sprintf("User%d", j%2),
					"content": fmt.Sprintf("This is test message %d in conversation %d with additional text", j, i),
					"time":    baseTime.Add(time.Duration(j) * time.Minute).Format(time.RFC3339),
				})
			}

			record := types.Record{
				Data: map[string]interface{}{
					"id":           fmt.Sprintf("conversation-%d", i),
					"source":       "test",
					"conversation": messages,
				},
				Timestamp: baseTime.Add(time.Duration(i) * time.Hour),
				Source:    "test",
			}

			// Marshal and write to file
			jsonData, err := json.Marshal(record)
			require.NoError(t, err)
			_, err = file.WriteString(string(jsonData) + "\n")
			require.NoError(t, err)
		}
		require.NoError(t, file.Close())

		// Test reading all records
		records, err := ReadJSONLBatch(file.Name(), 0, 3)
		require.NoError(t, err)
		assert.Len(t, records, 3)

		// Verify the records were read correctly
		for i, record := range records {
			assert.Equal(t, fmt.Sprintf("conversation-%d", i), record.Data["id"])
			conversations, ok := record.Data["conversation"].([]interface{})
			assert.True(t, ok)
			assert.Len(t, conversations, 500)
		}
	})
}

func TestReadJSONLBatch_ErrorCases(t *testing.T) {
	t.Run("Empty file path", func(t *testing.T) {
		records, err := ReadJSONLBatch("", 0, 10)
		assert.Error(t, err)
		assert.Nil(t, records)
		assert.Contains(t, err.Error(), "filePath cannot be empty")
	})

	t.Run("Negative start index", func(t *testing.T) {
		records, err := ReadJSONLBatch("test.jsonl", -1, 10)
		assert.Error(t, err)
		assert.Nil(t, records)
		assert.Contains(t, err.Error(), "startIndex cannot be negative")
	})

	t.Run("Zero batch size", func(t *testing.T) {
		records, err := ReadJSONLBatch("test.jsonl", 0, 0)
		assert.Error(t, err)
		assert.Nil(t, records)
		assert.Contains(t, err.Error(), "batchSize must be positive")
	})

	t.Run("Negative batch size", func(t *testing.T) {
		records, err := ReadJSONLBatch("test.jsonl", 0, -5)
		assert.Error(t, err)
		assert.Nil(t, records)
		assert.Contains(t, err.Error(), "batchSize must be positive")
	})

	t.Run("Non-existent file", func(t *testing.T) {
		records, err := ReadJSONLBatch("non-existent-file.jsonl", 0, 10)
		assert.Error(t, err)
		assert.Nil(t, records)
		assert.Contains(t, err.Error(), "failed to open file")
	})

	t.Run("Invalid JSON in file", func(t *testing.T) {
		tempFile := createInvalidJSONLFile(t)
		defer os.Remove(tempFile) //nolint:errcheck

		records, err := ReadJSONLBatch(tempFile, 0, 10)
		assert.Error(t, err)
		assert.Nil(t, records)
		assert.Contains(t, err.Error(), "failed to unmarshal line")
	})
}

func TestReadJSONLBatch_EdgeCases(t *testing.T) {
	t.Run("Empty file", func(t *testing.T) {
		tempFile := createEmptyFile(t)
		defer os.Remove(tempFile) //nolint:errcheck

		records, err := ReadJSONLBatch(tempFile, 0, 10)
		require.NoError(t, err)
		assert.Empty(t, records)
	})

	t.Run("Single record file", func(t *testing.T) {
		testRecord := []types.Record{
			{
				Data: map[string]interface{}{
					"content": "Only record",
				},
				Timestamp: time.Now(),
				Source:    "test",
			},
		}

		tempFile := createTestJSONLFile(t, testRecord)
		defer os.Remove(tempFile) //nolint:errcheck

		records, err := ReadJSONLBatch(tempFile, 0, 10)
		require.NoError(t, err)
		assert.Len(t, records, 1)
		assert.Equal(t, "Only record", records[0].Data["content"])
	})

	t.Run("Batch size of 1", func(t *testing.T) {
		testRecords := []types.Record{
			{
				Data:      map[string]interface{}{"id": "1"},
				Timestamp: time.Now(),
				Source:    "test",
			},
			{
				Data:      map[string]interface{}{"id": "2"},
				Timestamp: time.Now().Add(time.Hour),
				Source:    "test",
			},
		}

		tempFile := createTestJSONLFile(t, testRecords)
		defer os.Remove(tempFile) //nolint:errcheck

		// Read first record
		records, err := ReadJSONLBatch(tempFile, 0, 1)
		require.NoError(t, err)
		assert.Len(t, records, 1)
		assert.Equal(t, "1", records[0].Data["id"])

		// Read second record
		records, err = ReadJSONLBatch(tempFile, 1, 1)
		require.NoError(t, err)
		assert.Len(t, records, 1)
		assert.Equal(t, "2", records[0].Data["id"])
	})
}

func TestReadJSONLBatch_Sorting(t *testing.T) {
	testRecords := []types.Record{
		{
			Data:      map[string]interface{}{"order": "3rd"},
			Timestamp: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			Source:    "test",
		},
		{
			Data:      map[string]interface{}{"order": "1st"},
			Timestamp: time.Date(2024, 1, 10, 10, 0, 0, 0, time.UTC),
			Source:    "test",
		},
		{
			Data:      map[string]interface{}{"order": "2nd"},
			Timestamp: time.Date(2024, 1, 12, 10, 0, 0, 0, time.UTC),
			Source:    "test",
		},
	}

	tempFile := createTestJSONLFile(t, testRecords)
	defer os.Remove(tempFile) //nolint:errcheck

	t.Run("Records are sorted by timestamp ascending", func(t *testing.T) {
		records, err := ReadJSONLBatch(tempFile, 0, 10)
		require.NoError(t, err)
		assert.Len(t, records, 3)

		assert.Equal(t, "1st", records[0].Data["order"])
		assert.Equal(t, "2nd", records[1].Data["order"])
		assert.Equal(t, "3rd", records[2].Data["order"])

		assert.True(t, records[0].Timestamp.Before(records[1].Timestamp))
		assert.True(t, records[1].Timestamp.Before(records[2].Timestamp))
	})

	t.Run("Single record doesn't break sorting", func(t *testing.T) {
		records, err := ReadJSONLBatch(tempFile, 0, 1)
		require.NoError(t, err)
		assert.Len(t, records, 1)
	})
}

func TestCountJSONLLines(t *testing.T) {
	testRecords := []types.Record{
		{Data: map[string]interface{}{"id": "1"}, Timestamp: time.Now(), Source: "test"},
		{Data: map[string]interface{}{"id": "2"}, Timestamp: time.Now(), Source: "test"},
		{Data: map[string]interface{}{"id": "3"}, Timestamp: time.Now(), Source: "test"},
	}

	tempFile := createTestJSONLFile(t, testRecords)
	defer os.Remove(tempFile) //nolint:errcheck

	t.Run("Count lines in normal file", func(t *testing.T) {
		count, err := CountJSONLLines(tempFile)
		require.NoError(t, err)
		assert.Equal(t, 3, count)
	})

	t.Run("Count lines in empty file", func(t *testing.T) {
		emptyFile := createEmptyFile(t)
		defer os.Remove(emptyFile) //nolint:errcheck

		count, err := CountJSONLLines(emptyFile)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("Error on empty file path", func(t *testing.T) {
		count, err := CountJSONLLines("")
		assert.Error(t, err)
		assert.Equal(t, 0, count)
		assert.Contains(t, err.Error(), "filePath cannot be empty")
	})

	t.Run("Error on non-existent file", func(t *testing.T) {
		count, err := CountJSONLLines("non-existent.jsonl")
		assert.Error(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("Count lines with very large lines", func(t *testing.T) {
		// Create a test file with very large lines (simulating WhatsApp conversations)
		file, err := os.CreateTemp("", "test_large_lines_*.jsonl")
		require.NoError(t, err)
		defer func() {
			_ = os.Remove(file.Name())
		}()

		// Create a large JSON object (> 64KB to exceed default buffer)
		largeData := make(map[string]interface{})
		largeData["id"] = "test-conversation"
		largeData["source"] = "whatsapp"

		// Create a large conversation array
		var messages []map[string]string
		for i := 0; i < 1000; i++ {
			messages = append(messages, map[string]string{
				"speaker": "User",
				"content": fmt.Sprintf("This is a test message number %d with some additional text to make it longer", i),
				"time":    time.Now().Format(time.RFC3339),
			})
		}
		largeData["conversation"] = messages

		// Write 3 large lines
		for i := 0; i < 3; i++ {
			jsonData, err := json.Marshal(largeData)
			require.NoError(t, err)

			_, err = file.WriteString(string(jsonData) + "\n")
			require.NoError(t, err)
		}
		require.NoError(t, file.Close())

		// Test counting lines
		count, err := CountJSONLLines(file.Name())
		require.NoError(t, err)
		assert.Equal(t, 3, count, "Should count 3 large lines correctly")
	})
}

func createTestJSONLFile(t *testing.T, records []types.Record) string {
	t.Helper()

	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "test.jsonl")

	file, err := os.Create(tempFile)
	require.NoError(t, err)
	defer file.Close() //nolint:errcheck

	for _, record := range records {
		jsonRecord := struct {
			Data      map[string]interface{} `json:"data"`
			Timestamp string                 `json:"timestamp"`
			Source    string                 `json:"source"`
		}{
			Data:      record.Data,
			Timestamp: record.Timestamp.Format(time.RFC3339),
			Source:    record.Source,
		}

		jsonData, err := json.Marshal(jsonRecord)
		require.NoError(t, err)

		_, err = file.Write(jsonData)
		require.NoError(t, err)

		_, err = file.WriteString("\n")
		require.NoError(t, err)
	}

	return tempFile
}

func createInvalidJSONLFile(t *testing.T) string {
	t.Helper()

	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "invalid.jsonl")

	err := os.WriteFile(tempFile, []byte("invalid json line\n{\"valid\": \"json\"}\n"), 0o644)
	require.NoError(t, err)

	return tempFile
}

func createEmptyFile(t *testing.T) string {
	t.Helper()

	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "empty.jsonl")

	file, err := os.Create(tempFile)
	require.NoError(t, err)
	file.Close() //nolint:errcheck

	return tempFile
}

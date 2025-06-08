package helpers

import (
	"encoding/json"
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
	defer os.Remove(tempFile)

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
		defer os.Remove(tempFile)

		records, err := ReadJSONLBatch(tempFile, 0, 10)
		assert.Error(t, err)
		assert.Nil(t, records)
		assert.Contains(t, err.Error(), "failed to unmarshal line")
	})
}

func TestReadJSONLBatch_EdgeCases(t *testing.T) {
	t.Run("Empty file", func(t *testing.T) {
		tempFile := createEmptyFile(t)
		defer os.Remove(tempFile)

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
		defer os.Remove(tempFile)

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
		defer os.Remove(tempFile)

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
	defer os.Remove(tempFile)

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
	defer os.Remove(tempFile)

	t.Run("Count lines in normal file", func(t *testing.T) {
		count, err := CountJSONLLines(tempFile)
		require.NoError(t, err)
		assert.Equal(t, 3, count)
	})

	t.Run("Count lines in empty file", func(t *testing.T) {
		emptyFile := createEmptyFile(t)
		defer os.Remove(emptyFile)

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
}

func createTestJSONLFile(t *testing.T, records []types.Record) string {
	t.Helper()

	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "test.jsonl")

	file, err := os.Create(tempFile)
	require.NoError(t, err)
	defer file.Close()

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
	file.Close()

	return tempFile
}

package misc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProcessFile(t *testing.T) {
	// Create a temporary file with some content
	tempDir, err := os.MkdirTemp("", "misc_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test text file
	testContent := "This is a test file.\nIt has multiple lines.\nEach line has some content.\nWe want to make sure it processes correctly."
	// Write the content to the file
	testFilePath := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFilePath, []byte(testContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test with default chunk size
	source := New(tempDir)
	records, err := source.ProcessFile(testFilePath)
	if err != nil {
		t.Fatalf("ProcessFile failed: %v", err)
	}

	// With default chunk size of 1000, the entire content should fit in one record
	if len(records) != 1 {
		t.Errorf("Expected 1 record, got %d", len(records))
	}

	if len(records) > 0 {
		content, ok := records[0].Data["content"].(string)
		if !ok {
			t.Errorf("Expected content to be a string")
		}

		// Normalize line endings for comparison
		normalizedExpected := strings.ReplaceAll(testContent, "\r\n", "\n")
		normalizedActual := strings.ReplaceAll(content, "\r\n", "\n")

		if normalizedActual != normalizedExpected {
			t.Errorf("Content mismatch.\nExpected: %q\nGot: %q", normalizedExpected, normalizedActual)
		}
	}

	// Test with a smaller chunk size to force multiple chunks
	source = New(tempDir).WithChunkSize(30)
	records, err = source.ProcessFile(testFilePath)
	if err != nil {
		t.Fatalf("ProcessFile with small chunk size failed: %v", err)
	}

	// With 30-character chunks, we should have multiple records
	if len(records) <= 1 {
		t.Errorf("Expected multiple records with small chunk size, got %d", len(records))
	}

	// Check if chunk IDs are sequential
	for i, record := range records {
		chunkID, ok := record.Data["chunk"].(int)
		if !ok {
			t.Errorf("Expected chunk to be an int")
		}
		if chunkID != i {
			t.Errorf("Expected chunk ID %d, got %d", i, chunkID)
		}
	}
}

func TestProcessDirectory(t *testing.T) {
	// Create a temporary directory structure
	tempDir, err := os.MkdirTemp("", "misc_test_dir")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a subdirectory
	subDir := filepath.Join(tempDir, "subdir")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	// Create multiple test files with different extensions
	files := map[string]string{
		"test1.txt": "This is text file 1",
		"test2.txt": "This is text file 2",
		"test3.md":  "# This is a markdown file",
		"test4.log": "INFO: This is a log file",
		"test5.csv": "field1,field2,field3", // Should be ignored by default
	}

	for name, content := range files {
		filePath := filepath.Join(tempDir, name)
		err = os.WriteFile(filePath, []byte(content), 0o644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", name, err)
		}
	}

	// Create a file in the subdirectory
	subDirFile := filepath.Join(subDir, "subdir_test.txt")
	err = os.WriteFile(subDirFile, []byte("This is in the subdirectory"), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test file in subdirectory: %v", err)
	}

	// Test directory processing
	source := New(tempDir)
	records, err := source.ProcessDirectory()
	if err != nil {
		t.Fatalf("ProcessDirectory failed: %v", err)
	}

	// Should have 4 records (3 txt files and 1 md file, ignoring the csv)
	expectedCount := 5 // txt (2) + md (1) + log (1) + subdirectory txt (1)
	if len(records) != expectedCount {
		t.Errorf("Expected %d records, got %d", expectedCount, len(records))
	}

	// Test with custom extensions
	source = New(tempDir).WithExtensions([]string{".txt"})
	records, err = source.ProcessDirectory()
	if err != nil {
		t.Fatalf("ProcessDirectory with custom extensions failed: %v", err)
	}

	// Should have 3 .txt files only (2 in root, 1 in subdirectory)
	expectedCount = 3
	if len(records) != expectedCount {
		t.Errorf("Expected %d records with .txt extension only, got %d", expectedCount, len(records))
	}
}

package misc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

func TestProcessFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "misc_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Fatalf("Failed to remove temp directory: %v", err)
		}
	}()

	testContent := "This is a test file.\nIt has multiple lines.\nEach line has some content.\nWe want to make sure it processes correctly."
	testFilePath := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFilePath, []byte(testContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	source := New(nil)
	records, err := source.ProcessFile(testFilePath)
	if err != nil {
		t.Fatalf("ProcessFile failed: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("Expected 1 record, got %d", len(records))
	}

	if len(records) > 0 {
		content, ok := records[0].Data["content"].(string)
		if !ok {
			t.Errorf("Expected content to be a string")
		}

		normalizedExpected := strings.ReplaceAll(testContent, "\r\n", "\n")
		normalizedActual := strings.ReplaceAll(content, "\r\n", "\n")

		if normalizedActual != normalizedExpected {
			t.Errorf("Content mismatch.\nExpected: %q\nGot: %q", normalizedExpected, normalizedActual)
		}
	}

	source = New(nil).WithChunkSize(30)
	records, err = source.ProcessFile(testFilePath)
	if err != nil {
		t.Fatalf("ProcessFile with small chunk size failed: %v", err)
	}

	if len(records) <= 1 {
		t.Errorf("Expected multiple records with small chunk size, got %d", len(records))
	}

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
	tempDir, err := os.MkdirTemp("", "misc_test_dir")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Fatalf("Failed to remove temp directory: %v", err)
		}
	}()

	subDir := filepath.Join(tempDir, "subdir")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	files := map[string]string{
		"test1.txt": "This is text file 1",
		"test2.txt": "This is text file 2",
		"test3.md":  "# This is a markdown file",
		"test4.log": "INFO: This is a log file",
		"test5.csv": "field1,field2,field3",
	}

	for name, content := range files {
		filePath := filepath.Join(tempDir, name)
		err = os.WriteFile(filePath, []byte(content), 0o644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", name, err)
		}
	}

	subDirFile := filepath.Join(subDir, "subdir_test.txt")
	err = os.WriteFile(subDirFile, []byte("This is in the subdirectory"), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test file in subdirectory: %v", err)
	}

	openAiService := ai.NewOpenAIService("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", "https://api.openai.com/v1")

	source := New(openAiService)
	records, err := source.ProcessDirectory(tempDir)
	if err != nil {
		t.Fatalf("ProcessDirectory failed: %v", err)
	}

	expectedCount := 5
	if len(records) != expectedCount {
		t.Errorf("Expected %d records, got %d", expectedCount, len(records))
	}

	source = New(nil)
	records, err = source.ProcessDirectory(tempDir)
	if err != nil {
		t.Fatalf("ProcessDirectory failed: %v", err)
	}

	expectedCount = 3
	if len(records) != expectedCount {
		t.Errorf("Expected %d records with .txt extension only, got %d", expectedCount, len(records))
	}
}

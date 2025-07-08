package misc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWordDocumentExtraction(t *testing.T) {
	testDir := t.TempDir()
	testFilePath := filepath.Join(testDir, "test.docx")

	integrationTestFile := filepath.Join("..", "integration", "testdata", "misc", "test_doc.docx")

	if _, err := os.Stat(integrationTestFile); err == nil {
		input, err := os.ReadFile(integrationTestFile)
		require.NoError(t, err, "Failed to read integration test file")

		err = os.WriteFile(testFilePath, input, 0o644)
		require.NoError(t, err, "Failed to create test file")
	} else {
		t.Skip("Integration test file not found, skipping Word document extraction test")
	}

	logger := log.New(os.Stderr)
	logger.SetLevel(log.DebugLevel)

	textProcessor := &TextDocumentProcessor{
		logger: logger,
	}

	t.Run("ExtractTextFromWord", func(t *testing.T) {
		text, err := textProcessor.ExtractTextFromWord(testFilePath)
		require.NoError(t, err, "Should extract text from Word document")

		assert.NotEmpty(t, text, "Extracted text should not be empty")
		assert.True(t, len(strings.TrimSpace(text)) > 0, "Should extract meaningful text")

		assert.NotContains(t, text, "<?xml", "Should not contain XML headers")
		assert.NotContains(t, text, "<w:", "Should not contain Word XML tags")
		assert.NotContains(t, text, "\x00", "Should not contain null bytes")

		t.Logf("Extracted text length: %d characters", len(text))
		t.Logf("Text preview: %s", text[:min(200, len(text))])
	})
}

func TestFileTypeDetection(t *testing.T) {
	tests := []struct {
		filename     string
		expectedType string
	}{
		{"document.txt", "text"},
		{"document.pdf", "pdf"},
		{"document.docx", "docx"},
		{"Document.DOCX", "docx"},
		{"document.doc", "text"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			ext := strings.ToLower(filepath.Ext(tt.filename))
			isPdf := ext == ".pdf"
			isDocx := ext == ".docx"

			var actualType string
			if isPdf {
				actualType = "pdf"
			} else if isDocx {
				actualType = "docx"
			} else {
				actualType = "text"
			}

			assert.Equal(t, tt.expectedType, actualType, "File type detection failed for %s", tt.filename)
		})
	}
}

func TestWordDocumentErrorHandling(t *testing.T) {
	logger := log.New(os.Stderr)
	logger.SetLevel(log.DebugLevel)

	textProcessor := &TextDocumentProcessor{
		logger: logger,
	}

	t.Run("NonExistentWordFile", func(t *testing.T) {
		_, err := textProcessor.ExtractTextFromWord("nonexistent.docx")
		assert.Error(t, err, "Should fail for non-existent file")
		assert.Contains(t, err.Error(), "failed to extract text from Word file", "Error should mention Word extraction failure")
	})

	t.Run("InvalidWordFile", func(t *testing.T) {
		tempFile := filepath.Join(t.TempDir(), "invalid.docx")
		err := os.WriteFile(tempFile, []byte("This is not a Word document"), 0o644)
		require.NoError(t, err)

		_, err = textProcessor.ExtractTextFromWord(tempFile)
		assert.Error(t, err, "Should fail for invalid Word file")
		assert.Contains(t, err.Error(), "failed to extract text from Word file", "Error should mention Word extraction failure")
	})
}

func TestPowerPointDocumentExtraction(t *testing.T) {
	testDir := t.TempDir()
	testPptxPath := filepath.Join(testDir, "test.pptx")
	testPptPath := filepath.Join(testDir, "test.ppt")

	integrationPptxFile := filepath.Join("..", "integration", "testdata", "misc", "test_presentation.pptx")
	integrationPptFile := filepath.Join("..", "integration", "testdata", "misc", "test_presentation.ppt")
	hasPptx := false
	hasPpt := false

	if _, err := os.Stat(integrationPptxFile); err == nil {
		input, err := os.ReadFile(integrationPptxFile)
		if err == nil {
			err = os.WriteFile(testPptxPath, input, 0o644)
			if err == nil {
				hasPptx = true
			}
		}
	}

	if _, err := os.Stat(integrationPptFile); err == nil {
		input, err := os.ReadFile(integrationPptFile)
		if err == nil {
			err = os.WriteFile(testPptPath, input, 0o644)
			if err == nil {
				hasPpt = true
			}
		}
	}

	logger := log.New(os.Stderr)
	logger.SetLevel(log.DebugLevel)

	textProcessor := &TextDocumentProcessor{
		logger: logger,
	}

	t.Run("ExtractTextFromPPTX", func(t *testing.T) {
		if !hasPptx {
			t.Skip("PPTX test file not available, skipping PowerPoint extraction test")
		}

		text, err := textProcessor.ExtractTextFromPPTX(testPptxPath)
		require.NoError(t, err, "Should extract text from PPTX document")

		assert.NotEmpty(t, text, "Extracted text should not be empty")
		assert.True(t, len(strings.TrimSpace(text)) > 0, "Should extract meaningful text")

		assert.NotContains(t, text, "<?xml", "Should not contain XML headers")
		assert.NotContains(t, text, "<p:", "Should not contain PowerPoint XML tags")
		assert.NotContains(t, text, "<a:", "Should not contain drawing XML tags")
		assert.NotContains(t, text, "\x00", "Should not contain null bytes")

		t.Logf("Extracted PPTX text length: %d characters", len(text))
		t.Logf("PPTX text preview: %s", text[:min(200, len(text))])
	})

	t.Run("ExtractTextFromPPT", func(t *testing.T) {
		if !hasPpt {
			t.Skip("PPT test file not available, skipping legacy PowerPoint extraction test")
		}

		text, err := textProcessor.ExtractTextFromPPT(testPptPath)
		require.NoError(t, err, "Should extract text from PPT document")

		assert.NotEmpty(t, text, "Extracted text should not be empty")
		assert.True(t, len(strings.TrimSpace(text)) > 0, "Should extract meaningful text")

		t.Logf("Extracted PPT text length: %d characters", len(text))
		t.Logf("PPT text preview: %s", text[:min(200, len(text))])
	})

	t.Run("ProcessPowerPointFile", func(t *testing.T) {
		if !hasPptx {
			t.Skip("PPTX test file not available, skipping PowerPoint file processing test")
		}

		// Test that we can at least detect the file type correctly
		file, err := os.Open(testPptxPath)
		require.NoError(t, err)
		defer func() {
			if err := file.Close(); err != nil {
				t.Logf("Failed to close file: %v", err)
			}
		}()

		fileInfo, err := file.Stat()
		require.NoError(t, err)

		ext := strings.ToLower(filepath.Ext(fileInfo.Name()))
		assert.True(t, ext == ".pptx" || ext == ".ppt", "Should detect PowerPoint file extension")

		// Test text extraction specifically
		text, err := textProcessor.ExtractTextFromPPTX(testPptxPath)
		require.NoError(t, err)
		assert.NotEmpty(t, text, "Should extract meaningful content")

		t.Logf("PowerPoint file processed successfully with %d characters", len(text))
	})
}

func TestPowerPointDocumentErrorHandling(t *testing.T) {
	logger := log.New(os.Stderr)
	logger.SetLevel(log.DebugLevel)

	textProcessor := &TextDocumentProcessor{
		logger: logger,
	}

	t.Run("NonExistentPPTXFile", func(t *testing.T) {
		_, err := textProcessor.ExtractTextFromPPTX("/nonexistent/file.pptx")
		assert.Error(t, err, "Should fail for non-existent PPTX file")
		assert.Contains(t, err.Error(), "failed to open PPTX file", "Error should mention file opening failure")
	})

	t.Run("NonExistentPPTFile", func(t *testing.T) {
		_, err := textProcessor.ExtractTextFromPPT("/nonexistent/file.ppt")
		assert.Error(t, err, "Should fail for non-existent PPT file")
		assert.Contains(t, err.Error(), "failed to open PPT file", "Error should mention file opening failure")
	})

	t.Run("InvalidPPTXFile", func(t *testing.T) {
		tempFile := filepath.Join(os.TempDir(), "invalid.pptx")
		err := os.WriteFile(tempFile, []byte("This is not a PowerPoint document"), 0o644)
		require.NoError(t, err)
		defer func() {
			if err := os.Remove(tempFile); err != nil {
				t.Logf("Failed to remove temp file: %v", err)
			}
		}()

		_, err = textProcessor.ExtractTextFromPPTX(tempFile)
		assert.Error(t, err, "Should fail for invalid PPTX file")
	})

	t.Run("InvalidPPTFile", func(t *testing.T) {
		tempFile := filepath.Join(os.TempDir(), "invalid.ppt")
		err := os.WriteFile(tempFile, []byte("This is not a PowerPoint document"), 0o644)
		require.NoError(t, err)
		defer func() {
			if err := os.Remove(tempFile); err != nil {
				t.Logf("Failed to remove temp file: %v", err)
			}
		}()

		_, err = textProcessor.ExtractTextFromPPT(tempFile)
		assert.Error(t, err, "Should fail for invalid PPT file")
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

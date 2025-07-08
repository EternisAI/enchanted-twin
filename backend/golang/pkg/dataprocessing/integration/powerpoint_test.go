package integration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/misc"
)

func TestPowerPointProcessing(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected struct {
			shouldSucceed    bool
			minContentLen    int
			expectedType     string
			shouldContain    []string
			shouldNotContain []string
		}
	}{
		{
			name:     "ProcessPPTXDocument",
			filename: "test_presentation.pptx",
			expected: struct {
				shouldSucceed    bool
				minContentLen    int
				expectedType     string
				shouldContain    []string
				shouldNotContain []string
			}{
				shouldSucceed:    true,
				minContentLen:    10,
				expectedType:     "pptx",
				shouldContain:    []string{},
				shouldNotContain: []string{"<", ">", "<?xml"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			env := SetupTestEnvironment(t)
			defer env.Cleanup(t)

			processor, err := misc.NewTextDocumentProcessor(env.store, env.logger)
			require.NoError(t, err)

			testFilePath := filepath.Join("testdata", "synced-document", test.filename)

			if _, err := os.Stat(testFilePath); os.IsNotExist(err) {
				t.Skip("Test PowerPoint file not found, skipping test")
			}

			ctx := context.Background()
			documents, err := processor.ProcessFile(ctx, testFilePath)

			if test.expected.shouldSucceed {
				require.NoError(t, err, "Should process PowerPoint file successfully")
				require.NotEmpty(t, documents, "Should produce documents")

				firstDoc := documents[0]

				assert.Equal(t, "synced-document", firstDoc.FieldSource)
				assert.Contains(t, firstDoc.FieldContent, "content")
				assert.Contains(t, firstDoc.FieldMetadata, "filename")
				assert.Contains(t, firstDoc.FieldMetadata, "type")

				docType := firstDoc.FieldMetadata["type"]
				assert.Equal(t, test.expected.expectedType, docType)

				filename := firstDoc.FieldMetadata["filename"]
				assert.Equal(t, test.filename, filename)

				content := firstDoc.FieldContent
				assert.GreaterOrEqual(t, len(content), test.expected.minContentLen)

				for _, required := range test.expected.shouldContain {
					assert.Contains(t, content, required, "Content should contain required text")
				}

				for _, forbidden := range test.expected.shouldNotContain {
					assert.NotContains(t, content, forbidden, "Content should not contain XML tags")
				}

				t.Logf("Extracted content length: %d characters", len(content))
				if len(content) > 0 {
					preview := content
					if len(preview) > 200 {
						preview = preview[:200] + "..."
					}
					t.Logf("Content preview: %s", preview)
				}
			} else {
				assert.Error(t, err, "Should fail to process invalid file")
			}
		})
	}
}

func TestPowerPointTextExtraction(t *testing.T) {
	env := SetupTestEnvironment(t)
	defer env.Cleanup(t)

	textProcessor, err := misc.NewTextDocumentProcessor(env.store, env.logger)
	require.NoError(t, err)

	t.Run("ExtractTextFromPPTX", func(t *testing.T) {
		testFilePath := filepath.Join("testdata", "synced-document", "test_presentation.pptx")

		if _, err := os.Stat(testFilePath); os.IsNotExist(err) {
			t.Skip("Test PPTX file not found, skipping extraction test")
		}

		text, err := textProcessor.ExtractTextFromPPTX(testFilePath)
		require.NoError(t, err, "Should extract text from PPTX document")

		assert.NotEmpty(t, text, "Extracted text should not be empty")
		assert.True(t, len(strings.TrimSpace(text)) > 0, "Should extract meaningful text")

		assert.NotContains(t, text, "<?xml", "Should not contain XML headers")
		assert.NotContains(t, text, "<p:", "Should not contain PowerPoint XML tags")
		assert.NotContains(t, text, "<a:", "Should not contain drawing XML tags")

		t.Logf("Extracted text length: %d characters", len(text))
		t.Logf("Text preview: %s", text[:min(200, len(text))])
	})

	t.Run("ExtractTextFromPPT", func(t *testing.T) {
		testFilePath := filepath.Join("testdata", "synced-document", "test_presentation.ppt")

		if _, err := os.Stat(testFilePath); os.IsNotExist(err) {
			t.Skip("Test PPT file not found, skipping extraction test")
		}

		text, err := textProcessor.ExtractTextFromPPT(testFilePath)
		require.NoError(t, err, "Should extract text from PPT document")

		assert.NotEmpty(t, text, "Extracted text should not be empty")
		assert.True(t, len(strings.TrimSpace(text)) > 0, "Should extract meaningful text")

		t.Logf("Extracted text length: %d characters", len(text))
		t.Logf("Text preview: %s", text[:min(200, len(text))])
	})
}

func TestPowerPointErrorHandling(t *testing.T) {
	logger := log.New(os.Stderr)
	logger.SetLevel(log.DebugLevel)

	textProcessor := &misc.TextDocumentProcessor{}

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

func TestPowerPointIntegration(t *testing.T) {
	env := SetupTestEnvironment(t)
	defer env.Cleanup(t)

	processor, err := misc.NewTextDocumentProcessor(env.store, env.logger)
	require.NoError(t, err)

	testFilePath := filepath.Join("testdata", "synced-document", "test_presentation.pptx")

	if _, err := os.Stat(testFilePath); os.IsNotExist(err) {
		t.Skip("Test PowerPoint file not found, skipping integration test")
	}

	ctx := context.Background()

	textDocuments, err := processor.ProcessFile(ctx, testFilePath)
	require.NoError(t, err)
	require.NotEmpty(t, textDocuments)

	doc := textDocuments[0]
	assert.NotEmpty(t, doc.FieldID)
	assert.NotEmpty(t, doc.FieldContent)
	assert.Equal(t, "misc", doc.FieldSource)

	t.Logf("Document ID: %s", doc.FieldID)
	t.Logf("Document content length: %d", len(doc.FieldContent))
}

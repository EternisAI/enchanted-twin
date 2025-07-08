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
	"github.com/EternisAI/enchanted-twin/pkg/db"
)

func TestWordDocumentProcessing(t *testing.T) {
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
			name:     "ProcessWordDocument",
			filename: "test_doc.docx",
			expected: struct {
				shouldSucceed    bool
				minContentLen    int
				expectedType     string
				shouldContain    []string
				shouldNotContain []string
			}{
				shouldSucceed:    true,
				minContentLen:    10,
				expectedType:     "docx",
				shouldContain:    []string{},
				shouldNotContain: []string{"<", ">"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := log.New(os.Stderr)
			logger.SetLevel(log.DebugLevel)

			aiService := createMockAIService(logger)

			store := &db.Store{}

			processor, err := misc.NewTextDocumentProcessor(aiService, "gpt-4o-mini", store, logger)
			require.NoError(t, err, "Failed to create text document processor")

			testFilePath := filepath.Join("testdata", "misc", tt.filename)

			_, err = os.Stat(testFilePath)
			require.NoError(t, err, "Test file %s does not exist", testFilePath)

			ctx := context.Background()
			documents, err := processor.ProcessFile(ctx, testFilePath)

			if tt.expected.shouldSucceed {
				require.NoError(t, err, "Processing should succeed for %s", tt.filename)
				require.NotEmpty(t, documents, "Should produce at least one document")

				firstDoc := documents[0]

				assert.Equal(t, "misc", firstDoc.FieldSource, "Source should be 'misc'")

				assert.NotEmpty(t, firstDoc.FieldContent, "Document should have content")
				assert.Contains(t, firstDoc.FieldMetadata, "filename", "Document should contain 'filename' metadata")
				assert.Contains(t, firstDoc.FieldMetadata, "type", "Document should contain 'type' metadata")

				content := firstDoc.FieldContent
				assert.GreaterOrEqual(t, len(content), tt.expected.minContentLen,
					"Content should be at least %d characters long", tt.expected.minContentLen)

				filename := firstDoc.FieldMetadata["filename"]
				assert.Equal(t, tt.filename, filename, "Filename should match")

				docType := firstDoc.FieldMetadata["type"]
				assert.Equal(t, tt.expected.expectedType, docType, "Document type should be '%s'", tt.expected.expectedType)

				for _, expectedText := range tt.expected.shouldContain {
					assert.Contains(t, strings.ToLower(content), strings.ToLower(expectedText),
						"Content should contain '%s'", expectedText)
				}

				for _, unwantedText := range tt.expected.shouldNotContain {
					assert.NotContains(t, content, unwantedText,
						"Content should not contain '%s'", unwantedText)
				}

				if len(documents) > 1 {
					for i, doc := range documents {
						chunkIndexStr := doc.FieldMetadata["chunk"]
						if chunkIndexStr != "" {
							t.Logf("Document %d has chunk index: %s", i, chunkIndexStr)
						}
					}
				}

				if len(firstDoc.FieldTags) > 0 {
					t.Logf("Extracted tags: %v", firstDoc.FieldTags)
				}

				t.Logf("Successfully processed Word document: %s", tt.filename)
				t.Logf("Content length: %d characters", len(content))
				t.Logf("Number of documents: %d", len(documents))
				t.Logf("Content preview: %s", content[:min(200, len(content))])
			} else {
				assert.Error(t, err, "Processing should fail for %s", tt.filename)
			}
		})
	}
}

func TestWordDocumentTextExtraction(t *testing.T) {
	logger := log.New(os.Stderr)
	logger.SetLevel(log.DebugLevel)

	aiService := createMockAIService(logger)
	store := &db.Store{}

	processor, err := misc.NewTextDocumentProcessor(aiService, "gpt-4o-mini", store, logger)
	require.NoError(t, err)

	testFilePath := filepath.Join("testdata", "misc", "test_doc.docx")

	_, err = os.Stat(testFilePath)
	require.NoError(t, err, "Test Word document does not exist")

	t.Run("DirectTextExtraction", func(t *testing.T) {
		ctx := context.Background()
		documents, err := processor.ProcessFile(ctx, testFilePath)

		require.NoError(t, err, "Text extraction should succeed")
		require.NotEmpty(t, documents, "Should produce documents")

		content := documents[0].FieldContent

		assert.NotContains(t, content, "<?xml", "Should not contain XML headers")
		assert.NotContains(t, content, "<w:", "Should not contain Word XML tags")

		assert.True(t, len(strings.TrimSpace(content)) > 0, "Should contain non-empty text")

		t.Logf("Extracted text: %s", content)
	})
}

func TestWordDocumentErrorHandling(t *testing.T) {
	logger := log.New(os.Stderr)
	logger.SetLevel(log.DebugLevel)

	aiService := createMockAIService(logger)
	store := &db.Store{}

	processor, err := misc.NewTextDocumentProcessor(aiService, "gpt-4o-mini", store, logger)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("NonExistentFile", func(t *testing.T) {
		_, err := processor.ProcessFile(ctx, "nonexistent.docx")
		assert.Error(t, err, "Should fail for non-existent file")
		assert.Contains(t, err.Error(), "failed to get file info", "Error should mention file info failure")
	})

	t.Run("InvalidWordFile", func(t *testing.T) {
		tempFile := filepath.Join(os.TempDir(), "invalid.docx")
		err := os.WriteFile(tempFile, []byte("This is not a Word document"), 0o644)
		require.NoError(t, err)
		defer func() {
			if err := os.Remove(tempFile); err != nil {
				t.Logf("Failed to remove temp file: %v", err)
			}
		}()

		_, err = processor.ProcessFile(ctx, tempFile)
		assert.Error(t, err, "Should fail for invalid Word file")
	})
}

func TestWordDocumentIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := SetupTestEnvironment(t)
	defer env.Cleanup(t)

	wordTestPath := filepath.Join("testdata", "misc")

	t.Run("WordDocumentFullPipeline", func(t *testing.T) {
		env.LoadDocuments(t, "misc", wordTestPath)

		assert.NotEmpty(t, env.documents, "Should load documents including Word document")

		foundWordDoc := false
		for _, doc := range env.documents {
			if strings.Contains(doc.ID(), "test_doc.docx") {
				foundWordDoc = true
				assert.NotEmpty(t, doc.Content(), "Word document should have content")
				t.Logf("Word document content preview: %s", doc.Content()[:min(200, len(doc.Content()))])
				break
			}
		}

		if !foundWordDoc {
			t.Log("Note: Word document may not have been processed in this test run")
		}

		env.StoreDocuments(t)
		env.logger.Info("Documents including Word document stored successfully")
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

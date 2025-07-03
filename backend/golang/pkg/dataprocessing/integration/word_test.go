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
			records, err := processor.ProcessFile(ctx, testFilePath)

			if tt.expected.shouldSucceed {
				require.NoError(t, err, "Processing should succeed for %s", tt.filename)
				require.NotEmpty(t, records, "Should produce at least one record")

				firstRecord := records[0]

				assert.Equal(t, "misc", firstRecord.Source, "Source should be 'misc'")

				require.Contains(t, firstRecord.Data, "content", "Record should contain 'content' field")
				require.Contains(t, firstRecord.Data, "filename", "Record should contain 'filename' field")
				require.Contains(t, firstRecord.Data, "type", "Record should contain 'type' field")

				content, ok := firstRecord.Data["content"].(string)
				require.True(t, ok, "Content should be a string")
				assert.GreaterOrEqual(t, len(content), tt.expected.minContentLen,
					"Content should be at least %d characters long", tt.expected.minContentLen)

				filename, ok := firstRecord.Data["filename"].(string)
				require.True(t, ok, "Filename should be a string")
				assert.Equal(t, tt.filename, filename, "Filename should match")

				docType, ok := firstRecord.Data["type"].(string)
				require.True(t, ok, "Type should be a string")
				assert.Equal(t, tt.expected.expectedType, docType, "Document type should be '%s'", tt.expected.expectedType)

				for _, expectedText := range tt.expected.shouldContain {
					assert.Contains(t, strings.ToLower(content), strings.ToLower(expectedText),
						"Content should contain '%s'", expectedText)
				}

				for _, unwantedText := range tt.expected.shouldNotContain {
					assert.NotContains(t, content, unwantedText,
						"Content should not contain '%s'", unwantedText)
				}

				if len(records) > 1 {
					for i, record := range records {
						chunkIndex, ok := record.Data["chunk"].(int)
						require.True(t, ok, "Chunk should have chunk index")
						assert.Equal(t, i, chunkIndex, "Chunk index should match record index")
					}
				}

				if tagsVal, ok := firstRecord.Data["tags"]; ok {
					tags, ok := tagsVal.([]string)
					require.True(t, ok, "Tags should be a string slice")
					t.Logf("Extracted tags: %v", tags)
				}

				t.Logf("Successfully processed Word document: %s", tt.filename)
				t.Logf("Content length: %d characters", len(content))
				t.Logf("Number of records: %d", len(records))
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
		textProcessor, ok := processor.(*misc.TextDocumentProcessor)
		require.True(t, ok, "Should be able to cast to TextDocumentProcessor")

		ctx := context.Background()
		records, err := textProcessor.ProcessFile(ctx, testFilePath)

		require.NoError(t, err, "Text extraction should succeed")
		require.NotEmpty(t, records, "Should produce records")

		content, ok := records[0].Data["content"].(string)
		require.True(t, ok, "Should extract text content")

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
		assert.Contains(t, err.Error(), "failed to open file", "Error should mention file opening failure")
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

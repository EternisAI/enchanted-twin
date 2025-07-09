package jinaaiembedding

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJinaAIEmbeddingModel(t *testing.T) {
	t.Skip()

	appDataPath := os.Getenv("APP_DATA_PATH")
	assert.NotEmpty(t, appDataPath)

	sharedLibPath := os.Getenv("SHARED_LIBRARY_PATH")
	assert.NotEmpty(t, sharedLibPath)

	model, err := NewEmbedding(appDataPath, sharedLibPath)
	assert.NoError(t, err)
	defer model.Close()

	inputText := "This is an apple"
	vector, err := model.Embedding(context.Background(), inputText, "jina-embeddings-v2-base-en")
	assert.NoError(t, err)

	t.Logf("Embedding: %v", vector[:10])

	inputTexts := []string{
		"This is an apple",
		"This is a banana",
	}
	vectors, err := model.Embeddings(context.Background(), inputTexts, "jina-embeddings-v2-base-en")
	assert.NoError(t, err)
	assert.NotEmpty(t, vectors)
	assert.Len(t, vectors, len(inputTexts))

	for i, vector := range vectors {
		assert.NotEmpty(t, vector, "Vector %d should not be empty", i)
		assert.Len(t, vector, 768, "Vector %d should have 768 dimensions", i)
		t.Logf("Vector %d (first 10): %v", i, vector[:10])
	}
}

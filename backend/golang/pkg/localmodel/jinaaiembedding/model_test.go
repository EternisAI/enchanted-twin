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

	model, err := NewJinaAIEmbeddingModel(appDataPath, sharedLibPath)
	assert.NoError(t, err)
	defer model.Close()

	inputText := "This is an apple"
	vector, err := model.Embedding(context.Background(), inputText, "jina-embeddings-v2-base-en")
	assert.NoError(t, err)

	t.Logf("Embedding: %v", vector[:10])

	inputTexts := []string{}
	_, err = model.Embeddings(context.Background(), inputTexts, "jina-embeddings-v2-base-en")
	assert.NoError(t, err)
	// assert.NotEmpty(t, vectors)
}

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
	emb, err := model.Embedding(context.Background(), inputText, "jina-embeddings-v2-base-en")
	assert.NoError(t, err)

	t.Logf("Embedding: %v", emb[:10])
}

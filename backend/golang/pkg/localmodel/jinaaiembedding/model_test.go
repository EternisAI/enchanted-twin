package jinaaiembedding

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJinaAIEmbeddingModel(t *testing.T) {
	t.Skip()

	modelDir := "model/"
	onnxLib := "/usr/local/lib/onnxruntime/lib/libonnxruntime.so"

	model, err := NewJinaAIEmbeddingModel(modelDir, onnxLib)
	assert.NoError(t, err)
	defer model.Close()

	inputText := "This is an apple"
	emb, err := model.Embedding(context.Background(), inputText)
	assert.NoError(t, err)

	t.Logf("Embedding: %v", emb[:10])
}

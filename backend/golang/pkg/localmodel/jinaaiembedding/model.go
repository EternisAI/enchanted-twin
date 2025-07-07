package jinaaiembedding

import (
	"context"

	"github.com/EternisAI/enchanted-twin/pkg/localmodel"
)

var _ localmodel.EmbeddingModel = (*JinaAIEmbeddingModel)(nil)

type JinaAIEmbeddingModel struct {
	tokenizer *SentencePieceTokenizer
}

func NewJinaAIEmbeddingModel() *JinaAIEmbeddingModel {
	return &JinaAIEmbeddingModel{
		tokenizer: NewSentencePieceTokenizer(),
	}
}

func (m *JinaAIEmbeddingModel) Embedding(ctx context.Context, input string) ([]float32, error) {
	return nil, nil
}

func (m *JinaAIEmbeddingModel) Embeddings(ctx context.Context, inputs []string) ([][]float32, error) {
	return nil, nil
}

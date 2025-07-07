package jinaaiembedding

import "context"

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

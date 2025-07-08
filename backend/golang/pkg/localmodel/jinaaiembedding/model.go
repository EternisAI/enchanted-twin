package jinaaiembedding

import (
	"context"

	ort "github.com/yalue/onnxruntime_go"

	"github.com/EternisAI/enchanted-twin/pkg/localmodel"
)

var _ localmodel.EmbeddingModel = (*JinaAIEmbeddingModel)(nil)

type JinaAIEmbeddingModel struct {
	tokenizer *SentencePieceTokenizer
	session   *ort.DynamicAdvancedSession
}

func NewJinaAIEmbeddingModel(modelPath string) (*JinaAIEmbeddingModel, error) {
	// TODO
	// ort.SetSharedLibraryPath("/usr/local/lib/onnxruntime/libonnxruntime.dylib")

	err := ort.InitializeEnvironment()
	if err != nil {
		return nil, err
	}

	session, err := ort.NewDynamicAdvancedSession(modelPath,
		[]string{"input_ids", "attention_mask", "token_type_ids"},
		[]string{"last_hidden_state"}, nil)
	if err != nil {
		return nil, err
	}

	return &JinaAIEmbeddingModel{
		tokenizer: NewSentencePieceTokenizer(),
		session:   session,
	}, nil
}

func (m *JinaAIEmbeddingModel) Close() {
	if m.session != nil {
		m.session.Destroy()
	}
	ort.DestroyEnvironment()
}

func (m *JinaAIEmbeddingModel) Embedding(ctx context.Context, input string) ([]float32, error) {
	return nil, nil
}

func (m *JinaAIEmbeddingModel) Embeddings(ctx context.Context, inputs []string) ([][]float32, error) {
	return nil, nil
}

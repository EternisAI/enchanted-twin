package jinaaiembedding

import (
	"context"
	"math"
	"path/filepath"

	ort "github.com/yalue/onnxruntime_go"

	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

var _ ai.Embedding = (*JinaAIEmbeddingModel)(nil)

type JinaAIEmbeddingModel struct {
	tokenizer *SentencePieceTokenizer
	session   *ort.DynamicAdvancedSession
}

func NewJinaAIEmbeddingModel(appDataPath string, sharedLibraryPath string) (*JinaAIEmbeddingModel, error) {
	modelDir := filepath.Join(appDataPath, "models", "jina-embeddings-v2-base-en")
	tokenizerPath := filepath.Join(modelDir, "tokenizer.json")
	configPath := filepath.Join(modelDir, "config.json")
	modelPath := filepath.Join(modelDir, "model.onnx")
	onnxLibPath := filepath.Join(sharedLibraryPath, "onnxruntime-linux-x64-1.22.0", "lib", "libonnxruntime.so")

	tk := NewSentencePieceTokenizer()
	err := tk.LoadFromLocal(tokenizerPath, configPath)
	if err != nil {
		return nil, err
	}

	ort.SetSharedLibraryPath(onnxLibPath)

	err = ort.InitializeEnvironment()
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
		tokenizer: tk,
		session:   session,
	}, nil
}

func (m *JinaAIEmbeddingModel) Close() {
	if m.session != nil {
		_ = m.session.Destroy()
	}
	_ = ort.DestroyEnvironment()
}

func (m *JinaAIEmbeddingModel) Embedding(ctx context.Context, input string, model string) ([]float64, error) {
	inputIds, attentionMask := m.tokenizer.Encode(input)

	tokenTypeIds := make([]int64, len(inputIds))
	for i := range tokenTypeIds {
		tokenTypeIds[i] = 0
	}

	batchSize := 1
	seqLen := len(inputIds)
	embedDim := 768

	inputIdsShape := ort.NewShape(int64(batchSize), int64(seqLen))
	inputIdsTensor, err := ort.NewTensor(inputIdsShape, inputIds)
	if err != nil {
		return nil, err
	}
	defer func() { _ = inputIdsTensor.Destroy() }()

	attentionMaskShape := ort.NewShape(int64(batchSize), int64(seqLen))
	attentionMaskTensor, err := ort.NewTensor(attentionMaskShape, attentionMask)
	if err != nil {
		return nil, err
	}
	defer func() { _ = attentionMaskTensor.Destroy() }()

	tokenTypeIdsShape := ort.NewShape(int64(batchSize), int64(seqLen))
	tokenTypeIdsTensor, err := ort.NewTensor(tokenTypeIdsShape, tokenTypeIds)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tokenTypeIdsTensor.Destroy() }()

	outputShape := ort.NewShape(int64(batchSize), int64(seqLen), int64(embedDim))
	outputTensor, err := ort.NewEmptyTensor[float32](outputShape)
	if err != nil {
		return nil, err
	}
	defer func() { _ = outputTensor.Destroy() }()

	err = m.session.Run([]ort.Value{inputIdsTensor, attentionMaskTensor, tokenTypeIdsTensor}, []ort.Value{outputTensor})
	if err != nil {
		return nil, err
	}

	rawOutput := outputTensor.GetData()
	pooledEmbeddings := meanPooling(rawOutput, attentionMask, batchSize, seqLen, embedDim)
	finalEmbeddings := l2Normalize(pooledEmbeddings, batchSize, embedDim)

	// Convert to float64 for interface compliance
	result := make([]float64, len(finalEmbeddings))
	for i, v := range finalEmbeddings {
		result[i] = float64(v)
	}

	return result, nil
}

func (m *JinaAIEmbeddingModel) Embeddings(ctx context.Context, inputs []string, model string) ([][]float64, error) {
	// TODO implement me
	return nil, nil
}

func meanPooling(modelOutput []float32, attentionMask []int64, batchSize, seqLen, embedDim int) []float32 {
	result := make([]float32, batchSize*embedDim)

	for b := 0; b < batchSize; b++ {
		var sumMask float32
		for i := 0; i < embedDim; i++ {
			var sumEmbedding float32
			for s := 0; s < seqLen; s++ {
				maskVal := float32(attentionMask[b*seqLen+s])
				embeddingVal := modelOutput[b*seqLen*embedDim+s*embedDim+i]
				sumEmbedding += embeddingVal * maskVal
				if i == 0 {
					sumMask += maskVal
				}
			}
			if sumMask < 1e-9 {
				sumMask = 1e-9
			}
			result[b*embedDim+i] = sumEmbedding / sumMask
		}
	}
	return result
}

func l2Normalize(embeddings []float32, batchSize, embedDim int) []float32 {
	result := make([]float32, len(embeddings))

	for b := 0; b < batchSize; b++ {
		var norm float32
		for i := 0; i < embedDim; i++ {
			val := embeddings[b*embedDim+i]
			norm += val * val
		}
		norm = float32(math.Sqrt(float64(norm)))

		for i := 0; i < embedDim; i++ {
			result[b*embedDim+i] = embeddings[b*embedDim+i] / norm
		}
	}
	return result
}

package jinaaiembedding

import (
	"context"
	"math"
	"path/filepath"
	"runtime"

	ort "github.com/yalue/onnxruntime_go"

	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

var _ ai.Embedding = (*JinaAIEmbeddingModel)(nil)

type JinaAIEmbeddingModel struct {
	tokenizer *SentencePieceTokenizer
	session   *ort.DynamicAdvancedSession
}

func NewEmbedding(appDataPath string, sharedLibraryPath string) (*JinaAIEmbeddingModel, error) {
	modelDir := filepath.Join(appDataPath, "models", "jina-embeddings-v2-base-en")
	tokenizerPath := filepath.Join(modelDir, "tokenizer.json")
	configPath := filepath.Join(modelDir, "config.json")
	modelPath := filepath.Join(modelDir, "model.onnx")
	onnxLibPath := getONNXLibraryPath(sharedLibraryPath)

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
	results := make([][]float64, len(inputs))

	for i, input := range inputs {
		embedding, err := m.Embedding(ctx, input, model)
		if err != nil {
			return nil, err
		}
		results[i] = embedding
	}

	return results, nil
}

func meanPooling(modelOutput []float32, attentionMask []int64, batchSize, seqLen, embedDim int) []float32 {
	result := make([]float32, batchSize*embedDim)

	for b := range batchSize {
		var sumMask float32
		for i := range embedDim {
			var sumEmbedding float32
			for s := range seqLen {
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

	for b := range batchSize {
		var norm float32
		for i := range embedDim {
			val := embeddings[b*embedDim+i]
			norm += val * val
		}
		norm = float32(math.Sqrt(float64(norm)))
		if norm < 1e-9 {
			norm = 1e-9
		}

		for i := range embedDim {
			result[b*embedDim+i] = embeddings[b*embedDim+i] / norm
		}
	}
	return result
}

func getONNXLibraryPath(sharedLibraryPath string) string {
	var libName string
	var platformDir string

	arch := "x64"
	if runtime.GOARCH == "arm64" {
		arch = "arm64"
	}

	switch runtime.GOOS {
	case "windows":
		libName = "onnxruntime.dll"
		platformDir = "onnxruntime-win-" + arch + "-1.22.0"
	case "darwin":
		libName = "libonnxruntime.dylib"
		platformDir = "onnxruntime-osx-" + arch + "-1.22.0"
	case "linux":
		libName = "libonnxruntime.so"
		if arch == "arm64" {
			// Upstream archive uses 'aarch64' for Linux arm64 builds
			platformDir = "onnxruntime-linux-aarch64-1.22.0"
		} else {
			platformDir = "onnxruntime-linux-" + arch + "-1.22.0"
		}
	default:
		libName = "libonnxruntime.so"
		platformDir = "onnxruntime-linux-" + arch + "-1.22.0"
	}

	return filepath.Join(sharedLibraryPath, platformDir, "lib", libName)
}

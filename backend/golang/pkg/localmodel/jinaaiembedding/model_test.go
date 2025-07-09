package jinaaiembedding

import (
	"context"
	"math"
	"os"
	"sort"
	"testing"

	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/charmbracelet/log"
	"github.com/stretchr/testify/assert"
)

func TestJinaAIEmbeddingModel(t *testing.T) {
	appDataPath := os.Getenv("APP_DATA_PATH")
	if appDataPath == "" {
		t.Skip("APP_DATA_PATH environment variable not set")
	}

	sharedLibPath := os.Getenv("SHARED_LIBRARY_PATH")
	if sharedLibPath == "" {
		t.Skip("SHARED_LIBRARY_PATH environment variable not set")
	}

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

func cosineSimilarity(a, b []float64) float64 {
	var dotProduct, normA, normB float64
	for i := range len(a) {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

func TestCosineSimilarityWithVariousWords(t *testing.T) {
	appDataPath := os.Getenv("APP_DATA_PATH")
	if appDataPath == "" {
		t.Skip("APP_DATA_PATH environment variable not set")
	}

	sharedLibPath := os.Getenv("SHARED_LIBRARY_PATH")
	if sharedLibPath == "" {
		t.Skip("SHARED_LIBRARY_PATH environment variable not set")
	}

	model, err := NewEmbedding(appDataPath, sharedLibPath)
	assert.NoError(t, err)
	defer model.Close()

	words := []string{
		"cat", "dog", "kitty", "puppy", "feline", "canine",
		"automobile", "spaceship", "mathematics", "quantum", "refrigerator",
		"bureaucracy", "molecule", "telescope", "archaeology", "philosophy",
	}
	vectors, err := model.Embeddings(context.Background(), words, "jina-embeddings-v2-base-en")
	assert.NoError(t, err)
	assert.Len(t, vectors, len(words))

	wordVectors := make(map[string][]float64)
	for i, word := range words {
		wordVectors[word] = vectors[i]
		assert.Len(t, vectors[i], 768, "Vector for '%s' should have 768 dimensions", word)
	}

	catKittySimilarity := cosineSimilarity(wordVectors["cat"], wordVectors["kitty"])
	dogPuppySimilarity := cosineSimilarity(wordVectors["dog"], wordVectors["puppy"])
	catFelineSimilarity := cosineSimilarity(wordVectors["cat"], wordVectors["feline"])
	dogCanineSimilarity := cosineSimilarity(wordVectors["dog"], wordVectors["canine"])

	catMathematicsSimilarity := cosineSimilarity(wordVectors["cat"], wordVectors["mathematics"])
	dogSpaceshipSimilarity := cosineSimilarity(wordVectors["dog"], wordVectors["spaceship"])
	kittyBureaucracySimilarity := cosineSimilarity(wordVectors["kitty"], wordVectors["bureaucracy"])
	puppyQuantumSimilarity := cosineSimilarity(wordVectors["puppy"], wordVectors["quantum"])

	t.Logf("High similarity pairs:")
	t.Logf("  cat-kitty: %.4f", catKittySimilarity)
	t.Logf("  dog-puppy: %.4f", dogPuppySimilarity)
	t.Logf("  cat-feline: %.4f", catFelineSimilarity)
	t.Logf("  dog-canine: %.4f", dogCanineSimilarity)

	t.Logf("Low similarity pairs:")
	t.Logf("  cat-mathematics: %.4f", catMathematicsSimilarity)
	t.Logf("  dog-spaceship: %.4f", dogSpaceshipSimilarity)
	t.Logf("  kitty-bureaucracy: %.4f", kittyBureaucracySimilarity)
	t.Logf("  puppy-quantum: %.4f", puppyQuantumSimilarity)

	// Assert that semantically similar words have higher similarity
	assert.Greater(t, catKittySimilarity, catMathematicsSimilarity,
		"cat-kitty similarity should be greater than cat-mathematics")
	assert.Greater(t, dogPuppySimilarity, dogSpaceshipSimilarity,
		"dog-puppy similarity should be greater than dog-spaceship")
	assert.Greater(t, catFelineSimilarity, kittyBureaucracySimilarity,
		"cat-feline similarity should be greater than kitty-bureaucracy")
	assert.Greater(t, dogCanineSimilarity, puppyQuantumSimilarity,
		"dog-canine similarity should be greater than puppy-quantum")

	catKittySim := cosineSimilarity(wordVectors["cat"], wordVectors["kitty"])
	dogKittySim := cosineSimilarity(wordVectors["dog"], wordVectors["kitty"])
	assert.Greater(t, catKittySim, dogKittySim,
		"kitty should be more similar to cat than to dog")
}

func TestCompareOpenAIEmbeddings(t *testing.T) {
	appDataPath := os.Getenv("APP_DATA_PATH")
	if appDataPath == "" {
		t.Skip("APP_DATA_PATH environment variable not set")
	}

	sharedLibPath := os.Getenv("SHARED_LIBRARY_PATH")
	if sharedLibPath == "" {
		t.Skip("SHARED_LIBRARY_PATH environment variable not set")
	}

	localModel, err := NewEmbedding(appDataPath, sharedLibPath)
	assert.NoError(t, err)
	defer localModel.Close()

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY environment variable not set")
	}

	logger := log.New(os.Stdout)
	openaiService := ai.NewOpenAIService(logger, apiKey, "https://api.openai.com/v1")

	words := []string{"bank", "money", "river", "finance", "tree", "loan"}

	t.Logf("Getting embeddings from JinaAI local model...")
	jinaVectors, err := localModel.Embeddings(context.Background(), words, "jina-embeddings-v2-base-en")
	assert.NoError(t, err)
	assert.Len(t, jinaVectors, len(words))

	t.Logf("Getting embeddings from OpenAI...")
	openaiVectors, err := openaiService.Embeddings(context.Background(), words, "text-embedding-3-small")
	assert.NoError(t, err)
	assert.Len(t, openaiVectors, len(words))

	// Create maps for easier access
	jinaWordVectors := make(map[string][]float64)
	openaiWordVectors := make(map[string][]float64)
	for i, word := range words {
		jinaWordVectors[word] = jinaVectors[i]
		openaiWordVectors[word] = openaiVectors[i]
	}

	type SimilarityResult struct {
		word        string
		jinaScore   float64
		openaiScore float64
	}

	// Calculate similarities of all words with "bank"
	var results []SimilarityResult
	bankJinaVector := jinaWordVectors["bank"]
	bankOpenaiVector := openaiWordVectors["bank"]

	for _, word := range words {
		if word == "bank" {
			continue // Skip comparing bank with itself
		}
		results = append(results, SimilarityResult{
			word:        word,
			jinaScore:   cosineSimilarity(bankJinaVector, jinaWordVectors[word]),
			openaiScore: cosineSimilarity(bankOpenaiVector, openaiWordVectors[word]),
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].jinaScore > results[j].jinaScore
	})

	t.Logf("\nJinaAI Local Model Similarities with 'bank' (ordered by score):")
	for _, result := range results {
		t.Logf("  bank-%s: %.4f", result.word, result.jinaScore)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].openaiScore > results[j].openaiScore
	})

	t.Logf("\nOpenAI Similarities with 'bank' (ordered by score):")
	for _, result := range results {
		t.Logf("  bank-%s: %.4f", result.word, result.openaiScore)
	}

	jinaScores := make(map[string]float64)
	openaiScores := make(map[string]float64)
	for _, result := range results {
		jinaScores[result.word] = result.jinaScore
		openaiScores[result.word] = result.openaiScore
	}

	// Test that bank's financial context words have higher similarity than unrelated words
	assert.Greater(t, jinaScores["money"], jinaScores["tree"], "JinaAI: bank-money should be more similar than bank-tree")
	assert.Greater(t, jinaScores["finance"], jinaScores["tree"], "JinaAI: bank-finance should be more similar than bank-tree")
	assert.Greater(t, jinaScores["loan"], jinaScores["tree"], "JinaAI: bank-loan should be more similar than bank-tree")
	assert.Greater(t, openaiScores["money"], openaiScores["tree"], "OpenAI: bank-money should be more similar than bank-tree")
	assert.Greater(t, openaiScores["finance"], openaiScores["tree"], "OpenAI: bank-finance should be more similar than bank-tree")
	assert.Greater(t, openaiScores["loan"], openaiScores["tree"], "OpenAI: bank-loan should be more similar than bank-tree")

	t.Logf("\nModel Comparison:")
	t.Logf("  JinaAI vector dimensions: %d", len(jinaVectors[0]))
	t.Logf("  OpenAI vector dimensions: %d", len(openaiVectors[0]))
}

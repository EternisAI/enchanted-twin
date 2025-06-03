package evolvingmemory

import (
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
	"github.com/joho/godotenv"

	"github.com/EternisAI/enchanted-twin/pkg/ai"
)

// createTestAIServices creates separate AI services for completions and embeddings.
func createTestAIServices() (*ai.Service, *ai.Service) {
	envPath := filepath.Join("..", "..", "..", "..", ".env")
	err := godotenv.Load(envPath)
	if err != nil {
		log.Default().Warn("Failed to load .env file", "path", envPath, "error", err)
	}

	completionsKey := os.Getenv("COMPLETIONS_API_KEY")
	if completionsKey == "" {
		return nil, nil
	}

	completionsURL := os.Getenv("COMPLETIONS_API_URL")
	if completionsURL == "" {
		completionsURL = "https://api.openai.com/v1"
	}

	embeddingsKey := os.Getenv("EMBEDDINGS_API_KEY")
	if embeddingsKey == "" {
		embeddingsKey = completionsKey
	}

	embeddingsURL := os.Getenv("EMBEDDINGS_API_URL")
	if embeddingsURL == "" {
		embeddingsURL = "https://api.openai.com/v1"
	}

	log.Default().Debug("Creating separate AI services",
		"completions_url", completionsURL,
		"embeddings_url", embeddingsURL)

	completionsService := ai.NewOpenAIService(log.Default(), completionsKey, completionsURL)
	embeddingsService := ai.NewOpenAIService(log.Default(), embeddingsKey, embeddingsURL)

	return completionsService, embeddingsService
}

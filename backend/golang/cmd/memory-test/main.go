package main

import (
	"fmt"
	"log"

	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/integration"
	"github.com/google/uuid"
)

func main() {
	source := "telegram" // see pkg dataprocessing for supported sources
	inputPath := "data_input/telegram_export.json.zip"

	completionsModel := "gpt-4o-mini"
	completionsApiUrl := "https://openrouter.ai/api/v1"
	completionsApiKey := "<your-openai-api-key>"

	embeddingModel := "text-embedding-3-small"
	embeddingApiKey := "<your-openai-api-key>"
	embeddingsApiUrl := "https://api.openai.com/v1"

	id := uuid.New().String()
	outputPath := fmt.Sprintf(
		"%s/%s_%s.jsonl",
		"./output",
		source,
		id,
	)

	if err := integration.IntegrationTest(integration.IntegrationTestConfig{
		Source:            source,
		InputPath:         inputPath,
		OutputPath:        outputPath,
		CompletionsModel:  completionsModel,
		CompletionsApiUrl: completionsApiUrl,
		CompletionsApiKey: completionsApiKey,
		EmbeddingsModel:   embeddingModel,
		EmbeddingsApiKey:  embeddingApiKey,
		EmbeddingsApiUrl:  embeddingsApiUrl,
	}); err != nil {
		log.Fatal("Integration test failed:", err)
	}
	log.Println("Integration test completed successfully")
}

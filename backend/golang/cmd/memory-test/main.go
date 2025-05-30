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
	// embeddingModel := "text-embedding-3-small"

	embeddingApiKey := "<your-openai-api-key>"
	completionsApiKey := "<your-openai-api-key>"

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
		EmbeddingsApiKey:  embeddingApiKey,
		CompletionsApiKey: completionsApiKey,
	}); err != nil {
		log.Fatal("Integration test failed:", err)
	}
	log.Println("Integration test completed successfully")
}

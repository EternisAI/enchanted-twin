package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/uuid"

	"github.com/EternisAI/enchanted-twin/pkg/config"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/integration"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("Received signal %v, initiating graceful shutdown...", sig)
		cancel()
	}()

	envs, err := config.LoadConfig(false)
	if err != nil {
		log.Fatal("Error loading config:", err)
	}

	source := "chatgpt" // telegram, gmail, chatgpt, slack, misc
	inputPath := "cmd/memory-test/sample-data/chatgpt.zip"

	completionsModel := "gpt-4o-mini"
	completionsApiUrl := "https://openrouter.ai/api/v1"

	completionsApiKey := envs.CompletionsAPIKey

	embeddingModel := "text-embedding-3-small"
	embeddingApiKey := envs.EmbeddingsAPIKey
	embeddingsApiUrl := "https://api.openai.com/v1"

	id := uuid.New().String()
	outputPath := fmt.Sprintf(
		"%s/%s_%s.jsonl",
		"./output",
		source,
		id,
	)

	if err := integration.IntegrationTestMemory(ctx, integration.IntegrationTestMemoryConfig{
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
		if err == context.Canceled {
			log.Println("Integration test was canceled")
			return
		}
		log.Fatal("Integration test failed:", err)
	}
	log.Println("Integration test completed successfully")
}

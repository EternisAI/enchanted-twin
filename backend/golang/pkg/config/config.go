package config

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	OpenAIAPIKey       string
	GraphqlPort        string
	OpenAIBaseURL      string
	CompletionsModel   string
	EmbeddingsAPIURL   string
	EmbeddingsModel    string
	EmbeddingsAPIKey   string
	DBPath             string
	OutputPath         string
	OllamaBaseURL      string
	TelegramToken      string
	TelegramChatServer string
}

func getEnv(key, defaultValue string, printEnv bool) string {
	value := os.Getenv(key)
	if printEnv {
		fmt.Println("Env", slog.Any("key", key), slog.Any("value", value))
	}
	if value == "" {
		return defaultValue
	}
	return value
}

func getEnvOrPanic(key string, printEnv bool) string { //nolint
	value := getEnv(key, "", printEnv)
	if value == "" {
		panic(fmt.Sprintf("Environment variable %s is not set", key))
	}
	return value
}

func LoadConfig(printEnv bool) (*Config, error) {
	_ = godotenv.Load()
	conf := &Config{
		OpenAIAPIKey:       getEnv("OPENAI_API_KEY", "", printEnv),
		GraphqlPort:        getEnv("GRAPHQL_PORT", "3000", printEnv),
		OpenAIBaseURL:      getEnv("OPENAI_BASE_URL", "https://api.openai.com/v1", printEnv),
		CompletionsModel:   getEnvOrPanic("COMPLETIONS_MODEL", printEnv),
		EmbeddingsAPIURL:   getEnv("EMBEDDINGS_API_URL", "https://api.openai.com/v1", printEnv),
		EmbeddingsModel:    getEnvOrPanic("EMBEDDINGS_MODEL", printEnv),
		EmbeddingsAPIKey:   getEnv("EMBEDDINGS_API_KEY", "", printEnv),
		DBPath:             getEnv("DB_PATH", "./store.db", printEnv),
		OutputPath:         getEnv("OUTPUT_PATH", "./output", printEnv),
		OllamaBaseURL:      getEnv("OLLAMA_BASE_URL", "", printEnv),
		TelegramToken:      getEnv("TELEGRAM_TOKEN", "", printEnv),
		TelegramChatServer: getEnv("TELEGRAM_CHAT_SERVER", "", printEnv),
	}
	return conf, nil
}

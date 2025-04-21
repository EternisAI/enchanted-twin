package config

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	OpenAIAPIKey     string
	GraphqlPort      string
	OpenAIBaseURL    string
	EmbeddingsAPIURL string
	CompletionsModel string
	EmbeddingsModel  string
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
		OpenAIAPIKey:     getEnv("OPENAI_API_KEY", "", printEnv),
		GraphqlPort:      getEnv("GRAPHQL_PORT", "3000", printEnv),
		OpenAIBaseURL:    getEnv("OPENAI_BASE_URL", "https://api.openai.com/v1", printEnv),
		EmbeddingsAPIURL: getEnv("EMBEDDINGS_API_URL", "https://api.openai.com/v1", printEnv),
		CompletionsModel: getEnvOrPanic("COMPLETIONS_MODEL", printEnv),
		EmbeddingsModel:  getEnvOrPanic("EMBEDDINGS_MODEL", printEnv),
	}
	return conf, nil
}

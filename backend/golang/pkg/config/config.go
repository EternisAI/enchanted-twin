package config

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	OpenAIAPIKey string
	GraphqlPort  string
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

func getEnvOrPanic(key string, printEnv bool) string {
	value := getEnv(key, "", printEnv)
	if value == "" {
		panic(fmt.Sprintf("Environment variable %s is not set", key))
	}
	return value
}

func LoadConfig(printEnv bool) (*Config, error) {
	_ = godotenv.Load()
	return &Config{
		OpenAIAPIKey: getEnvOrPanic("OPENAI_API_KEY", printEnv),
		GraphqlPort:  getEnvOrPanic("GRAPHQL_PORT", printEnv),
	}, nil
}

package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/charmbracelet/log"
	"github.com/joho/godotenv"
)

type Config struct {
	OpenAIAPIKey         string
	GraphqlPort          string
	OpenAIBaseURL        string
	CompletionsModel     string
	EmbeddingsAPIURL     string
	EmbeddingsModel      string
	EmbeddingsAPIKey     string
	DBPath               string
	AppDataPath          string
	OllamaBaseURL        string
	TelegramToken        string
	TelegramChatServer   string
	TelegramTDLibAPIID   int32
	TelegramTDLibAPIHash string
}

func getEnv(key, defaultValue string, printEnv bool) string {
	logger := log.Default()
	value := os.Getenv(key)
	if printEnv {
		logger.Info("Env", "key", key, "value", value)
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

	conf := &Config{
		OpenAIAPIKey:         getEnv("OPENAI_API_KEY", "", printEnv),
		GraphqlPort:          getEnv("GRAPHQL_PORT", "3000", printEnv),
		OpenAIBaseURL:        getEnv("OPENAI_BASE_URL", "https://api.openai.com/v1", printEnv),
		CompletionsModel:     getEnvOrPanic("COMPLETIONS_MODEL", printEnv),
		EmbeddingsAPIURL:     getEnv("EMBEDDINGS_API_URL", "https://api.openai.com/v1", printEnv),
		EmbeddingsModel:      getEnvOrPanic("EMBEDDINGS_MODEL", printEnv),
		EmbeddingsAPIKey:     getEnv("EMBEDDINGS_API_KEY", "", printEnv),
		DBPath:               getEnv("DB_PATH", "./store.db", printEnv),
		AppDataPath:          getEnv("APP_DATA_PATH", "./output", printEnv),
		OllamaBaseURL:        getEnv("OLLAMA_BASE_URL", "", printEnv),
		TelegramToken:        getEnv("TELEGRAM_TOKEN", "", printEnv),
		TelegramChatServer:   getEnvOrPanic("TELEGRAM_CHAT_SERVER", printEnv),
		TelegramTDLibAPIHash: getEnv("TELEGRAM_TDLIB_API_HASH", "", printEnv),
	}

	// Parse TDLib API ID as integer
	apiIDStr := getEnv("TELEGRAM_TDLIB_API_ID", "0", printEnv)
	if apiIDStr != "0" && apiIDStr != "" {
		apiID, err := strconv.ParseInt(apiIDStr, 10, 32)
		if err != nil {
			log.Warn("Invalid TELEGRAM_TDLIB_API_ID value", "error", err)
		} else {
			conf.TelegramTDLibAPIID = int32(apiID)
		}
	}

	return conf, nil
}

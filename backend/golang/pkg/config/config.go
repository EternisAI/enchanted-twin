// Owner: august@eternis.ai
package config

import (
	"fmt"
	"os"

	"github.com/charmbracelet/log"
	"github.com/joho/godotenv"
)

type Config struct {
	CompletionsAPIURL  string
	CompletionsAPIKey  string
	CompletionsModel   string
	ReasoningModel     string
	GraphqlPort        string
	EmbeddingsAPIURL   string
	EmbeddingsAPIKey   string
	EmbeddingsModel    string
	DBPath             string
	AppDataPath        string
	TelegramToken      string
	TelegramChatServer string
	ContainerRuntime   string
	WeaviatePort       string
	EnchantedMcpURL    string
	InviteServerURL    string
	// LiveKit configuration
	LiveKitURL       string
	LiveKitAPIKey    string
	LiveKitAPISecret string
	LiveKitPort      string
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
		CompletionsAPIURL:  getEnv("COMPLETIONS_API_URL", "https://api.openai.com/v1", printEnv),
		CompletionsAPIKey:  getEnv("COMPLETIONS_API_KEY", "", printEnv),
		CompletionsModel:   getEnv("COMPLETIONS_MODEL", "gpt-4.1-mini", printEnv),
		ReasoningModel:     getEnv("REASONING_MODEL", "gpt-4.1-mini", printEnv),
		GraphqlPort:        getEnv("GRAPHQL_PORT", "44999", printEnv),
		EmbeddingsAPIURL:   getEnv("EMBEDDINGS_API_URL", "https://api.openai.com/v1", printEnv),
		EmbeddingsModel:    getEnv("EMBEDDINGS_MODEL", "text-embedding-3-small", printEnv),
		EmbeddingsAPIKey:   getEnv("EMBEDDINGS_API_KEY", "", printEnv),
		DBPath:             getEnv("DB_PATH", "./store.db", printEnv),
		AppDataPath:        getEnv("APP_DATA_PATH", "./output", printEnv),
		TelegramToken:      getEnv("TELEGRAM_TOKEN", "", printEnv),
		TelegramChatServer: getEnvOrPanic("TELEGRAM_CHAT_SERVER", printEnv),
		ContainerRuntime:   getEnv("CONTAINER_RUNTIME", "podman", printEnv),
		WeaviatePort:       getEnv("WEAVIATE_PORT", "51414", printEnv),
		EnchantedMcpURL:    getEnv("ENCHANTED_MCP_URL", "", printEnv),
		InviteServerURL:    getEnv("INVITE_SERVER_URL", "", printEnv),
		// LiveKit configuration with defaults
		LiveKitURL:       getEnv("LIVEKIT_URL", "ws://localhost:7880", printEnv),
		LiveKitAPIKey:    getEnv("LIVEKIT_API_KEY", "devkey", printEnv),
		LiveKitAPISecret: getEnv("LIVEKIT_API_SECRET", "secret1234567890abcdef1234567890", printEnv),
		LiveKitPort:      getEnv("LIVEKIT_PORT", "7880", printEnv),
	}
	return conf, nil
}

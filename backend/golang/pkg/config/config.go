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
	WatchDirectoryPath string
	TelegramChatServer string
	ContainerRuntime   string
	WeaviatePort       string
	EnchantedMcpURL    string
	ProxyTeeURL        string
	UseLocalEmbedding  string
	AnonymizerType     string
	TelegramBotName    string
	TTSEndpoint        string
}

func getEnv(key, defaultValue string, printEnv bool, logger *log.Logger) string {
	value := os.Getenv(key)
	if printEnv && logger != nil {
		logger.Info("Env", "key", key, "value", value)
	}
	if value == "" {
		return defaultValue
	}
	return value
}

func getEnvOrPanic(key string, printEnv bool, logger *log.Logger) string {
	value := getEnv(key, "", printEnv, logger)
	if value == "" {
		panic(fmt.Sprintf("Environment variable %s is not set", key))
	}
	return value
}

func LoadConfig(printEnv bool, logger *log.Logger) (*Config, error) {
	_ = godotenv.Load()

	conf := &Config{
		CompletionsAPIURL:  getEnv("COMPLETIONS_API_URL", "https://api.openai.com/v1", printEnv, logger),
		CompletionsAPIKey:  getEnv("COMPLETIONS_API_KEY", "", printEnv, logger),
		CompletionsModel:   getEnv("COMPLETIONS_MODEL", "gpt-4.1-mini", printEnv, logger),
		ReasoningModel:     getEnv("REASONING_MODEL", "gpt-4.1-mini", printEnv, logger),
		GraphqlPort:        getEnv("GRAPHQL_PORT", "44999", printEnv, logger),
		EmbeddingsAPIURL:   getEnv("EMBEDDINGS_API_URL", "https://api.openai.com/v1", printEnv, logger),
		EmbeddingsModel:    getEnv("EMBEDDINGS_MODEL", "text-embedding-3-small", printEnv, logger),
		EmbeddingsAPIKey:   getEnv("EMBEDDINGS_API_KEY", "", printEnv, logger),
		DBPath:             getEnv("DB_PATH", "./output/sqlite/store.db", printEnv, logger),
		AppDataPath:        getEnv("APP_DATA_PATH", "./output", printEnv, logger),
		TelegramChatServer: getEnvOrPanic("TELEGRAM_CHAT_SERVER", printEnv, logger),
		ContainerRuntime:   getEnv("CONTAINER_RUNTIME", "podman", printEnv, logger),
		WeaviatePort:       getEnv("WEAVIATE_PORT", "51414", printEnv, logger),
		EnchantedMcpURL:    getEnv("ENCHANTED_MCP_URL", "", printEnv, logger),
		ProxyTeeURL:        getEnv("PROXY_TEE_URL", "", printEnv, logger),
		UseLocalEmbedding:  getEnv("USE_LOCAL_EMBEDDINGS", "", printEnv, logger),
		AnonymizerType:     getEnv("ANONYMIZER_TYPE", "llm", printEnv, logger),
		TelegramBotName:    getEnv("TELEGRAM_BOT_NAME", "TalkEnchantedBot", printEnv, logger),
		TTSEndpoint:        getEnv("TTS_ENDPOINT", "https://inference.tinfoil.sh/v1/audio/speech", printEnv, logger),
	}
	return conf, nil
}

// Owner: august@eternis.ai
package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

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
	PostgresPort       string
	PostgresDataPath   string // Deprecated: PostgreSQL data is now always stored in AppDataPath/postgres-data
	MemoryBackend      string
	EnchantedMcpURL    string
	ProxyTeeURL        string
	UseLocalEmbedding  string
	AnonymizerType     string
	TelegramBotName    string
	TTSEndpoint        string
	// Logging configuration
	LogFormat string
	LogLevel  string
	LogOutput string
	// Component-specific log levels
	ComponentLogLevels map[string]string `yaml:"component_log_levels" json:"component_log_levels"`
}

func getEnv(key, defaultValue string, printEnv bool) string {
	value := os.Getenv(key)
	if printEnv {
		if value == "" {
			log.Printf("ENV: %s = %s (default)", key, defaultValue)
		} else {
			// Mask sensitive values (API keys, tokens, passwords)
			displayValue := value
			if isSensitiveKey(key) {
				displayValue = maskSensitiveValue(value)
			}
			log.Printf("ENV: %s = %s", key, displayValue)
		}
	}
	if value == "" {
		return defaultValue
	}
	return value
}

// isSensitiveKey determines if an environment variable contains sensitive information.
func isSensitiveKey(key string) bool {
	sensitiveKeys := []string{
		"API_KEY", "TOKEN", "PASSWORD", "SECRET", "KEY", "AUTH",
		"COMPLETIONS_API_KEY", "EMBEDDINGS_API_KEY", "TELEGRAM_BOT_TOKEN",
	}
	for _, sensitive := range sensitiveKeys {
		if len(key) >= len(sensitive) && key[len(key)-len(sensitive):] == sensitive {
			return true
		}
	}
	return false
}

// maskSensitiveValue masks sensitive values for logging.
func maskSensitiveValue(value string) string {
	l := len(value)
	if l <= 8 {
		return "***masked***"
	}
	// For short tokens (9-12 chars), reveal only first and last character.
	if l <= 12 {
		return value[:1] + "***masked***" + value[l-1:]
	}
	// For longer tokens, reveal only the first 4 and last 4 characters.
	return value[:4] + "***masked***" + value[l-4:]
}

func getEnvOrPanic(key string, printEnv bool) string {
	value := getEnv(key, "", printEnv)
	if value == "" {
		panic(fmt.Sprintf("Environment variable %s is not set", key))
	}
	return value
}

// LoadConfigWithAutoDetection loads configuration with automatic printEnv detection.
// The printEnv flag is determined by the DEBUG_CONFIG_PRINT environment variable.
// Set DEBUG_CONFIG_PRINT=true to enable environment variable logging during config load.
func LoadConfigWithAutoDetection() (*Config, error) {
	// Determine printEnv flag from environment variable
	printEnv := os.Getenv("DEBUG_CONFIG_PRINT") == "true"
	return LoadConfig(printEnv)
}

func LoadConfig(printEnv bool) (*Config, error) {
	_ = godotenv.Load()

	if printEnv {
		log.Printf("Loading configuration with environment variable debugging enabled")
	}

	conf := &Config{
		CompletionsAPIURL:  getEnv("COMPLETIONS_API_URL", "https://api.openai.com/v1", printEnv),
		CompletionsAPIKey:  getEnv("COMPLETIONS_API_KEY", "", printEnv),
		CompletionsModel:   getEnv("COMPLETIONS_MODEL", "gpt-4.1-mini", printEnv),
		ReasoningModel:     getEnv("REASONING_MODEL", "gpt-4.1-mini", printEnv),
		GraphqlPort:        getEnv("GRAPHQL_PORT", "44999", printEnv),
		EmbeddingsAPIURL:   getEnv("EMBEDDINGS_API_URL", "https://api.openai.com/v1", printEnv),
		EmbeddingsModel:    getEnv("EMBEDDINGS_MODEL", "text-embedding-3-small", printEnv),
		EmbeddingsAPIKey:   getEnv("EMBEDDINGS_API_KEY", "", printEnv),
		DBPath:             getEnv("DB_PATH", "./output/sqlite/store.db", printEnv),
		AppDataPath:        getEnv("APP_DATA_PATH", "./output", printEnv),
		TelegramChatServer: getEnvOrPanic("TELEGRAM_CHAT_SERVER", printEnv),
		ContainerRuntime:   getEnv("CONTAINER_RUNTIME", "podman", printEnv),
		PostgresPort:       getEnv("POSTGRES_PORT", "5432", printEnv),
		MemoryBackend:      getEnv("MEMORY_BACKEND", "postgresql", printEnv),
		EnchantedMcpURL:    getEnv("ENCHANTED_MCP_URL", "", printEnv),
		ProxyTeeURL:        getEnv("PROXY_TEE_URL", "", printEnv),
		UseLocalEmbedding:  getEnv("USE_LOCAL_EMBEDDINGS", "", printEnv),
		AnonymizerType:     getEnv("ANONYMIZER_TYPE", "llm", printEnv),
		TelegramBotName:    getEnv("TELEGRAM_BOT_NAME", "TalkEnchantedBot", printEnv),
		TTSEndpoint:        getEnv("TTS_ENDPOINT", "https://inference.tinfoil.sh/v1/audio/speech", printEnv),
		LogFormat:          getEnv("LOG_FORMAT", "json", printEnv),
		LogLevel:           getEnv("LOG_LEVEL", "info", printEnv),
		LogOutput:          getEnv("LOG_OUTPUT", "stdout", printEnv),
		ComponentLogLevels: make(map[string]string),
	}

	conf.PostgresDataPath = filepath.Join(conf.AppDataPath, "postgres-data")

	conf.LoadComponentLogLevels()

	return conf, nil
}

func (c *Config) LoadComponentLogLevels() {
	if c.ComponentLogLevels == nil {
		c.ComponentLogLevels = make(map[string]string)
	}

	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "LOG_LEVEL_") {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				key := parts[0]
				value := parts[1]

				componentID := strings.TrimPrefix(key, "LOG_LEVEL_")
				c.ComponentLogLevels[componentID] = value
			}
		}
	}
}

func (c *Config) GetComponentLogLevel(componentID string) string {
	if level, exists := c.ComponentLogLevels[componentID]; exists {
		return level
	}
	return "info"
}

func (c *Config) SetComponentLogLevel(componentID string, level string) {
	if c.ComponentLogLevels == nil {
		c.ComponentLogLevels = make(map[string]string)
	}
	c.ComponentLogLevels[componentID] = level
}

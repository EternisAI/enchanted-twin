package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port    string
	GinMode string

	// Google OAuth
	GoogleClientID     string
	GoogleClientSecret string

	// Slack OAuth
	SlackClientID     string
	SlackClientSecret string

	// Twitter OAuth
	TwitterClientID     string
	TwitterClientSecret string

	// Composio API
	ComposioAPIKey        string
	ComposioTwitterConfig string

	// OpenAI API
	OpenAIAPIKey     string
	OpenRouterAPIKey string
	TinfoilAPIKey    string
}

var AppConfig *Config

func LoadConfig() {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	AppConfig = &Config{
		Port:    getEnvOrDefault("PORT", "8080"),
		GinMode: getEnvOrDefault("GIN_MODE", "debug"),

		// Google
		GoogleClientID:     getEnvOrDefault("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnvOrDefault("GOOGLE_CLIENT_SECRET", ""),

		// Slack
		SlackClientID:     getEnvOrDefault("SLACK_CLIENT_ID", ""),
		SlackClientSecret: getEnvOrDefault("SLACK_CLIENT_SECRET", ""),

		// Twitter
		TwitterClientID: getEnvOrDefault("TWITTER_CLIENT_ID", ""),

		// Composio
		ComposioAPIKey: getEnvOrDefault("COMPOSIO_API_KEY", ""),
		ComposioTwitterConfig: getEnvOrDefault("COMPOSIO_TWITTER_CONFIG", ""),

		// OpenAI
		OpenAIAPIKey:     getEnvOrDefault("OPENAI_API_KEY", ""),
		OpenRouterAPIKey: getEnvOrDefault("OPENROUTER_API_KEY", ""),
		TinfoilAPIKey:    getEnvOrDefault("TINFOIL_API_KEY", ""),
	}

	// Validate required configs
	if AppConfig.GoogleClientID == "" || AppConfig.SlackClientID == "" || AppConfig.TwitterClientID == "" {
		log.Println("Warning: Some OAuth client IDs are missing. Please check your environment variables.")
	}

	if AppConfig.ComposioAPIKey == "" {
		log.Println("Warning: Composio API key is missing. Please set COMPOSIO_API_KEY environment variable.")
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

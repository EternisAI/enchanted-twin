package helpers

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

func LoadEnvFile(maxDepth int) error {
	// Use default depth if invalid value provided
	if maxDepth <= 0 {
		maxDepth = 5
	}

	// Check if ENV_FILE path is explicitly specified
	if envFilePath := os.Getenv("ENV_FILE"); envFilePath != "" {
		if err := godotenv.Load(envFilePath); err == nil {
			return nil
		}
	}

	// Start with current directory
	path := ".env"
	if err := godotenv.Load(path); err == nil {
		return nil
	}

	// Check parent directories up to maxDepth levels
	prefix := ".."
	for i := 1; i <= maxDepth; i++ {
		path = prefix + "/.env"
		if err := godotenv.Load(path); err == nil {
			return nil
		}
		prefix = prefix + "/.."
	}

	return fmt.Errorf("could not find .env file after checking %d parent directories", maxDepth)
}

func GetEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	fmt.Printf("key: %s, value: %s\n", key, value)
	if value == "" {
		return defaultValue
	}
	return value
}

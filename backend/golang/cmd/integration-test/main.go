package main

import (
	"log"

	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/integration"
)

func main() {
	if err := integration.IntegrationTest(); err != nil {
		log.Fatal("Integration test failed:", err)
	}
	log.Println("Integration test completed successfully")
}

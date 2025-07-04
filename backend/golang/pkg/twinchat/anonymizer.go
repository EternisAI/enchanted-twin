package twinchat

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"
)

// MockAnonymizer returns a mock privacy dictionary JSON that accumulates entries over time
func MockAnonymizer(ctx context.Context, chatID string, message string) (*string, error) {
	// Base dictionary (starting structure)
	baseDict := map[string]interface{}{
		"arthur":        "User",
		"augusto":       "User",
		"artur":         "User",
		"san francisco": "Location",
		"new york":      "Location",
		"google":        "Company",
		"apple":         "Company",
		"microsoft":     "Company",
	}

	// Pool of additional mock entries to add randomly
	mockEntries := []map[string]interface{}{
		{"john": "User"},
		{"jane": "User"},
		{"bob": "User"},
		{"alice": "User"},
		{"london": "Location"},
		{"paris": "Location"},
		{"tokyo": "Location"},
		{"berlin": "Location"},
		{"facebook": "Company"},
		{"amazon": "Company"},
		{"netflix": "Company"},
		{"tesla": "Company"},
		{"john.doe@gmail.com": "Email"},
		{"jane.smith@yahoo.com": "Email"},
		{"user123@hotmail.com": "Email"},
		{"555-1234": "Phone"},
		{"555-5678": "Phone"},
		{"555-9999": "Phone"},
		{"project alpha": "Project"},
		{"secret meeting": "Event"},
		{"confidential document": "Document"},
		{"api key": "Credential"},
		{"password123": "Credential"},
		{"credit card": "Financial"},
		{"bank account": "Financial"},
		{"social security": "PII"},
		{"driver license": "PII"},
	}

	// Start with base dictionary
	currentDict := make(map[string]interface{})
	for k, v := range baseDict {
		currentDict[k] = v
	}

	// Add 2-4 random new entries each time
	rand.Seed(time.Now().UnixNano())
	numToAdd := rand.Intn(3) + 2 // 2-4 entries

	for i := 0; i < numToAdd && len(mockEntries) > 0; i++ {
		// Pick a random entry
		index := rand.Intn(len(mockEntries))
		entry := mockEntries[index]

		// Add the entry to current dict
		for k, v := range entry {
			currentDict[k] = v
		}

		// Remove the used entry to avoid duplicates
		mockEntries = append(mockEntries[:index], mockEntries[index+1:]...)
	}

	// Add metadata
	currentDict["_metadata"] = map[string]interface{}{
		"chat_id":      chatID,
		"last_updated": time.Now().Format(time.RFC3339),
		"total_rules":  len(currentDict) - 1, // -1 to exclude metadata itself
		"version":      fmt.Sprintf("v%d", rand.Intn(10)+1),
	}

	jsonData, err := json.Marshal(currentDict)
	if err != nil {
		return nil, err
	}

	jsonString := string(jsonData)
	return &jsonString, nil
}
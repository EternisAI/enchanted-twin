package twinchat

import (
	"context"
	"encoding/json"
)

// MockAnonymizer returns a mock privacy dictionary JSON
func MockAnonymizer(ctx context.Context, chatID string, message string) (*string, error) {
	mockDict := map[string]interface{}{
		"arthur": "User",
		"augusto": "User",
		"artur": "User",
		"san francisco": "Location",
		"new york": "Location",
		"google": "Company",
		"apple": "Company",
		"microsoft": "Company",
	}

	jsonData, err := json.Marshal(mockDict)
	if err != nil {
		return nil, err
	}

	jsonString := string(jsonData)
	return &jsonString, nil
} 
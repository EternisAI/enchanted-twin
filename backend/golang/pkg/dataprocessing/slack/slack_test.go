package slack

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	"github.com/stretchr/testify/assert"

	"github.com/EternisAI/enchanted-twin/pkg/db"
)

func TestProcessFile(t *testing.T) {
	logger := log.New(os.Stdout)
	store, err := db.NewStore(context.Background(), "test", nil)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	slack, err := NewSlackProcessor(store, logger)
	if err != nil {
		t.Fatalf("Failed to create slack processor: %v", err)
	}

	// Create a temporary directory structure
	tempDir, err := os.MkdirTemp("", "slack-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer func() {
		err = os.RemoveAll(tempDir)
		if err != nil {
			t.Fatalf("Failed to remove temp directory: %v", err)
		}
	}()

	// Create a channel directory
	channelDir := filepath.Join(tempDir, "general")
	err = os.MkdirAll(channelDir, 0o755)
	if err != nil {
		t.Fatalf("Failed to create channel directory: %v", err)
	}

	// Create test Slack messages
	testMessages := []SlackMessage{
		{
			Text: "Hello world",
			UserProfile: UserProfile{
				Name: "john_doe",
			},
			Timestamp: "1671944298.000000", // 2022-12-25T04:38:18Z
		},
		{
			Text: "How are you?",
			UserProfile: UserProfile{
				Name: "jane_doe",
			},
			Timestamp: "1671944358.000000", // 2022-12-25T04:39:18Z
		},
	}

	// Write test data to JSON file
	testFile := filepath.Join(channelDir, "2022-12-25.json")
	jsonData, err := json.Marshal(testMessages)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	err = os.WriteFile(testFile, jsonData, 0o644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Test ProcessFile
	docs, err := slack.ProcessFile(context.Background(), testFile)
	if err != nil {
		t.Fatalf("ProcessFile failed: %v", err)
	}

	assert.Equal(t, 1, len(docs), "Expected 1 conversation document")

	doc := docs[0]
	assert.Equal(t, "slack", doc.Source())
	assert.Equal(t, []string{"social", "chat", "work"}, doc.Tags())
	assert.Equal(t, 2, len(doc.People), "Expected 2 people in conversation")
	assert.Contains(t, doc.People, "john_doe")
	assert.Contains(t, doc.People, "jane_doe")
	assert.Equal(t, 2, len(doc.Conversation), "Expected 2 messages in conversation")

	// Check first message
	assert.Equal(t, "john_doe", doc.Conversation[0].Speaker)
	assert.Equal(t, "Hello world", doc.Conversation[0].Content)
	expectedTime1 := time.Unix(1671944298, 0)
	assert.True(t, doc.Conversation[0].Time.Equal(expectedTime1))

	// Check second message
	assert.Equal(t, "jane_doe", doc.Conversation[1].Speaker)
	assert.Equal(t, "How are you?", doc.Conversation[1].Content)
	expectedTime2 := time.Unix(1671944358, 0)
	assert.True(t, doc.Conversation[1].Time.Equal(expectedTime2))

	// Check metadata
	assert.Equal(t, "conversation", doc.Metadata()["type"])
	assert.Equal(t, "general", doc.Metadata()["channelName"])
	assert.Equal(t, "2", doc.Metadata()["messageCount"])
}

func TestShouldSkipFile(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		filename string
		expected bool
	}{
		{
			name:     "Valid JSON file",
			path:     "/path/to/channel/2022-12-25.json",
			filename: "2022-12-25.json",
			expected: false,
		},
		{
			name:     "macOS system file in __MACOSX",
			path:     "/path/__MACOSX/channel/._2022-12-25.json",
			filename: "._2022-12-25.json",
			expected: true,
		},
		{
			name:     "Dot-underscore file",
			path:     "/path/to/channel/._2022-12-25.json",
			filename: "._2022-12-25.json",
			expected: true,
		},
		{
			name:     "DS_Store file",
			path:     "/path/to/channel/.DS_Store",
			filename: ".DS_Store",
			expected: true,
		},
		{
			name:     "Thumbs.db file",
			path:     "/path/to/channel/Thumbs.db",
			filename: "Thumbs.db",
			expected: true,
		},
		{
			name:     "users.json metadata",
			path:     "/path/to/users.json",
			filename: "users.json",
			expected: true,
		},
		{
			name:     "channels.json metadata",
			path:     "/path/to/channels.json",
			filename: "channels.json",
			expected: true,
		},
		{
			name:     "dms.json metadata",
			path:     "/path/to/dms.json",
			filename: "dms.json",
			expected: true,
		},
		{
			name:     "groups.json metadata",
			path:     "/path/to/groups.json",
			filename: "groups.json",
			expected: true,
		},
		{
			name:     "mpims.json metadata",
			path:     "/path/to/mpims.json",
			filename: "mpims.json",
			expected: true,
		},
		{
			name:     "integration_logs.json metadata",
			path:     "/path/to/integration_logs.json",
			filename: "integration_logs.json",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldSkipFile(tt.path, tt.filename)
			assert.Equal(t, tt.expected, result, "shouldSkipFile(%s, %s) = %v, expected %v", tt.path, tt.filename, result, tt.expected)
		})
	}
}

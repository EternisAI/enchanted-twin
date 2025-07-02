package directorywatcher

import (
	"os"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/mocks"

	"github.com/EternisAI/enchanted-twin/pkg/db"
)

func TestDirectoryWatcher_isSupportedFile(t *testing.T) {
	store := &db.Store{}
	logger := log.New(os.Stdout)
	temporalClient := &mocks.Client{}

	watcher, err := NewDirectoryWatcher(store, nil, logger, temporalClient, "./test_data")
	require.NoError(t, err)

	testCases := []struct {
		filename string
		expected bool
	}{
		{"test.json", true},
		{"test.mbox", true},
		{"test.zip", true},
		{"test.sqlite", true},
		{"test.db", true},
		{"whatsapp_messages.db", true},
		{"telegram_export.json", true},
		{"gmail_archive.mbox", true},
		{"slack_export.zip", true},
		{"test.txt", false},
		{"test.pdf", false},
		{".hidden_file.json", false},
		{"temp_file.tmp", false},
	}

	for _, tc := range testCases {
		t.Run(tc.filename, func(t *testing.T) {
			result := watcher.isSupportedFile(tc.filename)
			assert.Equal(t, tc.expected, result, "File %s should be supported: %v", tc.filename, tc.expected)
		})
	}
}

func TestDirectoryWatcher_determineDataSourceType(t *testing.T) {
	store := &db.Store{}
	logger := log.New(os.Stdout)
	temporalClient := &mocks.Client{}

	watcher, err := NewDirectoryWatcher(store, nil, logger, temporalClient, "./test_data")
	require.NoError(t, err)

	testCases := []struct {
		filename string
		expected string
	}{
		{"whatsapp_messages.db", "WhatsApp"},
		{"whatsapp_backup.sqlite", "WhatsApp"},
		{"telegram_export.json", "Telegram"},
		{"gmail_archive.mbox", "Gmail"},
		{"gmail_backup.mbox", "Gmail"},
		{"slack_export.zip", "Slack"},
		{"twitter_archive.zip", "X"},
		{"x_data.zip", "X"},
		{"chatgpt_conversations.json", "ChatGPT"},
		{"chatgpt_export.zip", "ChatGPT"},
		{"random_data.json", ""},
		{"unknown_export.zip", "X"},
		{"unsupported.txt", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.filename, func(t *testing.T) {
			result := watcher.determineDataSourceType(tc.filename)
			assert.Equal(t, tc.expected, result, "File %s should be detected as: %s", tc.filename, tc.expected)
		})
	}
}

func TestDirectoryWatcher_CreateAndStart(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "watcher_test_*")
	require.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Errorf("Failed to remove temporary directory: %v", err)
		}
	}()

	store := &db.Store{}
	logger := log.New(os.Stdout)
	temporalClient := &mocks.Client{}

	watcher, err := NewDirectoryWatcher(store, nil, logger, temporalClient, tempDir)
	require.NoError(t, err)
	require.NotNil(t, watcher)

	assert.Equal(t, tempDir, watcher.watchDir)
	assert.NotNil(t, watcher.watcher)
	assert.NotNil(t, watcher.shutdownCh)
	assert.NotNil(t, watcher.fileBuffer)
}

func TestDirectoryWatcher_BufferEvents(t *testing.T) {
	store := &db.Store{}
	logger := log.New(os.Stdout)
	temporalClient := &mocks.Client{}

	watcher, err := NewDirectoryWatcher(store, nil, logger, temporalClient, "./test_data")
	require.NoError(t, err)

	event1 := &FileEvent{
		Path:      "/test/file1.json",
		Operation: "CREATE",
		Timestamp: time.Now(),
	}

	event2 := &FileEvent{
		Path:      "/test/file1.json",
		Operation: "WRITE",
		Timestamp: time.Now(),
	}

	watcher.bufferFileEvent(event1)
	time.Sleep(10 * time.Millisecond)
	watcher.bufferFileEvent(event2)

	watcher.bufferMu.Lock()
	bufferedEvent, exists := watcher.fileBuffer["/test/file1.json"]
	watcher.bufferMu.Unlock()

	assert.True(t, exists, "Event should be buffered")
	assert.Equal(t, "WRITE", bufferedEvent.Operation, "Second event should overwrite first")
}

func TestDirectoryWatcher_detectJSONContentType(t *testing.T) {
	store := &db.Store{}
	logger := log.New(os.Stdout)
	temporalClient := &mocks.Client{}

	watcher, err := NewDirectoryWatcher(store, nil, logger, temporalClient, "./test_data")
	require.NoError(t, err)

	// Create a temporary test file with X/Twitter data
	tempFile, err := os.CreateTemp("", "test_x_data_*.json")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())

	// Write X/Twitter JSON content
	xContent := `[
  {
    "like": {
      "tweetId": "1234567890",
      "fullText": "This is a sample tweet that I liked on X/Twitter",
      "expandedUrl": "https://twitter.com/user/status/1234567890"
    }
  },
  {
    "tweet": {
      "created_at": "Mon Jan 01 12:00:00 +0000 2024",
      "id_str": "9876543210",
      "full_text": "This is my own tweet on X/Twitter platform",
      "retweet_count": "5",
      "favorite_count": "10",
      "lang": "en"
    }
  }
]`

	_, err = tempFile.WriteString(xContent)
	require.NoError(t, err)
	tempFile.Close()

	// Test content detection
	result := watcher.detectJSONContentType(tempFile.Name())
	assert.Equal(t, "X", result, "X/Twitter data should be detected correctly")

	// Test with Telegram-like data
	telegramFile, err := os.CreateTemp("", "test_telegram_*.json")
	require.NoError(t, err)
	defer os.Remove(telegramFile.Name())

	telegramContent := `{
  "personal_information": {
    "user_id": 123456789,
    "first_name": "Test",
    "username": "testuser"
  },
  "chats": {
    "list": []
  }
}`

	_, err = telegramFile.WriteString(telegramContent)
	require.NoError(t, err)
	telegramFile.Close()

	result = watcher.detectJSONContentType(telegramFile.Name())
	assert.Equal(t, "Telegram", result, "Telegram data should be detected correctly")
}

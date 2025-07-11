package x

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/stretchr/testify/assert"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/db"
)

func TestProcessDirectory(t *testing.T) {
	logger := log.New(os.Stdout)
	store, err := db.NewStore(context.Background(), "test")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	processor, err := NewXProcessor(store, logger)
	if err != nil {
		t.Fatalf("Failed to create x processor: %v", err)
	}

	// Create a temporary directory structure
	tempDir, err := os.MkdirTemp("", "x-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer func() {
		err = os.RemoveAll(tempDir)
		if err != nil {
			t.Fatalf("Failed to remove temp directory: %v", err)
		}
	}()

	// Create account.js file
	accountContent := `window.YTD.account.part0 = [
  {
    "account" : {
      "email" : "test@example.com",
      "createdVia" : "web",
      "username" : "testuser",
      "accountId" : "123456789",
      "createdAt" : "2020-01-01T00:00:00.000Z",
      "accountDisplayName" : "Test User"
    }
  }
]`

	accountFile := filepath.Join(tempDir, "account.js")
	err = os.WriteFile(accountFile, []byte(accountContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write account file: %v", err)
	}

	// Create tweets.js file
	tweetsContent := `window.YTD.tweets.part0 = [
  {
    "tweet" : {
      "retweetCount" : "0",
      "entities" : {
        "hashtags" : [ ],
        "symbols" : [ ],
        "user_mentions" : [ ],
        "urls" : [ ]
      },
      "display_text_range" : [ "0", "33" ],
      "favoriteCount" : "0",
      "id_str" : "1234567890",
      "truncated" : false,
      "lang" : "en",
      "possibly_sensitive" : false,
      "created_at" : "Mon Jan 01 00:00:00 +0000 2024",
      "source" : "<a href=\"https://mobile.twitter.com\" rel=\"nofollow\">Twitter Web App</a>",
      "full_text" : "Hello world! This is my first tweet."
    }
  }
]`

	tweetsFile := filepath.Join(tempDir, "tweets.js")
	err = os.WriteFile(tweetsFile, []byte(tweetsContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write tweets file: %v", err)
	}

	// Create like.js file
	likeContent := `window.YTD.like.part0 = [
  {
    "like" : {
      "tweetId" : "987654321",
      "fullText" : "This is a tweet I liked",
      "expandedUrl" : "https://twitter.com/someone/status/987654321"
    }
  }
]`

	likeFile := filepath.Join(tempDir, "like.js")
	err = os.WriteFile(likeFile, []byte(likeContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write like file: %v", err)
	}

	// Test ProcessDirectory
	docs, err := processor.ProcessDirectory(context.Background(), tempDir)
	if err != nil {
		t.Fatalf("ProcessDirectory failed: %v", err)
	}

	// Should have 2 documents: tweets and likes (account.js doesn't create documents)
	assert.Equal(t, 2, len(docs), "Expected 2 conversation documents")

	// Check that we have both tweets and likes documents
	var tweetsDoc, likesDoc *memory.ConversationDocument
	for i := range docs {
		if docs[i].FieldMetadata["type"] == "conversation" {
			if len(docs[i].FieldTags) > 0 {
				switch docs[i].FieldTags[1] {
				case "tweet":
					tweetsDoc = &docs[i]
				case "like":
					likesDoc = &docs[i]
				}
			}
		}
	}

	// Test tweets document
	assert.NotNil(t, tweetsDoc, "Expected tweets document")
	if tweetsDoc != nil {
		assert.Equal(t, "x", tweetsDoc.Source())
		assert.Contains(t, tweetsDoc.Tags(), "social")
		assert.Contains(t, tweetsDoc.Tags(), "tweet")
		assert.Equal(t, 1, len(tweetsDoc.Conversation), "Expected 1 tweet message")
		assert.Equal(t, "testuser", tweetsDoc.Conversation[0].Speaker)
		assert.Equal(t, "Hello world! This is my first tweet.", tweetsDoc.Conversation[0].Content)
		assert.Equal(t, "testuser", tweetsDoc.User)
		assert.Contains(t, tweetsDoc.People, "testuser")

		metadata := tweetsDoc.Metadata()
		assert.Equal(t, "conversation", metadata["type"])
		assert.Equal(t, "1", metadata["tweetCount"])
	}

	// Test likes document
	assert.NotNil(t, likesDoc, "Expected likes document")
	if likesDoc != nil {
		assert.Equal(t, "x", likesDoc.Source())
		assert.Contains(t, likesDoc.Tags(), "social")
		assert.Contains(t, likesDoc.Tags(), "like")
		assert.Equal(t, 1, len(likesDoc.Conversation), "Expected 1 like message")
		assert.Equal(t, "testuser", likesDoc.Conversation[0].Speaker)
		assert.Equal(t, "Liked: This is a tweet I liked", likesDoc.Conversation[0].Content)
		assert.Equal(t, "testuser", likesDoc.User)
		assert.Contains(t, likesDoc.People, "testuser")

		metadata := likesDoc.Metadata()
		assert.Equal(t, "conversation", metadata["type"])
		assert.Equal(t, "1", metadata["likeCount"])
	}
}

package gmail

import (
	"context"
	"os"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/stretchr/testify/assert"

	"github.com/EternisAI/enchanted-twin/pkg/db"
)

func TestGmailProcessor_ProcessFile(t *testing.T) {
	tmpFile := createTestMboxFile(t)
	defer func() { _ = os.Remove(tmpFile) }()

	logger := log.New(os.Stdout)
	store, err := db.NewStore(context.Background(), ":memory:")
	assert.NoError(t, err)

	processor, err := NewGmailProcessor(store, logger)
	assert.NoError(t, err)

	docs, err := processor.ProcessFile(context.Background(), tmpFile)
	assert.NoError(t, err)
	assert.NotEmpty(t, docs)

	doc := docs[0]
	assert.Equal(t, "gmail", doc.FieldSource)
	assert.Contains(t, doc.FieldTags, "email")
	assert.NotEmpty(t, doc.Conversation)
	assert.NotEmpty(t, doc.People)
}

func createTestMboxFile(t *testing.T) string {
	// Create a conversation where user@example.com sends TO test@example.com
	// This will pass the interaction-based filtering
	content := `From user@example.com Mon Apr 07 14:31:02 +0000 2025
From: user@example.com
To: test@example.com
Subject: Test Email
Date: Mon, 07 Apr 2025 14:31:02 +0000

Hello, this is a test email from the user.

From test@example.com Mon Apr 07 14:35:00 +0000 2025
From: test@example.com
To: user@example.com
Subject: Re: Test Email
Date: Mon, 07 Apr 2025 14:35:00 +0000

Thanks for your email! This is a reply.
`

	tmpFile, err := os.CreateTemp("", "test-*.mbox")
	assert.NoError(t, err)

	_, err = tmpFile.WriteString(content)
	assert.NoError(t, err)

	err = tmpFile.Close()
	assert.NoError(t, err)

	return tmpFile.Name()
}

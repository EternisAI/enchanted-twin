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
	store, err := db.NewStore(context.Background(), "test")
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
	content := `From test@example.com Mon Apr 07 14:31:02 +0000 2025
From: test@example.com
To: user@example.com
Subject: Test Email
Date: Mon, 07 Apr 2025 14:31:02 +0000

This is a test email content.
`

	tmpFile, err := os.CreateTemp("", "test-*.mbox")
	assert.NoError(t, err)

	_, err = tmpFile.WriteString(content)
	assert.NoError(t, err)

	err = tmpFile.Close()
	assert.NoError(t, err)

	return tmpFile.Name()
}

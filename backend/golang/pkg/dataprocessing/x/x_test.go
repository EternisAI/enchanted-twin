package x

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	"github.com/stretchr/testify/assert"

	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/helpers"
)

func TestToDocuments(t *testing.T) {
	tempFile, err := os.CreateTemp("", "test-x-*.jsonl")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() {
		err = os.Remove(tempFile.Name())
		if err != nil {
			t.Fatalf("Failed to remove temp file: %v", err)
		}
	}()

	testData := `{"data":{"conversationId":"1638683789647032320-1676928456225898496","myMessage":false,"recipientId":"1638683789647032320","senderId":"1676928456225898496","text":"Hello\nican't login in discord\nloading indefinitely\nworks on phone though\nregion: Mexico\nthanks","type":"directMessage"},"timestamp":"2024-09-11T21:05:12Z","source":"x"}
{"data":{"expandedUrl":"","fullText":"A verified internet scales humanity","tweetId":"12345","type":"like"},"timestamp":"2025-04-18T17:21:50-06:00","source":"x"}
{"data":{"favoriteCount":"0","fullText":"@ChopJurassic @ReallyAmerican1 yes you do","id":"1904572225459806695","lang":"en","retweetCount":"0","type":"tweet","userId":"0"},"timestamp":"2025-03-25T16:32:58Z","source":"x"}`

	if _, err := tempFile.WriteString(testData); err != nil {
		t.Fatalf("Failed to write test data: %v", err)
	}
	err = tempFile.Close()
	if err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	count, err := helpers.CountJSONLLines(tempFile.Name())
	records, err := helpers.ReadJSONLBatch(tempFile.Name(), 0, count)
	if err != nil {
		t.Fatalf("ReadJSONL failed: %v", err)
	}
	logger := log.New(os.Stdout)
	source := NewXProcessor(nil, logger)
	documents, err := source.ToDocuments(context.Background(), records)
	if err != nil {
		t.Fatalf("ToDocuments failed: %v", err)
	}

	assert.Equal(t, 3, len(documents), "Expected 3 documents")

	expectedTimestamp1, _ := time.Parse(time.RFC3339, "2024-09-11T21:05:12Z")
	assert.Equal(t, "Hello\nican't login in discord\nloading indefinitely\nworks on phone though\nregion: Mexico\nthanks", documents[0].Content())
	assert.Equal(t, &expectedTimestamp1, documents[0].Timestamp())
	assert.Contains(t, documents[0].Tags(), "social")
	assert.Contains(t, documents[0].Tags(), "x")
	assert.Contains(t, documents[0].Tags(), "direct_message")

	metadata := documents[0].Metadata()
	assert.Equal(t, "direct_message", metadata["type"])

	expectedTimestamp3, _ := time.Parse(time.RFC3339, "2025-03-25T16:32:58Z")
	assert.Equal(t, "@ChopJurassic @ReallyAmerican1 yes you do", documents[1].Content())
	assert.Equal(t, &expectedTimestamp3, documents[1].Timestamp())
	assert.Contains(t, documents[1].Tags(), "social")
	assert.Contains(t, documents[1].Tags(), "x")
	assert.Contains(t, documents[1].Tags(), "tweet")
	assert.Equal(t, map[string]string{
		"type":          "tweet",
		"id":            "1904572225459806695",
		"favoriteCount": "0",
		"retweetCount":  "0",
	}, documents[1].Metadata())

	expectedTimestamp2, _ := time.Parse(time.RFC3339, "2025-04-18T17:21:50-06:00")
	assert.Equal(t, "A verified internet scales humanity", documents[2].Content())
	assert.Equal(t, &expectedTimestamp2, documents[2].Timestamp())
	assert.Contains(t, documents[2].Tags(), "social")
	assert.Contains(t, documents[2].Tags(), "x")
	assert.Contains(t, documents[2].Tags(), "like")
	assert.Equal(t, map[string]string{
		"type": "like",
		"id":   "12345",
		"url":  "",
	}, documents[2].Metadata())
}

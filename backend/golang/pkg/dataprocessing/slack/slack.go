package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/processor"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
	"github.com/EternisAI/enchanted-twin/pkg/db"
)

type UserProfile struct {
	Name string `json:"name"`
}

type SlackMessage struct {
	Text        string      `json:"text"`
	UserProfile UserProfile `json:"user_profile"`
	Timestamp   string      `json:"ts"`
}

type SlackProcessor struct{}

func NewSlackProcessor() processor.Processor {
	return &SlackProcessor{}
}

func (s *SlackProcessor) Name() string {
	return "slack"
}

func (s *SlackProcessor) ProcessFile(ctx context.Context, filePath string, store *db.Store) ([]types.Record, error) {
	jsonData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var messages []SlackMessage
	if err := json.Unmarshal(jsonData, &messages); err != nil {
		return nil, err
	}

	// Extract channel name from the file path
	// Expected format: .../channel-name/YYYY-MM-DD.json
	channelName := filepath.Base(filepath.Dir(filePath))

	var records []types.Record
	for _, message := range messages {
		timestamp, err := parseTimestamp(message.Timestamp)
		if err != nil {
			// fmt.Printf("Warning: Failed to parse message timestamp in file %s: %v\n", filePath, err)
			continue
		}

		messageData := map[string]any{
			"text":        message.Text,
			"username":    message.UserProfile.Name,
			"channelName": channelName,
			"myMessage":   strings.EqualFold(message.UserProfile.Name, ""),
		}

		record := types.Record{
			Data:      messageData,
			Timestamp: timestamp,
			Source:    s.Name(),
		}

		if message.Text != "" {
			records = append(records, record)
		}
	}

	return records, nil
}

func (s *SlackProcessor) ProcessDirectory(ctx context.Context, inputPath string, store *db.Store) ([]types.Record, error) {
	var allRecords []types.Record

	err := filepath.Walk(inputPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if filepath.Ext(path) != ".json" {
			return nil
		}

		records, err := s.ProcessFile(ctx, path, store)
		if err != nil {
			fmt.Printf("Warning: Failed to process file %s: %v\n", path, err)
			return nil
		}

		allRecords = append(allRecords, records...)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return allRecords, nil
}

func (s *SlackProcessor) ToDocuments(records []types.Record) ([]memory.Document, error) {
	textDocuments := make([]memory.TextDocument, 0, len(records))

	for _, record := range records {
		getString := func(key string) string {
			if val, ok := record.Data[key]; ok {
				if strVal, ok := val.(string); ok {
					return strVal
				}
			}
			return ""
		}

		message := getString("text")
		authorUsername := getString("username")
		channelName := getString("channelName")

		// Skip records with missing required fields
		if message == "" || authorUsername == "" || channelName == "" {
			continue
		}

		message = fmt.Sprintf("From %s in channel %s: %s", authorUsername, channelName, message)

		textDocuments = append(textDocuments, memory.TextDocument{
			FieldSource:    "slack",
			FieldContent:   message,
			FieldTimestamp: &record.Timestamp,
			FieldTags:      []string{"social", "slack", "chat"},
			FieldMetadata: map[string]string{
				"type":           "message",
				"channelName":    channelName,
				"authorUsername": authorUsername,
			},
		})
	}

	var documents []memory.Document
	for _, document := range textDocuments {
		documents = append(documents, &document)
	}

	return documents, nil
}

func (s *SlackProcessor) Sync(ctx context.Context, accessToken string) ([]types.Record, bool, error) {
	return nil, false, fmt.Errorf("sync operation not supported for Slack")
}

func parseTimestamp(ts string) (time.Time, error) {
	// Slack timestamps are in the format "1735051993.888329"
	parts := strings.Split(ts, ".")
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("invalid timestamp format: %s", ts)
	}

	seconds, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, err
	}

	return time.Unix(seconds, 0), nil
}

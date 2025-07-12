// owner: slimane@eternis.ai

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

	"github.com/charmbracelet/log"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
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

type SlackProcessor struct {
	store  *db.Store
	logger *log.Logger
}

func NewSlackProcessor(store *db.Store, logger *log.Logger) (*SlackProcessor, error) {
	if store == nil {
		return nil, fmt.Errorf("store is nil")
	}

	if logger == nil {
		return nil, fmt.Errorf("logger is nil")
	}
	return &SlackProcessor{store: store, logger: logger}, nil
}

// ProcessFile processes a single Slack channel file and returns ConversationDocument.
func (s *SlackProcessor) ProcessFile(ctx context.Context, filePath string) ([]memory.ConversationDocument, error) {
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

	// Group messages by channel into a single conversation
	var conversationMessages []memory.ConversationMessage
	var people []string
	peopleMap := make(map[string]bool)

	var firstMessage, lastMessage time.Time
	messageCount := 0

	for _, message := range messages {
		if message.Text == "" {
			continue
		}

		timestamp, err := parseTimestamp(message.Timestamp)
		if err != nil {
			s.logger.Warn("Failed to parse message timestamp in file", "filePath", filePath, "error", err)
			continue
		}

		username := message.UserProfile.Name
		if username == "" {
			username = "unknown"
		}

		// Track people in the conversation
		if !peopleMap[username] {
			peopleMap[username] = true
			people = append(people, username)
		}

		// Track time range
		if messageCount == 0 {
			firstMessage = timestamp
			lastMessage = timestamp
		} else {
			if timestamp.Before(firstMessage) {
				firstMessage = timestamp
			}
			if timestamp.After(lastMessage) {
				lastMessage = timestamp
			}
		}

		conversationMessages = append(conversationMessages, memory.ConversationMessage{
			Speaker: username,
			Content: message.Text,
			Time:    timestamp,
		})
		messageCount++
	}

	if len(conversationMessages) == 0 {
		return []memory.ConversationDocument{}, nil
	}

	// Create document ID using channel name and date range
	docID := fmt.Sprintf("slack-channel-%s-%d-%d", channelName, firstMessage.Unix(), lastMessage.Unix())

	conversationDoc := memory.ConversationDocument{
		FieldID:      docID,
		FieldSource:  "slack",
		FieldTags:    []string{"social", "chat", "work"},
		People:       people,
		User:         "", // We don't have user identification in Slack exports
		Conversation: conversationMessages,
		FieldMetadata: map[string]string{
			"type":         "conversation",
			"channelName":  channelName,
			"messageCount": fmt.Sprintf("%d", messageCount),
		},
	}

	return []memory.ConversationDocument{conversationDoc}, nil
}

// ProcessDirectory processes all Slack channel files in a directory.
func (s *SlackProcessor) ProcessDirectory(ctx context.Context, inputPath string) ([]memory.ConversationDocument, error) {
	var allDocuments []memory.ConversationDocument
	channelMap := make(map[string][]memory.ConversationMessage)
	channelPeople := make(map[string]map[string]bool)
	channelMetadata := make(map[string]map[string]string)

	err := filepath.Walk(inputPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Skip non-JSON files
		if filepath.Ext(path) != ".json" {
			return nil
		}

		// Skip macOS system files and artifacts
		filename := filepath.Base(path)
		if shouldSkipFile(path, filename) {
			s.logger.Debug("Skipping system file", "path", path)
			return nil
		}

		documents, err := s.ProcessFile(ctx, path)
		if err != nil {
			s.logger.Warn("Failed to process file", "path", path, "error", err)
			return nil
		}

		// Group messages by channel across all files
		for _, doc := range documents {
			channelName := doc.FieldMetadata["channelName"]
			if channelMap[channelName] == nil {
				channelMap[channelName] = []memory.ConversationMessage{}
				channelPeople[channelName] = make(map[string]bool)
				channelMetadata[channelName] = make(map[string]string)
			}

			// Merge conversations from the same channel
			channelMap[channelName] = append(channelMap[channelName], doc.Conversation...)

			// Track all people in the channel
			for _, person := range doc.People {
				channelPeople[channelName][person] = true
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Create final conversation documents per channel
	for channelName, messages := range channelMap {
		if len(messages) == 0 {
			continue
		}

		// Convert people map to slice
		var people []string
		for person := range channelPeople[channelName] {
			people = append(people, person)
		}

		// Sort messages by timestamp
		// Simple bubble sort for timestamp ordering
		for i := 0; i < len(messages); i++ {
			for j := i + 1; j < len(messages); j++ {
				if messages[i].Time.After(messages[j].Time) {
					messages[i], messages[j] = messages[j], messages[i]
				}
			}
		}

		// Create final document ID
		firstTime := messages[0].Time
		lastTime := messages[len(messages)-1].Time
		docID := fmt.Sprintf("slack-channel-%s-%d-%d", channelName, firstTime.Unix(), lastTime.Unix())

		conversationDoc := memory.ConversationDocument{
			FieldID:      docID,
			FieldSource:  "slack",
			FieldTags:    []string{"social", "chat", "work"},
			People:       people,
			User:         "", // We don't have user identification in Slack exports
			Conversation: messages,
			FieldMetadata: map[string]string{
				"type":         "conversation",
				"channelName":  channelName,
				"messageCount": fmt.Sprintf("%d", len(messages)),
			},
		}

		allDocuments = append(allDocuments, conversationDoc)
	}

	return allDocuments, nil
}

// shouldSkipFile determines if a file should be skipped during processing.
func shouldSkipFile(path, filename string) bool {
	// Skip macOS system files and artifacts
	if strings.Contains(path, "__MACOSX") {
		return true
	}

	// Skip dot-underscore files (macOS resource forks)
	if strings.HasPrefix(filename, "._") {
		return true
	}

	// Skip other common system files
	if strings.HasPrefix(filename, ".DS_Store") {
		return true
	}

	// Skip Thumbs.db (Windows)
	if strings.HasPrefix(filename, "Thumbs.db") {
		return true
	}

	// Skip common Slack metadata files that aren't message data
	switch filename {
	case "users.json", "channels.json", "dms.json", "groups.json", "mpims.json", "integration_logs.json":
		return true
	}

	return false
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

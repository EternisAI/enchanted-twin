package slack

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/dataimport"
)

type UserProfile struct {
	Name string `json:"name"`
}

type SlackMessage struct {
	Text        string      `json:"text"`
	UserProfile UserProfile `json:"user_profile"`
	Timestamp   string      `json:"ts"`
}

type Source struct {
	inputPath string
}

func New(inputPath string) *Source {
	return &Source{
		inputPath: inputPath,
	}
}

func (s *Source) Name() string {
	return "slack"
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

func (s *Source) ProcessFile(filePath string, username string) ([]dataimport.Record, error) {
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

	var records []dataimport.Record
	for _, message := range messages {
		timestamp, err := parseTimestamp(message.Timestamp)
		if err != nil {
			// fmt.Printf("Warning: Failed to parse message timestamp in file %s: %v\n", filePath, err)
			continue
		}

		messageData := map[string]interface{}{
			"text":        message.Text,
			"username":    message.UserProfile.Name,
			"channelName": channelName,
			"myMessage":   strings.EqualFold(message.UserProfile.Name, username),
		}

		record := dataimport.Record{
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

func (s *Source) ProcessDirectory(userName string) ([]dataimport.Record, error) {
	var allRecords []dataimport.Record

	err := filepath.Walk(s.inputPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Only process .json files
		if filepath.Ext(path) != ".json" {
			return nil
		}

		records, err := s.ProcessFile(path, userName)
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

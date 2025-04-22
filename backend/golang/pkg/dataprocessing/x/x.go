package x

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/helpers"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

const (
	TypeLike          = "like"
	TypeTweet         = "tweets"
	TypeDirectMessage = "direct_messages"
)

type Source struct {
	inputPath string
}

func New(inputPath string) *Source {
	return &Source{
		inputPath: inputPath,
	}
}

func (s *Source) Name() string {
	return "x"
}

func (s *Source) ProcessFile(filePath string, userId string) ([]types.Record, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	fmt.Printf("Processing file: %s\n", filePath)
	fmt.Printf("Content length: %d bytes\n", len(content))

	fileType := ""
	fileName := filepath.Base(filePath)
	switch {
	case strings.Contains(fileName, "like"):
		fileType = TypeLike
	case strings.Contains(fileName, "tweets"):
		fileType = TypeTweet
	case strings.Contains(fileName, "direct-messages"):
		fileType = TypeDirectMessage
	default:
		return nil, fmt.Errorf("unsupported X/Twitter file type: %s", fileName)
	}

	fmt.Printf("Detected file type: %s\n", fileType)

	records, err := parseTwitterFileSimple(content, fileType, userId)
	if err != nil {
		fmt.Printf("Simple parser failed, trying regex parser: %v\n", err)
		records, err = parseTwitterFile(content, fileType, userId)
		if err != nil {
			fmt.Printf("Error processing %s: %v\n", fileType, err)
			return nil, err
		}
	}

	fmt.Printf("Successfully processed %s: found %d records\n", fileType, len(records))
	return records, nil
}

func parseTwitterTimestamp(timestampStr string) (time.Time, error) {
	formats := []string{
		"Mon Jan 02 15:04:05 -0700 2006",
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.000-07:00",
	}

	var err error
	var timestamp time.Time

	for _, format := range formats {
		timestamp, err = time.Parse(format, timestampStr)
		if err == nil {
			return timestamp, nil
		}
	}

	return time.Time{}, fmt.Errorf("failed to parse timestamp: %s", timestampStr)
}

func (s *Source) ProcessDirectory(userName string, xApiKey string) ([]types.Record, error) {
	var allRecords []types.Record

	userId, err := GetUserIDByUsername(userName, xApiKey)
	if err != nil {
		userId = "0"
	}
	fmt.Printf("User ID: %s\n", userId)

	err = filepath.Walk(s.inputPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if filepath.Ext(path) != ".js" {
			return nil
		}

		fileName := filepath.Base(path)
		if !isXDataFile(fileName) {
			return nil
		}

		records, err := s.ProcessFile(path, userId)
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

func isXDataFile(fileName string) bool {
	supportedFiles := []string{"like.js", "tweets.js", "direct-messages.js"}
	for _, supported := range supportedFiles {
		if strings.Contains(fileName, supported) {
			return true
		}
	}
	return false
}

type TwitterUserResponse struct {
	Data []struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Username string `json:"username"`
	} `json:"data"`
}

func GetUserIDByUsername(username string, bearerToken string) (string, error) {
	url := fmt.Sprintf("https://api.twitter.com/2/users/by?usernames=%s", username)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", bearerToken))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making request: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Error closing response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var userResponse TwitterUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&userResponse); err != nil {
		return "", fmt.Errorf("error decoding response: %v", err)
	}

	if len(userResponse.Data) == 0 {
		return "", fmt.Errorf("no user found with username: %s", username)
	}

	return userResponse.Data[0].ID, nil
}

type LikeData struct {
	ExpandedUrl string `json:"expandedUrl"`
	FullText    string `json:"fullText"`
	TweetId     string `json:"tweetId"`
	Type        string `json:"type"`
}
type TweetData struct {
	FavoriteCount string `json:"favoriteCount"`
	FullText      string `json:"fullText"`
	Id            string `json:"id"`
	Lang          string `json:"lang"`
	RetweetCount  string `json:"retweetCount"`
	Type          string `json:"type"`
	UserId        string `json:"userId"`
}

type DirectMessageData struct {
	ConversationId string `json:"conversationId"`
	MyMessage      bool   `json:"myMessage"`
	RecipientId    string `json:"recipientId"`
	SenderId       string `json:"senderId"`
	Text           string `json:"text"`
	Type           string `json:"type"`
}

func ToDocuments(path string) ([]memory.TextDocument, error) {
	records, err := helpers.ReadJSONL[types.Record](path)
	if err != nil {
		return nil, err
	}

	documents := make([]memory.TextDocument, 0, len(records))
	for _, record := range records {

		content := ""
		metadata := map[string]string{}
		tags := []string{"x"}

		getString := func(key string) string {
			if val, ok := record.Data[key]; ok {
				if strVal, ok := val.(string); ok {
					return strVal
				}
			}
			return ""
		}

		recordType := getString("type")
		switch recordType {
		case "like":
			content = getString("fullText")
			tweetId := getString("tweetId")
			metadata = map[string]string{
				"type": "like",
				"id":   tweetId,
				"url":  getString("expandedUrl"),
			}
			tags = append(tags, "like", tweetId)

		case "tweet":
			content = getString("fullText")
			id := getString("id")
			favoriteCount := getString("favoriteCount")
			retweetCount := getString("retweetCount")
			metadata = map[string]string{
				"type":          "tweet",
				"id":            id,
				"favoriteCount": favoriteCount,
				"retweetCount":  retweetCount,
			}
			tags = append(tags, "tweet", id, retweetCount, favoriteCount)

		case "direct_message":
			content = getString("text")
			metadata = map[string]string{
				"type": "direct_message",
			}
			tags = append(tags, "direct_message",
				getString("conversationId"),
				getString("recipientId"),
				getString("senderId"))
		}

		documents = append(documents, memory.TextDocument{
			Content:   content,
			Timestamp: &record.Timestamp,
			Tags:      tags,
			Metadata:  metadata,
		})
	}
	return documents, nil
}

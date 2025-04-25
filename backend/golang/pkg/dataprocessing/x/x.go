package x

import (
	"bytes"
	"context"
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

func ParseTwitterTimestamp(timestampStr string) (time.Time, error) {
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
	defer func() { _ = resp.Body.Close() }()

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

func ToDocuments(records []types.Record) ([]memory.TextDocument, error) {
	documents := make([]memory.TextDocument, 0, len(records))
	for _, record := range records {

		content := ""
		metadata := map[string]string{}
		tags := []string{"social", "x"}

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
				"type":   "like",
				"id":     tweetId,
				"url":    getString("expandedUrl"),
				"source": "x",
			}
			tags = append(tags, "like")

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
				"source":        "x",
			}
			tags = append(tags, "tweet")

		case "direct_message":
			content = getString("text")
			metadata = map[string]string{
				"type":   "direct_message",
				"source": "x",
			}
			tags = append(tags, "direct_message")

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

func (s *Source) Sync(ctx context.Context, accessToken string) ([]types.Record, error) {
	// Create HTTP client with OAuth token
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Get user ID first
	userReq, err := http.NewRequestWithContext(
		ctx,
		"GET",
		"https://api.twitter.com/2/users/me",
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create user request: %w", err)
	}

	fmt.Println("Making request with accessToken:", accessToken)
	userReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	userResp, err := client.Do(userReq)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user data: %w", err)
	}
	defer func() { _ = userResp.Body.Close() }()

	// Read the body
	bodyBytes, err := io.ReadAll(userResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	fmt.Printf("Response Status: %d\n", userResp.StatusCode)
	fmt.Printf("Response Body: %s\n", string(bodyBytes))

	// Create a new reader with the body bytes for json.Decoder
	userResp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	if userResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch user data. Status: %d, Response: %s", userResp.StatusCode, string(bodyBytes))
	}
	fmt.Println("userResp", userResp.Body)

	var userResponse struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(userResp.Body).Decode(&userResponse); err != nil {
		return nil, fmt.Errorf("failed to decode user response: %w", err)
	}

	fmt.Printf("Retrieved user ID: %s\n", userResponse.Data.ID)

	var records []types.Record

	// Get the latest tweets
	tweetsReq, err := http.NewRequestWithContext(
		ctx,
		"GET",
		fmt.Sprintf("https://api.twitter.com/2/users/%s/tweets", userResponse.Data.ID),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create tweets request: %w", err)
	}

	tweetsReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	q := tweetsReq.URL.Query()
	q.Set("max_results", "50") // Get last 50 tweets
	q.Set("tweet.fields", "created_at,public_metrics,lang")
	tweetsReq.URL.RawQuery = q.Encode()

	tweetsResp, err := client.Do(tweetsReq)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tweets: %w", err)
	}
	defer func() { _ = tweetsResp.Body.Close() }()

	if tweetsResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(tweetsResp.Body)
		return nil, fmt.Errorf("failed to fetch tweets. Status: %d, Response: %s", tweetsResp.StatusCode, string(body))
	}

	var tweetsResponse struct {
		Data []struct {
			ID            string `json:"id"`
			Text          string `json:"text"`
			CreatedAt     string `json:"created_at"`
			Lang          string `json:"lang"`
			PublicMetrics struct {
				RetweetCount int `json:"retweet_count"`
				LikeCount    int `json:"like_count"`
			} `json:"public_metrics"`
		} `json:"data"`
	}

	if err := json.NewDecoder(tweetsResp.Body).Decode(&tweetsResponse); err != nil {
		return nil, fmt.Errorf("failed to decode tweets response: %w", err)
	}

	for _, tweet := range tweetsResponse.Data {
		timestamp, err := time.Parse(time.RFC3339, tweet.CreatedAt)
		if err != nil {
			log.Printf("Warning: Failed to parse tweet timestamp: %v", err)
			continue
		}

		data := map[string]interface{}{
			"type":          "tweet",
			"id":            tweet.ID,
			"fullText":      tweet.Text,
			"retweetCount":  fmt.Sprintf("%d", tweet.PublicMetrics.RetweetCount),
			"favoriteCount": fmt.Sprintf("%d", tweet.PublicMetrics.LikeCount),
			"lang":          tweet.Lang,
			"userId":        userResponse.Data.ID,
		}

		records = append(records, types.Record{
			Data:      data,
			Timestamp: timestamp,
			Source:    s.Name(),
		})
	}

	// Get the latest likes
	likesReq, err := http.NewRequestWithContext(
		ctx,
		"GET",
		fmt.Sprintf("https://api.twitter.com/2/users/%s/liked_tweets?tweet.fields=created_at,public_metrics&max_results=10", userResponse.Data.ID),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create likes request: %w", err)
	}

	likesReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	q = likesReq.URL.Query()
	q.Set("max_results", "50") // Get last 50 likes
	q.Set("tweet.fields", "created_at,public_metrics,lang,entities")
	likesReq.URL.RawQuery = q.Encode()

	likesResp, err := client.Do(likesReq)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch likes: %w", err)
	}
	defer func() { _ = likesResp.Body.Close() }()

	if likesResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(likesResp.Body)
		return nil, fmt.Errorf("failed to fetch likes. Status: %d, Response: %s", likesResp.StatusCode, string(body))
	}

	var likesResponse struct {
		Data []struct {
			ID            string `json:"id"`
			Text          string `json:"text"`
			CreatedAt     string `json:"created_at"`
			Lang          string `json:"lang"`
			PublicMetrics struct {
				RetweetCount int `json:"retweet_count"`
				LikeCount    int `json:"like_count"`
			} `json:"public_metrics"`
			Entities struct {
				Urls []struct {
					ExpandedURL string `json:"expanded_url"`
				} `json:"urls"`
			} `json:"entities"`
		} `json:"data"`
	}

	if err := json.NewDecoder(likesResp.Body).Decode(&likesResponse); err != nil {
		return nil, fmt.Errorf("failed to decode likes response: %w", err)
	}

	for _, like := range likesResponse.Data {
		timestamp, err := time.Parse(time.RFC3339, like.CreatedAt)
		if err != nil {
			log.Printf("Warning: Failed to parse like timestamp: %v", err)
			continue
		}

		expandedUrl := ""
		if len(like.Entities.Urls) > 0 {
			expandedUrl = like.Entities.Urls[0].ExpandedURL
		}

		data := map[string]interface{}{
			"type":        "like",
			"tweetId":     like.ID,
			"fullText":    like.Text,
			"expandedUrl": expandedUrl,
		}

		records = append(records, types.Record{
			Data:      data,
			Timestamp: timestamp,
			Source:    s.Name(),
		})
	}

	return records, nil
}

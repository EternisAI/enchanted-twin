// owner: slimane@eternis.ai

package x

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/log"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/processor"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
	"github.com/EternisAI/enchanted-twin/pkg/db"
)

const (
	TypeLike          = "like"
	TypeTweet         = "tweets"
	TypeDirectMessage = "direct_messages"
	TypeAccount       = "account"
)

type XProcessor struct {
	store  *db.Store
	logger *log.Logger
}

func NewXProcessor(store *db.Store, logger *log.Logger) processor.Processor {
	return &XProcessor{store: store, logger: logger}
}

func (s *XProcessor) Name() string {
	return "x"
}

func (s *XProcessor) ProcessFile(ctx context.Context, filePath string) ([]types.Record, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	s.logger.Info("Processing file", "filePath", filePath)
	s.logger.Info("Content length", "length", len(content))

	fileType := ""
	fileName := filepath.Base(filePath)
	switch {
	case strings.Contains(fileName, "like"):
		fileType = TypeLike
	case strings.Contains(fileName, "tweets"):
		fileType = TypeTweet
	case strings.Contains(fileName, "direct-messages"):
		fileType = TypeDirectMessage
	case strings.Contains(fileName, "account"):
		fileType = TypeAccount
	default:
		return nil, fmt.Errorf("unsupported X/Twitter file type: %s", fileName)
	}

	s.logger.Info("Detected file type", "fileType", fileType)

	if fileType == TypeAccount {
		err := s.processAccountFile(ctx, content)
		if err != nil {
			s.logger.Warn("Error processing account file", "error", err)
			return nil, err
		}
		s.logger.Info("Successfully processed account file")
		return []types.Record{}, nil
	}

	records, err := parseTwitterFileSimple(content, fileType)
	if err != nil {
		s.logger.Warn("Simple parser failed, trying regex parser", "error", err)
		records, err = parseTwitterFile(content, fileType)
		if err != nil {
			s.logger.Warn("Error processing", "fileType", fileType, "error", err)
			return nil, err
		}
	}

	s.logger.Info("Successfully processed", "fileType", fileType, "records", len(records))
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

func (s *XProcessor) ProcessDirectory(ctx context.Context, inputPath string) ([]types.Record, error) {
	var allRecords []types.Record

	err := filepath.Walk(inputPath, func(path string, info os.FileInfo, err error) error {
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

		records, err := s.ProcessFile(ctx, path)
		if err != nil {
			s.logger.Warn("Failed to process file", "path", path, "error", err)
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
	if strings.HasPrefix(fileName, "._") {
		return false
	}

	supportedFiles := []string{"like.js", "tweets.js", "direct-messages.js", "account.js"}
	for _, supported := range supportedFiles {
		if fileName == supported {
			return true
		}
	}
	return false
}

type Like struct {
	Like struct {
		TweetID     string `json:"tweetId"`
		FullText    string `json:"fullText"`
		ExpandedURL string `json:"expandedUrl"`
	} `json:"like"`
}

type Tweet struct {
	Tweet struct {
		CreatedAt     string `json:"created_at"`
		ID            string `json:"id_str"`
		FullText      string `json:"full_text"`
		RetweetCount  string `json:"retweet_count"`
		FavoriteCount string `json:"favorite_count"`
		Lang          string `json:"lang"`
	} `json:"tweet"`
}

type DMConversation struct {
	DMConversation struct {
		ConversationID string `json:"conversationId"`
		Messages       []struct {
			MessageCreate struct {
				SenderID    string `json:"senderId"`
				RecipientID string `json:"recipientId"`
				Text        string `json:"text"`
				CreatedAt   string `json:"createdAt"`
			} `json:"messageCreate"`
		} `json:"messages"`
	} `json:"dmConversation"`
}

type TwitterUserResponse struct {
	Data []struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Username string `json:"username"`
	} `json:"data"`
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

type Account struct {
	Account struct {
		Email              string `json:"email"`
		CreatedVia         string `json:"createdVia"`
		Username           string `json:"username"`
		AccountID          string `json:"accountId"`
		CreatedAt          string `json:"createdAt"`
		AccountDisplayName string `json:"accountDisplayName"`
	} `json:"account"`
}

func (s *XProcessor) ToDocuments(ctx context.Context, records []types.Record) ([]memory.Document, error) {
	documents := make([]memory.TextDocument, 0, len(records))
	for _, record := range records {
		content := ""
		metadata := map[string]string{}
		tags := []string{"social"}
		var docID string

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
			docID = fmt.Sprintf("x-like-%s", tweetId)
			metadata = map[string]string{
				"type": "like",
				"id":   tweetId,
				"url":  getString("expandedUrl"),
			}
			tags = append(tags, "like")

		case "tweet":
			content = getString("fullText")
			id := getString("id")
			docID = fmt.Sprintf("x-tweet-%s", id)
			favoriteCount := getString("favoriteCount")
			retweetCount := getString("retweetCount")
			metadata = map[string]string{
				"type":          "tweet",
				"id":            id,
				"favoriteCount": favoriteCount,
				"retweetCount":  retweetCount,
			}
			tags = append(tags, "tweet")

		case "directMessage":
			content = getString("text")
			conversationId := getString("conversationId")
			senderId := getString("senderId")
			// Use conversation + sender + timestamp for unique DM ID
			docID = fmt.Sprintf("x-dm-%s-%s-%d", conversationId, senderId, record.Timestamp.Unix())
			metadata = map[string]string{
				"type": "direct_message",
			}
			tags = append(tags, "direct_message")
		}

		// Fallback ID if none was set
		if docID == "" {
			docID = fmt.Sprintf("x-%s-%d", recordType, record.Timestamp.Unix())
		}

		documents = append(documents, memory.TextDocument{
			FieldID:        docID,
			FieldSource:    "x",
			FieldContent:   content,
			FieldTimestamp: &record.Timestamp,
			FieldTags:      tags,
			FieldMetadata:  metadata,
		})
	}

	var documents_ []memory.Document
	for _, document := range documents {
		documents_ = append(documents_, &document)
	}

	return documents_, nil
}

func (s *XProcessor) extractUsername(ctx context.Context, account Account) (string, error) {
	if s.store == nil {
		return "", fmt.Errorf("store is nil")
	}

	extractedUsername := ""
	if account.Account.Username != "" {
		sourceUsername := db.SourceUsername{
			Source:   s.Name(),
			Username: account.Account.Username,
		}

		if account.Account.AccountID != "" {
			sourceUsername.UserID = &account.Account.AccountID
		}
		if account.Account.AccountDisplayName != "" {
			sourceUsername.FirstName = &account.Account.AccountDisplayName
		}
		if account.Account.Email != "" {
			sourceUsername.PhoneNumber = &account.Account.Email
		}
		if account.Account.CreatedVia != "" {
			sourceUsername.Bio = &account.Account.CreatedVia
		}

		s.logger.Info("Saving username to database", "sourceUsername", sourceUsername)

		if err := s.store.SetSourceUsername(ctx, sourceUsername); err != nil {
			s.logger.Warn("Failed to save username to database", "error", err)
			return "", err
		}

		extractedUsername = account.Account.Username
	}

	return extractedUsername, nil
}

func (s *XProcessor) processAccountFile(ctx context.Context, content []byte) error {
	contentStr := string(content)

	arrayPrefix := "window.YTD.account.part0 = "
	if !strings.Contains(contentStr, arrayPrefix) {
		return fmt.Errorf("invalid format: JavaScript array prefix not found")
	}

	contentStr = strings.TrimPrefix(contentStr, arrayPrefix)

	accountStart := strings.Index(contentStr, `"account"`)
	if accountStart == -1 {
		return fmt.Errorf("account object not found")
	}

	usernameRegex := regexp.MustCompile(`"username"\s*:\s*"([^"]+)"`)
	usernameMatch := usernameRegex.FindStringSubmatch(contentStr)
	if len(usernameMatch) < 2 {
		return fmt.Errorf("username not found in account file")
	}

	emailRegex := regexp.MustCompile(`"email"\s*:\s*"([^"]+)"`)
	emailMatch := emailRegex.FindStringSubmatch(contentStr)

	accountIdRegex := regexp.MustCompile(`"accountId"\s*:\s*"([^"]+)"`)
	accountIdMatch := accountIdRegex.FindStringSubmatch(contentStr)

	displayNameRegex := regexp.MustCompile(`"accountDisplayName"\s*:\s*"([^"]+)"`)
	displayNameMatch := displayNameRegex.FindStringSubmatch(contentStr)

	createdViaRegex := regexp.MustCompile(`"createdVia"\s*:\s*"([^"]+)"`)
	createdViaMatch := createdViaRegex.FindStringSubmatch(contentStr)

	account := Account{
		Account: struct {
			Email              string `json:"email"`
			CreatedVia         string `json:"createdVia"`
			Username           string `json:"username"`
			AccountID          string `json:"accountId"`
			CreatedAt          string `json:"createdAt"`
			AccountDisplayName string `json:"accountDisplayName"`
		}{
			Username: usernameMatch[1],
		},
	}

	if len(emailMatch) >= 2 {
		account.Account.Email = emailMatch[1]
	}
	if len(accountIdMatch) >= 2 {
		account.Account.AccountID = accountIdMatch[1]
	}
	if len(displayNameMatch) >= 2 {
		account.Account.AccountDisplayName = displayNameMatch[1]
	}
	if len(createdViaMatch) >= 2 {
		account.Account.CreatedVia = createdViaMatch[1]
	}

	_, err := s.extractUsername(ctx, account)
	if err != nil {
		return fmt.Errorf("failed to extract username: %w", err)
	}

	return nil
}

func (s *XProcessor) Sync(ctx context.Context, accessToken string) ([]types.Record, bool, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	userReq, err := http.NewRequestWithContext(
		ctx,
		"GET",
		"https://api.twitter.com/2/users/me",
		nil,
	)
	if err != nil {
		return nil, false, fmt.Errorf("failed to create user request: %w", err)
	}

	s.logger.Info("Making request with accessToken", "accessToken", accessToken)
	userReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	userResp, err := client.Do(userReq)
	if err != nil {
		return nil, false, fmt.Errorf("failed to fetch user data: %w", err)
	}
	defer func() { _ = userResp.Body.Close() }()

	bodyBytes, err := io.ReadAll(userResp.Body)
	if err != nil {
		return nil, false, fmt.Errorf("failed to read response body: %w", err)
	}

	s.logger.Info("Response Status", "statusCode", userResp.StatusCode)
	s.logger.Info("Response Body", "body", string(bodyBytes))

	userResp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	if userResp.StatusCode == 401 {
		return nil, false, fmt.Errorf(
			"failed to fetch user data. Status: %d, Response: %s",
			userResp.StatusCode,
			string(bodyBytes),
		)
	}

	if userResp.StatusCode != http.StatusOK {
		return nil, true, fmt.Errorf(
			"failed to fetch user data. Status: %d, Response: %s",
			userResp.StatusCode,
			string(bodyBytes),
		)
	}
	s.logger.Info("userResp", "body", userResp.Body)

	var userResponse struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(userResp.Body).Decode(&userResponse); err != nil {
		return nil, true, fmt.Errorf("failed to decode user response: %w", err)
	}

	s.logger.Info("Retrieved user ID", "id", userResponse.Data.ID)

	var records []types.Record

	tweetsReq, err := http.NewRequestWithContext(
		ctx,
		"GET",
		fmt.Sprintf("https://api.twitter.com/2/users/%s/tweets", userResponse.Data.ID),
		nil,
	)
	if err != nil {
		return nil, true, fmt.Errorf("failed to create tweets request: %w", err)
	}

	tweetsReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	q := tweetsReq.URL.Query()
	q.Set("max_results", "50")
	q.Set("tweet.fields", "created_at,public_metrics,lang")
	tweetsReq.URL.RawQuery = q.Encode()

	tweetsResp, err := client.Do(tweetsReq)
	if err != nil {
		return nil, true, fmt.Errorf("failed to fetch tweets: %w", err)
	}
	defer func() { _ = tweetsResp.Body.Close() }()

	if tweetsResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(tweetsResp.Body)
		return nil, true, fmt.Errorf(
			"failed to fetch tweets. Status: %d, Response: %s",
			tweetsResp.StatusCode,
			string(body),
		)
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
		return nil, true, fmt.Errorf("failed to decode tweets response: %w", err)
	}

	for _, tweet := range tweetsResponse.Data {
		timestamp, err := time.Parse(time.RFC3339, tweet.CreatedAt)
		if err != nil {
			s.logger.Warn("Failed to parse tweet timestamp", "error", err)
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

	likesReq, err := http.NewRequestWithContext(
		ctx,
		"GET",
		fmt.Sprintf(
			"https://api.twitter.com/2/users/%s/liked_tweets?tweet.fields=created_at,public_metrics&max_results=10",
			userResponse.Data.ID,
		),
		nil,
	)
	if err != nil {
		return nil, true, fmt.Errorf("failed to create likes request: %w", err)
	}

	likesReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	q = likesReq.URL.Query()
	q.Set("max_results", "50")
	q.Set("tweet.fields", "created_at,public_metrics,lang,entities")
	likesReq.URL.RawQuery = q.Encode()

	likesResp, err := client.Do(likesReq)
	if err != nil {
		return nil, true, fmt.Errorf("failed to fetch likes: %w", err)
	}
	defer func() { _ = likesResp.Body.Close() }()

	if likesResp.StatusCode == 401 {
		return nil, false, fmt.Errorf(
			"failed to fetch likes. Status: %d",
			likesResp.StatusCode,
		)
	}
	if likesResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(likesResp.Body)
		return nil, true, fmt.Errorf(
			"failed to fetch likes. Status: %d, Response: %s",
			likesResp.StatusCode,
			string(body),
		)
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
		return nil, true, fmt.Errorf("failed to decode likes response: %w", err)
	}

	for _, like := range likesResponse.Data {
		timestamp, err := time.Parse(time.RFC3339, like.CreatedAt)
		if err != nil {
			s.logger.Warn("Failed to parse like timestamp", "error", err)
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

	sort.Slice(records, func(i, j int) bool {
		return records[i].Timestamp.After(records[j].Timestamp)
	})

	return records, true, nil
}

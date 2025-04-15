package x

import (
	"fmt"
	"strings"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/dataimport"
)

func parseTwitterFileSimple(content []byte, fileType string, userName string) ([]dataimport.Record, error) {
	contentStr := string(content)

	var arrayPrefix string
	switch fileType {
	case TypeLike:
		arrayPrefix = "window.YTD.like.part0 = "
	case TypeTweet:
		arrayPrefix = "window.YTD.tweets.part0 = "
	case TypeDirectMessage:
		arrayPrefix = "window.YTD.direct_messages.part0 = "
	default:
		return nil, fmt.Errorf("unsupported file type: %s", fileType)
	}

	if !strings.Contains(contentStr, arrayPrefix) {
		return nil, fmt.Errorf("invalid format: JavaScript array prefix not found")
	}

	contentStr = strings.TrimPrefix(contentStr, arrayPrefix)

	switch fileType {
	case TypeLike:
		return parseLikesSimple(contentStr, userName)
	case TypeTweet:
		return parseTweets(contentStr, userName)
	case TypeDirectMessage:
		return parseDirectMessages(contentStr, userName)
	default:
		return nil, fmt.Errorf("unsupported file type: %s", fileType)
	}
}

func parseLikesSimple(content string, userName string) ([]dataimport.Record, error) {
	var records []dataimport.Record
	now := time.Now()

	parts := strings.Split(strings.TrimSpace(content), "},")

	fmt.Println("len(parts)", len(parts))

	if len(parts) > 0 {
		parts[0] = strings.TrimPrefix(strings.TrimSpace(parts[0]), "[")
		if len(parts) > 1 {
			parts[len(parts)-1] = strings.TrimSuffix(strings.TrimSpace(parts[len(parts)-1]), "}]")
		} else {
			parts[0] = strings.TrimSuffix(strings.TrimSpace(parts[0]), "}]")
		}
	}

	fmt.Printf("Split content into %d parts\n", len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		// Skip empty parts
		if part == "" {
			continue
		}

		// Skip if this part doesn't contain a like object
		if !strings.Contains(part, "like") {
			continue
		}

		tweetIdStart := strings.Index(part, "tweetId")
		if tweetIdStart == -1 {
			continue
		}

		// Find the colon after tweetId
		tweetIdStart = strings.Index(part[tweetIdStart:], ":") + tweetIdStart
		if tweetIdStart == -1 {
			continue
		}

		tweetIdStart += 1 // Skip the colon

		// Find the first quote after the colon
		tweetIdQuoteStart := strings.Index(strings.TrimSpace(part[tweetIdStart:]), "\"") + tweetIdStart
		if tweetIdQuoteStart == -1 {
			continue
		}

		tweetIdStart = tweetIdQuoteStart + 1
		tweetIdEnd := strings.Index(part[tweetIdStart:], "\"")
		if tweetIdEnd == -1 {
			continue
		}

		tweetId := part[tweetIdStart : tweetIdStart+tweetIdEnd]

		fullTextStart := strings.Index(part, "fullText")
		if fullTextStart == -1 {
			continue
		}

		// Find the colon after fullText
		fullTextStart = strings.Index(part[fullTextStart:], ":") + fullTextStart
		if fullTextStart == -1 {
			continue
		}

		fullTextStart += 1 // Skip the colon
		fullTextStart = strings.IndexAny(strings.TrimSpace(part[fullTextStart:]), "\"'") + fullTextStart
		if fullTextStart == -1 {
			continue
		}

		quote := part[fullTextStart : fullTextStart+1]
		fullTextStart += 1

		var fullTextEnd int
		if quote == "\"" {
			fullTextEnd = strings.Index(part[fullTextStart:], "\"")
		} else {
			fullTextEnd = strings.Index(part[fullTextStart:], "'")
		}
		if fullTextEnd == -1 {
			continue
		}

		fullText := part[fullTextStart : fullTextStart+fullTextEnd]

		expandedUrlStart := strings.Index(part, "expandedUrl")
		if expandedUrlStart == -1 {
			continue
		}

		// Find the colon after expandedUrl
		expandedUrlStart = strings.Index(part[expandedUrlStart:], ":") + expandedUrlStart
		if expandedUrlStart == -1 {
			continue
		}

		expandedUrlStart += 1 // Skip the colon

		// Find the first quote after the colon
		expandedUrlQuoteStart := strings.Index(strings.TrimSpace(part[expandedUrlStart:]), "\"") + expandedUrlStart
		if expandedUrlQuoteStart == -1 {
			continue
		}

		expandedUrlStart = expandedUrlQuoteStart + 1
		expandedUrlEnd := strings.Index(part[expandedUrlStart:], "\"")
		if expandedUrlEnd == -1 {
			continue
		}

		expandedUrl := part[expandedUrlStart : expandedUrlStart+expandedUrlEnd]

		data := map[string]interface{}{
			"type":        "like",
			"tweetId":     tweetId,
			"fullText":    fullText,
			"expandedUrl": expandedUrl,
		}

		record := dataimport.Record{
			Data:      data,
			Timestamp: now,
			Source:    "x",
		}

		records = append(records, record)
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("no like records found in the content")
	}

	if len(records) == 1 {
		duplicatedRecord := dataimport.Record{
			Data:      records[0].Data,
			Timestamp: records[0].Timestamp,
			Source:    records[0].Source,
		}
		records = append(records, duplicatedRecord)
	}

	return records, nil
}

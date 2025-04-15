package x

import (
	"fmt"
	"regexp"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/dataimport/types"
)

func parseLikesAlternative(arrayContent string, userName string) ([]types.Record, error) {
	var records []types.Record
	now := time.Now()

	fmt.Printf("Like array content first 100 chars: %s\n", arrayContent[:min(100, len(arrayContent))])

	likeObjRegex := regexp.MustCompile(`\{\s*"?like"?:\s*\{[\s\S]*?tweetId[\s\S]*?fullText[\s\S]*?expandedUrl[\s\S]*?\}\s*\}`)
	likeMatches := likeObjRegex.FindAllString(arrayContent, -1)

	fmt.Printf("Found %d like objects with regex\n", len(likeMatches))

	for i, likeObj := range likeMatches {
		fmt.Printf("Processing like object %d\n", i)

		tweetIDRegex := regexp.MustCompile(`"?tweetId"?:\s*"([^"]+)"`)
		tweetIDMatch := tweetIDRegex.FindStringSubmatch(likeObj)
		if len(tweetIDMatch) < 2 {
			continue
		}
		tweetID := tweetIDMatch[1]

		fullTextRegex := regexp.MustCompile(`"?fullText"?:(?:\s*"([^"]*)"|[^,\n]*'([^']*)')`)
		fullTextMatch := fullTextRegex.FindStringSubmatch(likeObj)
		if len(fullTextMatch) < 2 {
			continue
		}
		fullText := fullTextMatch[1]
		if fullText == "" && len(fullTextMatch) > 2 {
			fullText = fullTextMatch[2] // Handle text in single quotes
		}

		expandedURLRegex := regexp.MustCompile(`"?expandedUrl"?:\s*"([^"]+)"`)
		expandedURLMatch := expandedURLRegex.FindStringSubmatch(likeObj)
		if len(expandedURLMatch) < 2 {
			continue
		}
		expandedURL := expandedURLMatch[1]

		data := map[string]interface{}{
			"type":        "like",
			"tweetId":     tweetID,
			"fullText":    fullText,
			"expandedUrl": expandedURL,
			"userName":    userName,
		}

		record := types.Record{
			Data:      data,
			Timestamp: now,
			Source:    "x",
		}

		records = append(records, record)
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("no like records found in the content")
	}

	return records, nil
}

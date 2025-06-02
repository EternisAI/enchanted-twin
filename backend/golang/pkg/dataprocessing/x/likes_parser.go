package x

import (
	"fmt"
	"regexp"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
)

func parseLikesAlternative(arrayContent string) ([]types.Record, error) {
	var records []types.Record
	now := time.Now()

	fmt.Printf(
		"Like array content first 100 chars: %s\n",
		arrayContent[:min(100, len(arrayContent))],
	)

	// Find like objects using a more robust approach
	likeStartRegex := regexp.MustCompile(`\{\s*"?like"?\s*:\s*\{`)
	likeStartMatches := likeStartRegex.FindAllStringIndex(arrayContent, -1)

	fmt.Printf("Found %d like objects with regex\n", len(likeStartMatches))

	for i, startMatch := range likeStartMatches {
		fmt.Printf("Processing like object %d\n", i)

		// Find the end of this like object by counting braces
		start := startMatch[0]

		// Find the actual end by counting braces from the start of the like content
		likeContentStart := startMatch[1] - 1 // Position of the opening brace of like content
		braceCount := 1
		actualEnd := likeContentStart + 1

		for actualEnd < len(arrayContent) && braceCount > 0 {
			if arrayContent[actualEnd] == '{' {
				braceCount++
			} else if arrayContent[actualEnd] == '}' {
				braceCount--
			}
			actualEnd++
		}

		// Include the closing brace of the outer object
		outerBraceCount := 1
		for actualEnd < len(arrayContent) && outerBraceCount > 0 {
			if arrayContent[actualEnd] == '{' {
				outerBraceCount++
			} else if arrayContent[actualEnd] == '}' {
				outerBraceCount--
			}
			actualEnd++
		}

		likeObj := arrayContent[start:actualEnd]

		// Handle both quoted and unquoted keys
		tweetIDRegex := regexp.MustCompile(`"?tweetId"?\s*:\s*"([^"]+)"`)
		tweetIDMatch := tweetIDRegex.FindStringSubmatch(likeObj)
		if len(tweetIDMatch) < 2 {
			fmt.Printf("Could not find tweetId in like object %d\n", i)
			continue
		}
		tweetID := tweetIDMatch[1]

		// Handle multiline fullText with more flexible regex
		fullTextRegex := regexp.MustCompile(`"?fullText"?\s*:\s*"((?:[^"\\]|\\.|\n)*)"`)
		fullTextMatch := fullTextRegex.FindStringSubmatch(likeObj)
		var fullText string
		if len(fullTextMatch) >= 2 {
			fullText = fullTextMatch[1]
		}

		// If still empty, try multiline with string concatenation
		if fullText == "" {
			multilineRegex := regexp.MustCompile(`"?fullText"?\s*:\s*\n?\s*"([^"]*(?:\n[^"]*)*)"`)
			multilineMatch := multilineRegex.FindStringSubmatch(likeObj)
			if len(multilineMatch) >= 2 {
				fullText = multilineMatch[1]
			}
		}

		expandedURLRegex := regexp.MustCompile(`"?expandedUrl"?\s*:\s*"([^"]*)"`)
		expandedURLMatch := expandedURLRegex.FindStringSubmatch(likeObj)
		var expandedURL string
		if len(expandedURLMatch) >= 2 {
			expandedURL = expandedURLMatch[1]
		}

		fmt.Printf("Extracted like - tweetId: %s, fullText: %.50s..., expandedUrl: %s\n",
			tweetID, fullText, expandedURL)

		data := map[string]interface{}{
			"type":        "like",
			"tweetId":     tweetID,
			"fullText":    fullText,
			"expandedUrl": expandedURL,
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

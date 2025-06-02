package x

import (
	"fmt"
	"regexp"

	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
)

func parseTwitterFile(content []byte, fileType string) ([]types.Record, error) {
	contentStr := string(content)

	var arrayRegex *regexp.Regexp
	switch fileType {
	case TypeLike:
		arrayRegex = regexp.MustCompile(`window\.YTD\.like\.part0\s*=\s*(\[[\s\S]*\]);?`)
	case TypeTweet:
		arrayRegex = regexp.MustCompile(`window\.YTD\.tweets\.part0\s*=\s*(\[[\s\S]*\]);?`)
	case TypeDirectMessage:
		arrayRegex = regexp.MustCompile(`window\.YTD\.direct_messages\.part0\s*=\s*(\[[\s\S]*\]);?`)
	default:
		return nil, fmt.Errorf("unsupported file type: %s", fileType)
	}

	arrayMatches := arrayRegex.FindStringSubmatch(contentStr)
	if len(arrayMatches) < 2 {
		return nil, fmt.Errorf("invalid format: JavaScript array not found")
	}

	arrayContent := arrayMatches[1]

	switch fileType {
	case TypeLike:
		return parseLikesAlternative(arrayContent)
	case TypeTweet:
		return parseTweets(arrayContent)
	case TypeDirectMessage:
		return parseDirectMessages(arrayContent)
	default:
		return nil, fmt.Errorf("unsupported file type: %s", fileType)
	}
}

func parseTweets(arrayContent string) ([]types.Record, error) {
	var records []types.Record

	tweetStartRegex := regexp.MustCompile(`\{\s*"?tweet"?\s*:\s*\{`)
	tweetStartMatches := tweetStartRegex.FindAllStringIndex(arrayContent, -1)

	fmt.Printf("Found %d tweet objects\n", len(tweetStartMatches))

	for i, startMatch := range tweetStartMatches {
		fmt.Printf("Processing tweet object %d\n", i)

		start := startMatch[0]
		end := len(arrayContent)
		if i < len(tweetStartMatches)-1 {
			end = tweetStartMatches[i+1][0]
		}

		tweetObj := arrayContent[start:end]

		tweetContentStart := startMatch[1] - 1 // Position of the opening brace of tweet content
		braceCount := 1
		actualEnd := tweetContentStart + 1

		for actualEnd < len(arrayContent) && braceCount > 0 {
			if arrayContent[actualEnd] == '{' {
				braceCount++
			} else if arrayContent[actualEnd] == '}' {
				braceCount--
			}
			actualEnd++
		}

		outerBraceCount := 1
		for actualEnd < len(arrayContent) && outerBraceCount > 0 {
			if arrayContent[actualEnd] == '{' {
				outerBraceCount++
			} else if arrayContent[actualEnd] == '}' {
				outerBraceCount--
			}
			actualEnd++
		}

		tweetObj = arrayContent[start:actualEnd]

		createdAtRegex := regexp.MustCompile(`"?created_at"?\s*:\s*"([^"]+)"`)
		createdAtMatch := createdAtRegex.FindStringSubmatch(tweetObj)
		if len(createdAtMatch) < 2 {
			fmt.Printf("Could not find created_at in tweet %d\n", i)
			continue
		}
		createdAt := createdAtMatch[1]

		idRegex := regexp.MustCompile(`"?id_str"?\s*:\s*"([^"]+)"`)
		idMatch := idRegex.FindStringSubmatch(tweetObj)
		if len(idMatch) < 2 {
			fmt.Printf("Could not find id_str in tweet %d\n", i)
			continue
		}
		id := idMatch[1]

		fullTextRegex := regexp.MustCompile(`"?full_text"?\s*:\s*"((?:[^"\\]|\\.)*)"|"?full_text"?\s*:\s*\n?\s*"([\s\S]*?)"`)
		fullTextMatch := fullTextRegex.FindStringSubmatch(tweetObj)
		var fullText string
		if len(fullTextMatch) >= 2 {
			if fullTextMatch[1] != "" {
				fullText = fullTextMatch[1]
			} else if len(fullTextMatch) > 2 && fullTextMatch[2] != "" {
				fullText = fullTextMatch[2]
			}
		}

		retweetCountRegex := regexp.MustCompile(`"?retweet_count"?\s*:\s*"([^"]*)"`)
		retweetCountMatch := retweetCountRegex.FindStringSubmatch(tweetObj)
		var retweetCount string
		if len(retweetCountMatch) >= 2 {
			retweetCount = retweetCountMatch[1]
		}

		favoriteCountRegex := regexp.MustCompile(`"?favorite_count"?\s*:\s*"([^"]*)"`)
		favoriteCountMatch := favoriteCountRegex.FindStringSubmatch(tweetObj)
		var favoriteCount string
		if len(favoriteCountMatch) >= 2 {
			favoriteCount = favoriteCountMatch[1]
		}

		langRegex := regexp.MustCompile(`"?lang"?\s*:\s*"([^"]*)"`)
		langMatch := langRegex.FindStringSubmatch(tweetObj)
		var lang string
		if len(langMatch) >= 2 {
			lang = langMatch[1]
		}

		timestamp, err := ParseTwitterTimestamp(createdAt)
		if err != nil {
			fmt.Printf("Warning: Failed to parse tweet timestamp: %v\n", err)
			continue
		}

		fmt.Printf("Extracted tweet - id: %s, fullText: %.50s...\n", id, fullText)

		data := map[string]interface{}{
			"type":          "tweet",
			"id":            id,
			"fullText":      fullText,
			"retweetCount":  retweetCount,
			"favoriteCount": favoriteCount,
			"lang":          lang,
		}

		record := types.Record{
			Data:      data,
			Timestamp: timestamp,
			Source:    "x",
		}

		records = append(records, record)
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("no tweet records found in the content")
	}

	return records, nil
}

func parseDirectMessages(arrayContent string) ([]types.Record, error) {
	var records []types.Record

	convRegex := regexp.MustCompile(`\{\s*"?dmConversation"?\s*:\s*\{`)
	convStartPositions := convRegex.FindAllStringIndex(arrayContent, -1)

	if len(convStartPositions) == 0 {
		return nil, fmt.Errorf("no conversation objects found")
	}

	for i, startPos := range convStartPositions {
		endPos := len(arrayContent)
		if i < len(convStartPositions)-1 {
			endPos = convStartPositions[i+1][0]
		}

		convObj := arrayContent[startPos[0]:endPos]

		conversationIdRegex := regexp.MustCompile(
			`"?conversationId"?\s*:\s*(?:"([^"]+)"|'([^']+)')`,
		)
		conversationIdMatch := conversationIdRegex.FindStringSubmatch(convObj)
		if len(conversationIdMatch) < 2 {
			continue
		}
		conversationId := conversationIdMatch[1]
		if conversationId == "" && len(conversationIdMatch) > 2 {
			conversationId = conversationIdMatch[2] // Handle text in single quotes
		}

		messageRegex := regexp.MustCompile(`\{\s*"?messageCreate"?\s*:\s*\{`)
		messageStartPositions := messageRegex.FindAllStringIndex(convObj, -1)

		for j, msgStartPos := range messageStartPositions {
			msgEndPos := len(convObj)
			if j < len(messageStartPositions)-1 {
				msgEndPos = messageStartPositions[j+1][0]
			} else {
				messagesEndRegex := regexp.MustCompile(`\]\s*\}`)
				messagesEndMatch := messagesEndRegex.FindStringIndex(convObj[msgStartPos[0]:])
				if messagesEndMatch != nil {
					msgEndPos = msgStartPos[0] + messagesEndMatch[0]
				}
			}

			messageObj := convObj[msgStartPos[0]:msgEndPos]

			recipientIdRegex := regexp.MustCompile(`"?recipientId"?\s*:\s*(?:"([^"]+)"|'([^']+)')`)
			recipientIdMatch := recipientIdRegex.FindStringSubmatch(messageObj)
			if len(recipientIdMatch) < 2 {
				continue
			}
			recipientId := recipientIdMatch[1]
			if recipientId == "" && len(recipientIdMatch) > 2 {
				recipientId = recipientIdMatch[2]
			}

			textRegex := regexp.MustCompile(`"?text"?\s*:\s*(?:"([^"]*)"|'([^']*)')`)
			textMatch := textRegex.FindStringSubmatch(messageObj)
			if len(textMatch) < 2 {
				continue
			}
			text := textMatch[1]
			if text == "" && len(textMatch) > 2 {
				text = textMatch[2]
			}

			senderIdRegex := regexp.MustCompile(`"?senderId"?\s*:\s*(?:"([^"]+)"|'([^']+)')`)
			senderIdMatch := senderIdRegex.FindStringSubmatch(messageObj)
			if len(senderIdMatch) < 2 {
				continue
			}
			senderId := senderIdMatch[1]
			if senderId == "" && len(senderIdMatch) > 2 {
				senderId = senderIdMatch[2]
			}

			createdAtRegex := regexp.MustCompile(`"?createdAt"?\s*:\s*(?:"([^"]+)"|'([^']+)')`)
			createdAtMatch := createdAtRegex.FindStringSubmatch(messageObj)
			if len(createdAtMatch) < 2 {
				continue
			}
			createdAt := createdAtMatch[1]
			if createdAt == "" && len(createdAtMatch) > 2 {
				createdAt = createdAtMatch[2]
			}

			timestamp, err := ParseTwitterTimestamp(createdAt)
			if err != nil {
				fmt.Printf("Warning: Failed to parse direct message timestamp: %v\n", err)
				continue
			}

			data := map[string]interface{}{
				"type":           "directMessage",
				"conversationId": conversationId,
				"text":           text,
				"senderId":       senderId,
				"recipientId":    recipientId,
			}

			record := types.Record{
				Data:      data,
				Timestamp: timestamp,
				Source:    "x",
			}

			records = append(records, record)
		}
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("no direct message records found in the content")
	}

	return records, nil
}

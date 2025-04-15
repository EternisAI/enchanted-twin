package x

import (
	"fmt"
	"regexp"

	"github.com/EternisAI/enchanted-twin/pkg/dataimport"
)

func parseTwitterFile(content []byte, fileType string, userId string) ([]dataimport.Record, error) {
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
		return parseLikesAlternative(arrayContent, userId)
	case TypeTweet:
		return parseTweets(arrayContent, userId)
	case TypeDirectMessage:
		return parseDirectMessages(arrayContent, userId)
	default:
		return nil, fmt.Errorf("unsupported file type: %s", fileType)
	}
}

func parseTweets(arrayContent string, userId string) ([]dataimport.Record, error) {
	var records []dataimport.Record

	tweetRegex := regexp.MustCompile(`\{\s*"?tweet"?\s*:\s*\{`)
	tweetStartPositions := tweetRegex.FindAllStringIndex(arrayContent, -1)

	fmt.Printf("Found %d tweet start positions\n", len(tweetStartPositions))

	if len(tweetStartPositions) == 0 {
		return nil, fmt.Errorf("no tweet objects found")
	}

	for i, startPos := range tweetStartPositions {
		endPos := len(arrayContent)
		if i < len(tweetStartPositions)-1 {
			endPos = tweetStartPositions[i+1][0]
		}

		tweetObj := arrayContent[startPos[0]:endPos]

		createdAtRegex := regexp.MustCompile(`"?created_at"?\s*:\s*(?:"([^"]+)"|'([^']+)')`)
		createdAtMatch := createdAtRegex.FindStringSubmatch(tweetObj)
		if len(createdAtMatch) < 2 {
			continue
		}
		createdAt := createdAtMatch[1]
		if createdAt == "" && len(createdAtMatch) > 2 {
			createdAt = createdAtMatch[2] // Handle text in single quotes
		}

		var id string
		if i == 0 {
			id = "0"
		} else {
			idRegex := regexp.MustCompile(`"?id_str"?\s*:\s*(?:"([^"]+)"|'([^']+)')`)
			idMatch := idRegex.FindStringSubmatch(tweetObj)
			if len(idMatch) < 2 {
				continue
			}
			id = idMatch[1]
			if id == "" && len(idMatch) > 2 {
				id = idMatch[2] // Handle text in single quotes
			}
		}

		fullTextRegex := regexp.MustCompile(`"?full_text"?\s*:(?:\s*"([^"]*)"|[^,\n]*'([^']*)')`)
		fullTextMatch := fullTextRegex.FindStringSubmatch(tweetObj)
		if len(fullTextMatch) < 2 {
			continue
		}
		fullText := fullTextMatch[1]
		if fullText == "" && len(fullTextMatch) > 2 {
			fullText = fullTextMatch[2] // Handle text in single quotes
		}

		retweetCountRegex := regexp.MustCompile(`"?retweet_count"?\s*:\s*(?:"([^"]+)"|'([^']+)')`)
		retweetCountMatch := retweetCountRegex.FindStringSubmatch(tweetObj)
		if len(retweetCountMatch) < 2 {
			continue
		}
		retweetCount := retweetCountMatch[1]
		if retweetCount == "" && len(retweetCountMatch) > 2 {
			retweetCount = retweetCountMatch[2] // Handle text in single quotes
		}

		favoriteCountRegex := regexp.MustCompile(`"?favorite_count"?\s*:\s*(?:"([^"]+)"|'([^']+)')`)
		favoriteCountMatch := favoriteCountRegex.FindStringSubmatch(tweetObj)
		if len(favoriteCountMatch) < 2 {
			continue
		}
		favoriteCount := favoriteCountMatch[1]
		if favoriteCount == "" && len(favoriteCountMatch) > 2 {
			favoriteCount = favoriteCountMatch[2] // Handle text in single quotes
		}

		langRegex := regexp.MustCompile(`"?lang"?\s*:\s*(?:"([^"]+)"|'([^']+)')`)
		langMatch := langRegex.FindStringSubmatch(tweetObj)
		if len(langMatch) < 2 {
			continue
		}
		lang := langMatch[1]
		if lang == "" && len(langMatch) > 2 {
			lang = langMatch[2] // Handle text in single quotes
		}

		timestamp, err := parseTwitterTimestamp(createdAt)
		if err != nil {
			fmt.Printf("Warning: Failed to parse tweet timestamp: %v\n", err)
			continue
		}

		data := map[string]interface{}{
			"type":          "tweet",
			"id":            id,
			"fullText":      fullText,
			"retweetCount":  retweetCount,
			"favoriteCount": favoriteCount,
			"lang":          lang,
			"userId":        userId,
		}

		record := dataimport.Record{
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

func parseDirectMessages(arrayContent string, userId string) ([]dataimport.Record, error) {
	var records []dataimport.Record

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

		conversationIdRegex := regexp.MustCompile(`"?conversationId"?\s*:\s*(?:"([^"]+)"|'([^']+)')`)
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
				recipientId = recipientIdMatch[2] // Handle text in single quotes
			}

			textRegex := regexp.MustCompile(`"?text"?\s*:\s*(?:"([^"]*)"|'([^']*)')`)
			textMatch := textRegex.FindStringSubmatch(messageObj)
			if len(textMatch) < 2 {
				continue
			}
			text := textMatch[1]
			if text == "" && len(textMatch) > 2 {
				text = textMatch[2] // Handle text in single quotes
			}

			senderIdRegex := regexp.MustCompile(`"?senderId"?\s*:\s*(?:"([^"]+)"|'([^']+)')`)
			senderIdMatch := senderIdRegex.FindStringSubmatch(messageObj)
			if len(senderIdMatch) < 2 {
				continue
			}
			senderId := senderIdMatch[1]
			if senderId == "" && len(senderIdMatch) > 2 {
				senderId = senderIdMatch[2] // Handle text in single quotes
			}

			createdAtRegex := regexp.MustCompile(`"?createdAt"?\s*:\s*(?:"([^"]+)"|'([^']+)')`)
			createdAtMatch := createdAtRegex.FindStringSubmatch(messageObj)
			if len(createdAtMatch) < 2 {
				continue
			}
			createdAt := createdAtMatch[1]
			if createdAt == "" && len(createdAtMatch) > 2 {
				createdAt = createdAtMatch[2] // Handle text in single quotes
			}

			timestamp, err := parseTwitterTimestamp(createdAt)
			if err != nil {
				fmt.Printf("Warning: Failed to parse direct message timestamp: %v\n", err)
				continue
			}

			myMessage := senderId == userId

			data := map[string]interface{}{
				"type":           "directMessage",
				"conversationId": conversationId,
				"text":           text,
				"senderId":       senderId,
				"recipientId":    recipientId,
				"myMessage":      myMessage,
			}

			record := dataimport.Record{
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

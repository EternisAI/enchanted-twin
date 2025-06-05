package timeparser

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	dps "github.com/markusmobius/go-dateparser"
	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"

	"github.com/EternisAI/enchanted-twin/pkg/agent/types"
)

// TimeParserTool implements a tool for parsing time expressions from text.
type TimeParserTool struct {
	Logger *log.Logger
}

// NewTimeParserTool creates a new time parser tool.
func NewTimeParserTool(logger *log.Logger) *TimeParserTool {
	return &TimeParserTool{
		Logger: logger,
	}
}

// Execute runs the time parsing.
func (t *TimeParserTool) Execute(ctx context.Context, input map[string]any) (types.ToolResult, error) {
	textVal, exists := input["text"]
	if !exists {
		return nil, errors.New("text is required")
	}

	text, ok := textVal.(string)
	if !ok {
		return nil, errors.New("text must be a string")
	}

	localTimeZone := time.Local
	if tzVal, exists := input["timezone"]; exists {
		if tzStr, ok := tzVal.(string); ok && tzStr != "" {
			if location, err := time.LoadLocation(tzStr); err == nil {
				localTimeZone = location
			}
		}
	}

	resultText, err := t.parseAndReplaceTimeValues(text, localTimeZone)
	if err != nil {
		return nil, fmt.Errorf("failed to parse time values: %w", err)
	}

	return types.SimpleToolResult(resultText), nil
}

func (t *TimeParserTool) parseAndReplaceTimeValues(text string, localTimeZone *time.Location) (string, error) {
	now := time.Now().In(localTimeZone)
	result := text

	timePatterns := []string{
		`(?i)\b(?:at\s+)?\d{1,2}(?::\d{2})?\s*(?:AM|PM|am|pm)\b`, // at 3pm, 3:30AM
		`\b\d{1,2}:\d{2}\b`,                    // 14:30 (24-hour format)
		`\d{1,2}/\d{1,2}/\d{4}`,                // 1/20/2024 (full year only)
		`\d{4}-\d{1,2}-\d{1,2}`,                // 2024-01-20
		`(?i)\b(?:today|tomorrow|yesterday)\b`, // today, tomorrow
		`(?i)\b(?:next|last)\s+(?:monday|tuesday|wednesday|thursday|friday|saturday|sunday)\b`,                                        // next Monday
		`(?i)\b(?:january|february|march|april|may|june|july|august|september|october|november|december)\s+\d{1,2}(?:st|nd|rd|th)?\b`, // January 20th
		`(?i)\b(?:in\s+\d+\s+(?:minutes?|hours?|days?|weeks?|months?|years?))\b`,                                                      // in 2 hours
		`(?i)\b(?:\d+\s+(?:minutes?|hours?|days?|weeks?|months?|years?)\s+ago)\b`,                                                     // 3 hours ago
	}

	type Match struct {
		Start   int
		End     int
		Text    string
		Pattern string
	}

	var allMatches []Match
	for _, pattern := range timePatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringIndex(result, -1)

		for _, match := range matches {
			start := match[0]
			end := match[1]
			matchText := result[start:end]

			allMatches = append(allMatches, Match{
				Start:   start,
				End:     end,
				Text:    matchText,
				Pattern: pattern,
			})
		}
	}

	for i := 0; i < len(allMatches); i++ {
		for j := i + 1; j < len(allMatches); j++ {
			if allMatches[i].Start < allMatches[j].Start {
				allMatches[i], allMatches[j] = allMatches[j], allMatches[i]
			}
		}
	}

	var filteredMatches []Match
	for _, match := range allMatches {
		isOverlapping := false
		for _, existing := range filteredMatches {
			if (match.Start >= existing.Start && match.Start < existing.End) ||
				(match.End > existing.Start && match.End <= existing.End) ||
				(match.Start <= existing.Start && match.End >= existing.End) {
				isOverlapping = true
				break
			}
		}
		if !isOverlapping {
			filteredMatches = append(filteredMatches, match)
		}
	}

	config := &dps.Configuration{
		CurrentTime:          now,
		DefaultTimezone:      localTimeZone,
		PreferredDayOfMonth:  dps.Current,
		PreferredMonthOfYear: dps.CurrentMonth,
		PreferredDateSource:  dps.CurrentPeriod,
	}

	for _, match := range filteredMatches {
		originalText := match.Text
		start := match.Start
		end := match.End

		extendedMatch := t.extendTimeContext(result, start, end)
		extendedText := result[extendedMatch[0]:extendedMatch[1]]

		parsed, err := dps.Parse(config, extendedText)
		if err != nil || parsed.Time.IsZero() {
			parsed, err = dps.Parse(config, originalText)
			if err != nil || parsed.Time.IsZero() {
				continue
			}
		} else {
			start = extendedMatch[0]
			end = extendedMatch[1]
			originalText = extendedText
		}

		timeDiff := parsed.Time.Sub(now)
		if timeDiff > -365*24*time.Hour && timeDiff < 365*24*time.Hour { // Within 1 year
			utcTime := parsed.Time.UTC()

			beforeChar := ""
			afterChar := ""
			if start > 0 && result[start-1] != ' ' {
				beforeChar = " "
			}
			if end < len(result) && result[end] != ' ' {
				afterChar = " "
			}

			formattedTime := fmt.Sprintf("%s%s UTC (originally %s %s)%s",
				beforeChar,
				utcTime.Format("2006-01-02 15:04:05"),
				originalText,
				localTimeZone.String(),
				afterChar)

			result = result[:start] + formattedTime + result[end:]
		}
	}

	return result, nil
}

func (t *TimeParserTool) extendTimeContext(text string, start, end int) [2]int {
	extendedStart := start
	extendedEnd := end

	// Look backwards for context words
	if start >= 3 && strings.ToLower(text[start-3:start]) == "at " {
		extendedStart = start - 3
	} else if start >= 3 && strings.ToLower(text[start-3:start]) == "on " {
		extendedStart = start - 3
	} else if start >= 5 && strings.ToLower(text[start-5:start]) == "next " {
		extendedStart = start - 5
	} else if start >= 5 && strings.ToLower(text[start-5:start]) == "last " {
		extendedStart = start - 5
	} else if start >= 5 && strings.ToLower(text[start-5:start]) == "this " {
		extendedStart = start - 5
	}

	// Look forwards for AM/PM
	if end+3 <= len(text) && (strings.ToLower(text[end:end+3]) == " am" || strings.ToLower(text[end:end+3]) == " pm") {
		extendedEnd = end + 3
	}

	return [2]int{extendedStart, extendedEnd}
}

// Definition returns the OpenAI tool definition.
func (t *TimeParserTool) Definition() openai.ChatCompletionToolParam {
	return openai.ChatCompletionToolParam{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name: "time_parser_tool",
			Description: param.NewOpt(
				"Parse time values from text and convert them to UTC time. This tool identifies natural language time expressions (like 'tomorrow at 3pm', 'next Monday', 'in 2 hours') and structured time formats (like '14:30', '2024-01-15') in the input text, then replaces them with their UTC equivalents while preserving the original local time context.",
			),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]any{
					"text": map[string]string{
						"type":        "string",
						"description": "The text containing time expressions to parse and convert",
					},
					"timezone": map[string]string{
						"type":        "string",
						"description": "The local timezone to assume for time expressions (e.g., 'America/New_York', 'Europe/London'). Defaults to system local timezone if not provided.",
					},
				},
				"required": []string{"text"},
			},
		},
	}
}

package gmail

import (
	"bufio"
	"fmt"
	"io"
	"net/mail"
	"os"
	"regexp"
	"sort"
	"strings"
)

// EmailAddressFrequency tracks email addresses and their occurrence count.
type EmailAddressFrequency struct {
	Address string
	Count   int
	Sources []string // Track which headers this came from
}

// DetectUserEmailFromMbox analyzes an mbox file to determine the user's email address.
func DetectUserEmailFromMbox(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	// Count delivered-to headers first (most reliable)
	deliveredToCount := make(map[string]int)
	fromCount := make(map[string]int)
	toCount := make(map[string]int)

	var buf strings.Builder
	r := bufio.NewReader(f)
	inEmail := false

	for {
		line, err := r.ReadString('\n')
		if err == io.EOF {
			if inEmail {
				analyzeEmailHeaders(buf.String(), deliveredToCount, fromCount, toCount)
			}
			break
		}
		if err != nil {
			return "", err
		}

		if strings.HasPrefix(line, "From ") {
			if inEmail {
				analyzeEmailHeaders(buf.String(), deliveredToCount, fromCount, toCount)
				buf.Reset()
			}
			inEmail = true
		}

		if inEmail {
			buf.WriteString(line)
		}
	}

	// Priority 1: Delivered-To header (most reliable for Gmail exports)
	if userEmail := getMostFrequentEmail(deliveredToCount); userEmail != "" {
		return userEmail, nil
	}

	// Priority 2: From header analysis (for sent emails)
	if userEmail := getMostFrequentEmail(fromCount); userEmail != "" {
		return userEmail, nil
	}

	// Priority 3: To header analysis (less reliable)
	if userEmail := getMostFrequentEmail(toCount); userEmail != "" {
		return userEmail, nil
	}

	return "", fmt.Errorf("could not determine user email address")
}

// analyzeEmailHeaders extracts email addresses from different headers.
func analyzeEmailHeaders(emailContent string, deliveredTo, from, to map[string]int) {
	msg, err := mail.ReadMessage(strings.NewReader(emailContent))
	if err != nil {
		return
	}

	// Check Delivered-To header (most reliable)
	if deliveredToHeader := msg.Header.Get("Delivered-To"); deliveredToHeader != "" {
		if email := extractEmailAddress(deliveredToHeader); email != "" {
			deliveredTo[email]++
		}
	}

	// Check From header
	if fromHeader := msg.Header.Get("From"); fromHeader != "" {
		if email := extractEmailAddress(fromHeader); email != "" {
			from[email]++
		}
	}

	// Check To header
	if toHeader := msg.Header.Get("To"); toHeader != "" {
		if email := extractEmailAddress(toHeader); email != "" {
			to[email]++
		}
	}
}

// extractEmailAddress extracts email address from header value.
func extractEmailAddress(headerValue string) string {
	if addr, err := mail.ParseAddress(headerValue); err == nil {
		return strings.ToLower(addr.Address)
	}

	emailRegex := regexp.MustCompile(`([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})`)
	if matches := emailRegex.FindStringSubmatch(headerValue); len(matches) > 1 {
		return strings.ToLower(matches[1])
	}

	return ""
}

// getMostFrequentEmail returns the most frequently occurring email address.
func getMostFrequentEmail(emailCount map[string]int) string {
	if len(emailCount) == 0 {
		return ""
	}

	var frequencies []EmailAddressFrequency
	for email, count := range emailCount {
		frequencies = append(frequencies, EmailAddressFrequency{
			Address: email,
			Count:   count,
		})
	}

	sort.Slice(frequencies, func(i, j int) bool {
		return frequencies[i].Count > frequencies[j].Count
	})

	return frequencies[0].Address
}

// DetectUserEmailFromRecords analyzes processed records to determine user email.
func DetectUserEmailFromRecords(records []interface{}) (string, error) {
	deliveredToCount := make(map[string]int)
	fromCount := make(map[string]int)
	toCount := make(map[string]int)

	for _, record := range records {
		if rec, ok := record.(map[string]interface{}); ok {
			if data, ok := rec["data"].(map[string]interface{}); ok {
				if deliveredTo, ok := data["delivered_to"].(string); ok && deliveredTo != "" {
					if email := extractEmailAddress(deliveredTo); email != "" {
						deliveredToCount[email]++
					}
				}

				if from, ok := data["from"].(string); ok && from != "" {
					if email := extractEmailAddress(from); email != "" {
						fromCount[email]++
					}
				}

				if to, ok := data["to"].(string); ok && to != "" {
					if email := extractEmailAddress(to); email != "" {
						toCount[email]++
					}
				}
			}
		}
	}

	if userEmail := getMostFrequentEmail(deliveredToCount); userEmail != "" {
		return userEmail, nil
	}

	if userEmail := getMostFrequentEmail(fromCount); userEmail != "" {
		return userEmail, nil
	}

	if userEmail := getMostFrequentEmail(toCount); userEmail != "" {
		return userEmail, nil
	}

	return "", fmt.Errorf("could not determine user email address from records")
}

// AnalyzeEmailPatterns provides detailed analysis of email patterns for debugging.
func AnalyzeEmailPatterns(path string) (map[string]interface{}, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	deliveredToCount := make(map[string]int)
	fromCount := make(map[string]int)
	toCount := make(map[string]int)
	totalEmails := 0

	var buf strings.Builder
	r := bufio.NewReader(f)
	inEmail := false

	for {
		line, err := r.ReadString('\n')
		if err == io.EOF {
			if inEmail {
				analyzeEmailHeaders(buf.String(), deliveredToCount, fromCount, toCount)
				totalEmails++
			}
			break
		}
		if err != nil {
			return nil, err
		}

		if strings.HasPrefix(line, "From ") {
			if inEmail {
				analyzeEmailHeaders(buf.String(), deliveredToCount, fromCount, toCount)
				totalEmails++
				buf.Reset()
			}
			inEmail = true
		}

		if inEmail {
			buf.WriteString(line)
		}
	}

	return map[string]interface{}{
		"total_emails":   totalEmails,
		"delivered_to":   deliveredToCount,
		"from_addresses": fromCount,
		"to_addresses":   toCount,
		"detected_user":  detectUserFromCounts(deliveredToCount, fromCount, toCount),
	}, nil
}

func detectUserFromCounts(deliveredTo, from, to map[string]int) string {
	if userEmail := getMostFrequentEmail(deliveredTo); userEmail != "" {
		return userEmail
	}
	if userEmail := getMostFrequentEmail(from); userEmail != "" {
		return userEmail
	}
	if userEmail := getMostFrequentEmail(to); userEmail != "" {
		return userEmail
	}
	return ""
}

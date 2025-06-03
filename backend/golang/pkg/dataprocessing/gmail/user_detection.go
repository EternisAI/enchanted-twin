// owner: slimane@eternis.ai

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

const (
	// MaxSampleSize limits how many emails to analyze for user detection.
	MaxSampleSize = 1000
)

// DetectUserEmailFromMbox analyzes an mbox file to determine the user's email address.
// Uses sampling strategy to only process the first MaxSampleSize emails for performance.
func DetectUserEmailFromMbox(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	deliveredToCount := make(map[string]int)
	fromCount := make(map[string]int)
	toCount := make(map[string]int)

	var buf strings.Builder
	r := bufio.NewReader(f)
	inEmail := false
	emailsProcessed := 0

	for {
		line, err := r.ReadString('\n')
		if err == io.EOF {
			if inEmail && emailsProcessed < MaxSampleSize {
				analyzeEmailHeaders(buf.String(), deliveredToCount, fromCount, toCount)
				emailsProcessed++
			}
			break
		}
		if err != nil {
			return "", err
		}

		if strings.HasPrefix(line, "From ") {
			if inEmail {
				if emailsProcessed < MaxSampleSize {
					analyzeEmailHeaders(buf.String(), deliveredToCount, fromCount, toCount)
					emailsProcessed++
					buf.Reset()
				} else {
					break
				}
			}
			inEmail = true
		}

		if inEmail && emailsProcessed < MaxSampleSize {
			buf.WriteString(line)
		}
	}

	fmt.Printf("User detection: analyzed %d emails (sample size: %d)\n", emailsProcessed, MaxSampleSize)

	if userEmail := getMostFrequentEmail(deliveredToCount); userEmail != "" {
		fmt.Printf("User detected from Delivered-To headers: %s (count: %d)\n", userEmail, deliveredToCount[userEmail])
		return userEmail, nil
	}

	if userEmail := getMostFrequentEmail(fromCount); userEmail != "" {
		fmt.Printf("User detected from From headers: %s (count: %d)\n", userEmail, fromCount[userEmail])
		return userEmail, nil
	}

	if userEmail := getMostFrequentEmail(toCount); userEmail != "" {
		fmt.Printf("User detected from To headers: %s (count: %d)\n", userEmail, toCount[userEmail])
		return userEmail, nil
	}

	return "", fmt.Errorf("could not determine user email address from %d sampled emails", emailsProcessed)
}

// analyzeEmailHeaders extracts email addresses from different headers.
func analyzeEmailHeaders(emailContent string, deliveredTo, from, to map[string]int) {
	msg, err := mail.ReadMessage(strings.NewReader(emailContent))
	if err != nil {
		return
	}

	if deliveredToHeader := msg.Header.Get("Delivered-To"); deliveredToHeader != "" {
		if email := extractEmailAddress(deliveredToHeader); email != "" {
			deliveredTo[email]++
		}
	}

	if fromHeader := msg.Header.Get("From"); fromHeader != "" {
		if email := extractEmailAddress(fromHeader); email != "" {
			from[email]++
		}
	}

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

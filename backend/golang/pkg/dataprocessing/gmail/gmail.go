// owner: slimane@eternis.ai

package gmail

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html"
	"os"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/log"
	"github.com/mnako/letters"
	loghtml "golang.org/x/net/html"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/db"
)

type GmailProcessor struct {
	store  *db.Store
	logger *log.Logger
}

type emailWithMeta struct {
	email     *letters.Email
	threadID  string
	timestamp time.Time
}

func NewGmailProcessor(store *db.Store, logger *log.Logger) (*GmailProcessor, error) {
	if logger == nil || store == nil {
		return nil, fmt.Errorf("logger or store is nil")
	}
	return &GmailProcessor{store: store, logger: logger}, nil
}

func (g *GmailProcessor) Name() string { return "gmail" }

func (g *GmailProcessor) ProcessFile(ctx context.Context, path string) ([]memory.ConversationDocument, error) {
	userEmail, err := g.detectUserEmail(path)
	if err != nil {
		g.logger.Warn("Could not detect user email", "error", err)
	}

	emails, err := g.parseAllEmails(path)
	if err != nil {
		return nil, err
	}

	filtered := g.filterEmails(emails, userEmail)
	threads := g.buildThreads(filtered)
	return g.toConversationDocuments(threads, userEmail), nil
}

func (g *GmailProcessor) detectUserEmail(path string) (string, error) {
	userEmail, err := DetectUserEmailFromMbox(path)
	if err != nil {
		return "", err
	}

	if err := g.store.SetSourceUsername(context.Background(), db.SourceUsername{
		Source:   g.Name(),
		Username: userEmail,
	}); err != nil {
		g.logger.Warn("Failed to store user email", "error", err)
	}

	return userEmail, nil
}

func (g *GmailProcessor) parseAllEmails(path string) ([]*emailWithMeta, error) {
	total, err := g.countEmails(path)
	if err != nil {
		return nil, err
	}

	emails := make([]*emailWithMeta, 0, total)
	parser := letters.NewEmailParser(letters.WithFileFilter(letters.NoFiles))

	err = g.processEmailsParallel(path, parser, &emails)
	return emails, err
}

func (g *GmailProcessor) countEmails(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer func() { _ = f.Close() }()

	count := 0
	scanner := bufio.NewScanner(f)

	// Increase buffer size to handle very long email lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024) // 10MB max token size

	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "From ") {
			count++
		}
	}
	return count, scanner.Err()
}

func (g *GmailProcessor) processEmailsParallel(path string, parser *letters.EmailParser, emails *[]*emailWithMeta) error {
	jobs := make(chan string, runtime.NumCPU())
	results := make(chan *emailWithMeta, 100)

	var wg sync.WaitGroup
	for i := 0; i < runtime.NumCPU(); i++ {
		wg.Add(1)
		go g.emailWorker(parser, jobs, results, &wg)
	}

	go g.readEmailsFromFile(path, jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	for email := range results {
		if email != nil {
			*emails = append(*emails, email)
		}
	}

	return nil
}

func (g *GmailProcessor) emailWorker(parser *letters.EmailParser, jobs <-chan string, results chan<- *emailWithMeta, wg *sync.WaitGroup) {
	defer wg.Done()

	for content := range jobs {
		if email := g.parseEmail(parser, content); email != nil {
			results <- email
		}
	}
}

func (g *GmailProcessor) readEmailsFromFile(path string, jobs chan<- string) {
	defer close(jobs)

	f, err := os.Open(path)
	if err != nil {
		g.logger.Error("Failed to open file", "error", err)
		return
	}
	defer func() { _ = f.Close() }()

	var content strings.Builder
	scanner := bufio.NewScanner(f)

	// Increase buffer size to handle very long email lines (e.g., base64 attachments)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024) // 10MB max token size

	inEmail := false

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "From ") {
			if inEmail && content.Len() > 0 {
				jobs <- content.String()
				content.Reset()
			}
			inEmail = true
		} else if inEmail {
			content.WriteString(line + "\n")
		}
	}

	if scanner.Err() != nil {
		g.logger.Error("Scanner error while reading emails", "error", scanner.Err())
		return
	}

	if inEmail && content.Len() > 0 {
		jobs <- content.String()
	}
}

func (g *GmailProcessor) parseEmail(parser *letters.EmailParser, content string) *emailWithMeta {
	email, err := parser.Parse(strings.NewReader(content))
	if err != nil {
		return nil
	}

	if g.shouldSkipEmail(&email) {
		return nil
	}

	threadID := g.extractThreadID(&email)
	return &emailWithMeta{
		email:     &email,
		threadID:  threadID,
		timestamp: email.Headers.Date,
	}
}

func (g *GmailProcessor) shouldSkipEmail(email *letters.Email) bool {
	if g.hasSkipLabels(email) || g.hasSkipSender(email) {
		return true
	}
	return email.Text == "" && email.HTML == ""
}

func (g *GmailProcessor) hasSkipLabels(email *letters.Email) bool {
	skipLabels := []string{"Category Promotions", "Category Social", "Category Updates", "Category Forums"}

	if labels, ok := email.Headers.ExtraHeaders["X-Gmail-Labels"]; ok && len(labels) > 0 {
		for _, skipLabel := range skipLabels {
			if strings.Contains(labels[0], skipLabel) {
				return true
			}
		}
	}
	return false
}

func (g *GmailProcessor) hasSkipSender(email *letters.Email) bool {
	skipPatterns := []string{
		"no-reply", "noreply", "no_reply", "support", "alert", "notification",
		"ticket", "update", "info@", "contact@", "billing@", "newsletter",
		"news@", "auto@", "auto-", "mms@", "sms@", "mail@", "server@",
		"bounce", "daemon",
	}

	senders := email.Headers.From
	if email.Headers.Sender != nil {
		senders = append(senders, email.Headers.Sender)
	}

	for _, sender := range senders {
		if sender != nil {
			lowerAddr := strings.ToLower(sender.Address)
			for _, pattern := range skipPatterns {
				if strings.Contains(lowerAddr, pattern) {
					return true
				}
			}
		}
	}
	return false
}

func (g *GmailProcessor) extractThreadID(email *letters.Email) string {
	if len(email.Headers.References) > 0 {
		return string(email.Headers.References[0])
	}
	if len(email.Headers.InReplyTo) > 0 {
		return string(email.Headers.InReplyTo[0])
	}
	if email.Headers.MessageID != "" {
		return string(email.Headers.MessageID)
	}
	return g.subjectThreadID(email.Headers.Subject)
}

func (g *GmailProcessor) subjectThreadID(subject string) string {
	normalized := strings.ToLower(strings.TrimSpace(subject))
	normalized = strings.TrimPrefix(normalized, "re:")
	normalized = strings.TrimPrefix(normalized, "fwd:")
	normalized = strings.TrimSpace(normalized)

	h := sha256.Sum256([]byte(normalized))
	return fmt.Sprintf("subject-%x", h[:8])
}

func (g *GmailProcessor) filterEmails(emails []*emailWithMeta, userEmail string) []*emailWithMeta {
	if userEmail == "" {
		return emails
	}

	userLower := strings.ToLower(userEmail)
	senderCounts := make(map[string]int)
	recipientsOfMyEmails := make(map[string]bool)

	// Phase 1: Count senders and track user interactions
	for _, email := range emails {
		// Count senders (deduplicated per email)
		seenInThisEmail := make(map[string]bool)
		for _, from := range email.email.Headers.From {
			if from != nil && from.Address != "" {
				lowerAddr := strings.ToLower(from.Address)
				if !seenInThisEmail[lowerAddr] {
					senderCounts[lowerAddr]++
					seenInThisEmail[lowerAddr] = true
				}
			}
		}

		// Track recipients if email was sent BY the user
		if g.isUserEmail(email.email, userLower) {
			for _, to := range email.email.Headers.To {
				if to != nil && to.Address != "" {
					recipientsOfMyEmails[strings.ToLower(to.Address)] = true
				}
			}
			for _, cc := range email.email.Headers.Cc {
				if cc != nil && cc.Address != "" {
					recipientsOfMyEmails[strings.ToLower(cc.Address)] = true
				}
			}
		}
	}

	// Phase 2: Filter based on interaction OR frequency
	const minEmailCount = 5
	var filtered []*emailWithMeta
	for _, email := range emails {
		keep := false
		for _, from := range email.email.Headers.From {
			if from != nil && from.Address != "" {
				lowerAddr := strings.ToLower(from.Address)
				// Keep if: high frequency OR user interacted with them
				if senderCounts[lowerAddr] > minEmailCount || recipientsOfMyEmails[lowerAddr] {
					keep = true
					break
				}
			}
		}
		if keep {
			filtered = append(filtered, email)
		}
	}

	return filtered
}

func (g *GmailProcessor) isUserEmail(email *letters.Email, userEmail string) bool {
	for _, from := range email.Headers.From {
		if from != nil && strings.ToLower(from.Address) == userEmail {
			return true
		}
	}
	return false
}

func (g *GmailProcessor) buildThreads(emails []*emailWithMeta) map[string][]*emailWithMeta {
	threads := make(map[string][]*emailWithMeta)

	for _, email := range emails {
		threads[email.threadID] = append(threads[email.threadID], email)
	}

	for threadID := range threads {
		sort.Slice(threads[threadID], func(i, j int) bool {
			return threads[threadID][i].timestamp.Before(threads[threadID][j].timestamp)
		})
	}

	return threads
}

func (g *GmailProcessor) toConversationDocuments(threads map[string][]*emailWithMeta, userEmail string) []memory.ConversationDocument {
	var docs []memory.ConversationDocument

	for threadID, emails := range threads {
		if len(emails) == 0 {
			continue
		}

		doc := memory.ConversationDocument{
			FieldID:      fmt.Sprintf("gmail-thread-%s", threadID),
			FieldSource:  "gmail",
			FieldTags:    []string{"email", "conversation"},
			User:         userEmail,
			People:       g.extractPeople(emails),
			Conversation: g.buildConversation(emails),
			FieldMetadata: map[string]string{
				"thread_id": threadID,
				"subject":   emails[0].email.Headers.Subject,
			},
		}

		docs = append(docs, doc)
	}

	return docs
}

func (g *GmailProcessor) extractPeople(emails []*emailWithMeta) []string {
	people := make(map[string]bool)

	for _, email := range emails {
		for _, addr := range email.email.Headers.From {
			if addr != nil && addr.Address != "" {
				people[addr.Address] = true
			}
		}
		for _, addr := range email.email.Headers.To {
			if addr != nil && addr.Address != "" {
				people[addr.Address] = true
			}
		}
	}

	var result []string
	for person := range people {
		result = append(result, person)
	}
	return result
}

func (g *GmailProcessor) buildConversation(emails []*emailWithMeta) []memory.ConversationMessage {
	var messages []memory.ConversationMessage

	for _, email := range emails {
		content := email.email.Text
		if content == "" {
			content = g.htmlToText(email.email.HTML)
		}

		if content = strings.TrimSpace(content); content != "" {
			messages = append(messages, memory.ConversationMessage{
				Content: content,
				Speaker: g.getSpeaker(email.email),
				Time:    email.timestamp,
			})
		}
	}

	return messages
}

func (g *GmailProcessor) getSpeaker(email *letters.Email) string {
	if len(email.Headers.From) > 0 && email.Headers.From[0] != nil {
		return email.Headers.From[0].Address
	}
	if email.Headers.Sender != nil {
		return email.Headers.Sender.Address
	}
	return "unknown"
}

func (g *GmailProcessor) htmlToText(htmlContent string) string {
	// Decode common HTML entities and remaining UTF-8 sequences
	r := strings.NewReplacer(
		"=E2=80=99", "'",
		"=E2=9A=BD", "⚽",
		"=EF=B8=8F", "",
		"&nbsp;", " ",
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", "\"",
		"&apos;", "'",
	)
	htmlContent = r.Replace(htmlContent)

	// Try to decode any remaining UTF-8 sequences
	if decodedContent, err := g.decodeUTF8Sequences(htmlContent); err == nil {
		htmlContent = decodedContent
	}

	// Unescape basic HTML entities
	htmlContent = html.UnescapeString(htmlContent)

	doc, err := loghtml.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return ""
	}

	var textBuilder strings.Builder
	var lastText string
	var extract func(*loghtml.Node)
	extract = func(n *loghtml.Node) {
		switch n.Type {
		case loghtml.ElementNode:
			// Skip non-content tags
			switch strings.ToLower(n.Data) {
			case "style", "script", "noscript", "iframe", "head", "meta", "link":
				return
			}
		case loghtml.TextNode:
			text := strings.TrimSpace(n.Data)
			if text != "" {
				if lastText != "" && !strings.HasSuffix(lastText, "\n") {
					textBuilder.WriteString(" ")
				}
				textBuilder.WriteString(text)
				lastText = text
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}

		// Add newline after block elements
		if n.Type == loghtml.ElementNode {
			switch strings.ToLower(n.Data) {
			case "p", "div", "br", "h1", "h2", "h3", "h4", "h5", "h6", "li":
				if lastText != "" && !strings.HasSuffix(lastText, "\n") {
					textBuilder.WriteString("\n")
					lastText = "\n"
				}
			}
		}
	}
	extract(doc)

	result := textBuilder.String()
	result = strings.Join(strings.Fields(result), " ")
	for strings.Contains(result, "\n\n") {
		result = strings.ReplaceAll(result, "\n\n", "\n")
	}
	return strings.TrimSpace(result)
}

func (g *GmailProcessor) decodeUTF8Sequences(text string) (string, error) {
	re := regexp.MustCompile(`=([0-9A-F]{2})(=([0-9A-F]{2}))?(=([0-9A-F]{2}))?`)
	return re.ReplaceAllStringFunc(text, func(match string) string {
		parts := re.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}

		bytes := make([]byte, 0, 3)
		for i := 1; i < len(parts); i += 2 {
			if parts[i] != "" {
				b, err := hex.DecodeString(parts[i])
				if err != nil {
					return match
				}
				bytes = append(bytes, b[0])
			}
		}

		if str := string(bytes); utf8.ValidString(str) {
			return str
		}
		return match
	}), nil
}

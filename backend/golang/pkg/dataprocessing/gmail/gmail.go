// owner: slimane@eternis.ai

package gmail

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
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

type SenderDetail struct {
	Email       string `json:"email"`
	Count       int    `json:"count"`
	Interaction bool   `json:"interaction"`
	Reason      string `json:"reason,omitempty"`
}

type emailJob struct {
	emailIndex int
	emailData  string
}

type emailResult struct {
	emailIndex   int
	email        *emailWithMeta
	originalData string
	originalSize int
	err          error
	duration     time.Duration
}

type SkipReason struct {
	Reason   string   `json:"reason"`
	Count    int      `json:"count"`
	Examples []string `json:"examples,omitempty"`
}

const (
	processTimeout      = time.Second
	progressBarWidth    = 50
	saveSkippedAnalysis = false // Set to true for debugging skipped emails
)

func NewGmailProcessor(store *db.Store, logger *log.Logger) (*GmailProcessor, error) {
	if logger == nil || store == nil {
		return nil, fmt.Errorf("logger or store is nil")
	}
	return &GmailProcessor{store: store, logger: logger}, nil
}

func (g *GmailProcessor) Name() string { return "gmail" }

// Simple interface - unchanged API.
func (g *GmailProcessor) ProcessFile(ctx context.Context, path string) ([]memory.ConversationDocument, error) {
	return g.processFileInternal(ctx, path, "pipeline_output", false)
}

// New method for sender analysis only.
func (g *GmailProcessor) ProcessFileForSenders(ctx context.Context, path string, outputDir string) error {
	_, err := g.processFileInternal(ctx, path, outputDir, true)
	return err
}

// normalizeGmailAddress normalizes Gmail addresses by removing dots and converting to lowercase.
func normalizeGmailAddress(email string) string {
	email = strings.ToLower(strings.TrimSpace(email))

	// Only normalize actual Gmail addresses
	if !strings.HasSuffix(email, "@gmail.com") {
		return email
	}

	// Split at @ and remove dots from local part
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return email
	}

	localPart := strings.ReplaceAll(parts[0], ".", "")
	return localPart + "@" + parts[1]
}

// detectUserEmails detects all user email variants and returns the primary one plus all variants.
func (g *GmailProcessor) detectUserEmails(path string) (string, []string, error) {
	primaryEmail, err := DetectUserEmailFromMbox(path)
	if err != nil {
		return "", nil, err
	}

	if err := g.store.SetSourceUsername(context.Background(), db.SourceUsername{
		Source:   g.Name(),
		Username: primaryEmail,
	}); err != nil {
		g.logger.Warn("Failed to store user email", "error", err)
	}

	// Generate all possible Gmail variants
	variants := []string{primaryEmail}

	// If it's a Gmail address, add the normalized version
	if strings.HasSuffix(primaryEmail, "@gmail.com") {
		normalized := normalizeGmailAddress(primaryEmail)
		if normalized != primaryEmail {
			variants = append(variants, normalized)
		}

		// Also try the version with dots removed in display
		parts := strings.Split(primaryEmail, "@")
		if len(parts) == 2 {
			withoutDots := strings.ReplaceAll(parts[0], ".", "") + "@" + parts[1]
			if withoutDots != primaryEmail && withoutDots != normalized {
				variants = append(variants, withoutDots)
			}
		}
	}

	g.logger.Info("Detected user email variants", "primary", primaryEmail, "variants", variants)
	return primaryEmail, variants, nil
}

// Internal method with all the sophisticated logic.
func (g *GmailProcessor) processFileInternal(ctx context.Context, path string, outputDir string, sendersOnly bool) ([]memory.ConversationDocument, error) {
	primaryUserEmail, userEmailVariants, err := g.detectUserEmails(path)
	if err != nil {
		g.logger.Warn("Could not detect user email", "error", err)
	}

	// Setup output directory
	if outputDir == "" {
		outputDir = "output"
	}
	sendersFilePath := filepath.Join(outputDir, "senders.json")
	failedEmailFilepath := filepath.Join(outputDir, "F_0_gmail.mbox")
	skippedEmailsFilepath := filepath.Join(outputDir, "skipped_emails_analysis.json")

	// Check for existing senders file
	useExistingSendersFile := g.fileExists(sendersFilePath)
	if useExistingSendersFile && !sendersOnly {
		g.logger.Info("Found existing senders.json, using for filtering")
	} else if !useExistingSendersFile && !sendersOnly {
		g.logger.Info("No senders.json found, will analyze and create it")
	}

	// Count emails
	totalEmails, err := g.countEmails(path)
	if err != nil {
		return nil, fmt.Errorf("failed to count emails: %w", err)
	}
	if totalEmails == 0 {
		g.logger.Info("No emails found in file")
		return []memory.ConversationDocument{}, nil
	}

	g.logger.Info("Starting email processing", "total", totalEmails, "workers", runtime.NumCPU())

	// Phase 1: Parse all emails with sophisticated parallel processing
	emails, failures, skippedReasons, err := g.parseEmailsAdvanced(path, totalEmails, failedEmailFilepath, skippedEmailsFilepath)
	if err != nil {
		return nil, err
	}

	g.logger.Info("Parsing completed", "success", len(emails), "failed", len(failures), "skipped_reasons", len(skippedReasons))

	// Phase 2: Sender analysis
	var keepers map[string]bool
	var includedSenders, excludedSenders []SenderDetail

	if useExistingSendersFile && !sendersOnly {
		// Load existing senders
		keepers, err = g.loadExistingSenders(sendersFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to load existing senders: %w", err)
		}
		g.logger.Info("Loaded existing sender filtering", "included", len(keepers))
	} else {
		// Analyze senders using ALL user email variants
		includedSenders, excludedSenders, keepers = g.analyzeSenders(emails, userEmailVariants)
		g.logger.Info("Sender analysis completed", "included", len(includedSenders), "excluded", len(excludedSenders))

		// Save sender analysis
		if err := g.saveSenderAnalysis(sendersFilePath, includedSenders, excludedSenders, outputDir); err != nil {
			return nil, fmt.Errorf("failed to save sender analysis: %w", err)
		}

		// If only doing sender analysis, return early
		if sendersOnly {
			g.logger.Info("Sender analysis saved", "file", sendersFilePath)
			return nil, nil
		}
	}

	// Phase 3: Filter and build conversation documents
	filteredEmails := g.filterEmailsBySenders(emails, keepers)
	g.logger.Info("Email filtering completed", "kept", len(filteredEmails), "filtered_out", len(emails)-len(filteredEmails))

	threads := g.buildThreads(filteredEmails)
	documents := g.toConversationDocuments(threads, primaryUserEmail)

	g.logger.Info("Processing completed", "threads", len(threads), "documents", len(documents))
	return documents, nil
}

// Helper functions.
func (g *GmailProcessor) fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
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

func (g *GmailProcessor) parseEmail(parser *letters.EmailParser, content string) (*emailWithMeta, string, error) {
	email, err := parser.Parse(strings.NewReader(content))
	if err != nil {
		return nil, "", fmt.Errorf("letters parsing failed: %w", err)
	}

	if skipReason := g.shouldSkipEmail(&email); skipReason != "" {
		return nil, skipReason, nil // nil error means skip, not failure
	}

	threadID := g.extractThreadID(&email)
	return &emailWithMeta{
		email:     &email,
		threadID:  threadID,
		timestamp: email.Headers.Date,
	}, "", nil
}

func (g *GmailProcessor) shouldSkipEmail(email *letters.Email) string {
	if g.hasSkipLabels(email) {
		return "skip_labels"
	}
	if g.hasSkipSender(email) {
		return "skip_sender"
	}
	if email.Text == "" && email.HTML == "" {
		return "empty_content"
	}
	return ""
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
		"no-reply", "noreply", "no_reply", "donotreply", "do-not-reply", "support", "alert", "notification",
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

func (g *GmailProcessor) isUserEmail(email *letters.Email, userEmails []string) bool {
	for _, from := range email.Headers.From {
		if from != nil && from.Address != "" {
			fromNormalized := normalizeGmailAddress(from.Address)
			for _, userEmail := range userEmails {
				userNormalized := normalizeGmailAddress(userEmail)
				if fromNormalized == userNormalized {
					return true
				}
			}
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

	// Normalize the user email consistently with people extraction
	normalizedUserEmail := normalizeGmailAddress(userEmail)

	for threadID, emails := range threads {
		if len(emails) == 0 {
			continue
		}

		doc := memory.ConversationDocument{
			FieldID:      fmt.Sprintf("gmail-thread-%s", threadID),
			FieldSource:  "gmail",
			FieldTags:    []string{"email", "conversation"},
			User:         normalizedUserEmail,
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
				normalizedAddr := normalizeGmailAddress(addr.Address)
				people[normalizedAddr] = true
			}
		}
		for _, addr := range email.email.Headers.To {
			if addr != nil && addr.Address != "" {
				normalizedAddr := normalizeGmailAddress(addr.Address)
				people[normalizedAddr] = true
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
		return normalizeGmailAddress(email.Headers.From[0].Address)
	}
	if email.Headers.Sender != nil {
		return normalizeGmailAddress(email.Headers.Sender.Address)
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

func (g *GmailProcessor) parseEmailsAdvanced(path string, totalEmails int, failedFilePath string, skippedEmailsFilepath string) ([]*emailWithMeta, []emailResult, []SkipReason, error) {
	jobs := make(chan emailJob, runtime.NumCPU())
	results := make(chan emailResult, totalEmails)

	var wg sync.WaitGroup
	var processedCount, failedCount, skippedCount atomic.Int64

	// Start workers
	for i := 0; i < runtime.NumCPU(); i++ {
		wg.Add(1)
		go g.advancedEmailWorker(jobs, results, &wg)
	}

	// Read emails from file
	go g.distributeEmailJobs(path, jobs)

	// Collect results
	go func() {
		wg.Wait()
		close(results)
	}()

	var emails []*emailWithMeta
	var failures []emailResult
	var skippedReasons []SkipReason

	for result := range results {
		processedCount.Add(1)
		if result.err != nil {
			errStr := result.err.Error()
			if strings.HasPrefix(errStr, "SKIPPED:") {
				// This is a skipped email
				skippedCount.Add(1)
				reason := strings.TrimPrefix(errStr, "SKIPPED:")
				skippedReasons = append(skippedReasons, SkipReason{
					Reason:   reason,
					Count:    1,
					Examples: []string{g.getEmailPreview(result.originalData)},
				})
			} else {
				// This is a real failure
				failedCount.Add(1)
				failures = append(failures, result)
			}
		} else if result.email != nil {
			emails = append(emails, result.email)
		}

		// Progress reporting with visual bar
		if int(processedCount.Load())%100 == 0 || int(processedCount.Load()) == totalEmails {
			percent := float64(processedCount.Load()) * 100.0 / float64(totalEmails)
			barWidth := 40
			filled := int(percent / 100 * float64(barWidth))

			bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
			fmt.Fprintf(os.Stderr, "\r[%s] %.1f%% (%d/%d)", bar, percent, int(processedCount.Load()), totalEmails)

			if failedCount.Load() > 0 || skippedCount.Load() > 0 {
				fmt.Fprintf(os.Stderr, " [Failed: %d, Skipped: %d]", failedCount.Load(), skippedCount.Load())
			}

			// Add newline at completion
			if int(processedCount.Load()) == totalEmails {
				fmt.Fprintf(os.Stderr, "\n")
			}
		}
	}

	// Always save failed emails file (even if empty)
	g.saveFailedEmails(failedFilePath, failures)

	// Save skipped emails analysis (optional for debugging)
	if saveSkippedAnalysis {
		if err := g.saveSkippedEmailsAnalysis(skippedEmailsFilepath, skippedReasons); err != nil {
			return nil, nil, nil, fmt.Errorf("failed to save skipped emails analysis: %w", err)
		}
	}

	successCount := int64(len(emails))
	g.logger.Info("Email processing complete",
		"success", successCount,
		"failed", failedCount.Load(),
		"skipped", skippedCount.Load(),
		"total", totalEmails)

	return emails, failures, skippedReasons, nil
}

func (g *GmailProcessor) getEmailPreview(emailData string) string {
	lines := strings.Split(emailData, "\n")
	maxLines := 5
	if len(lines) < maxLines {
		maxLines = len(lines)
	}
	preview := strings.Join(lines[:maxLines], "\n")
	if len(preview) > 500 {
		preview = preview[:500] + "... [truncated]"
	}
	return preview
}

func (g *GmailProcessor) advancedEmailWorker(jobs <-chan emailJob, results chan<- emailResult, wg *sync.WaitGroup) {
	defer wg.Done()
	parser := letters.NewEmailParser(letters.WithFileFilter(letters.NoFiles))

	for job := range jobs {
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), processTimeout)

		resultChan := make(chan emailResult, 1)
		go func() {
			email, skipReason, err := g.parseEmail(parser, job.emailData)
			result := emailResult{
				emailIndex: job.emailIndex,
				email:      email,
				duration:   time.Since(start),
				err:        err,
			}
			// Set originalData for any error so failed emails can be saved
			if err != nil {
				result.originalData = job.emailData
				result.originalSize = len(job.emailData)
			} else if skipReason != "" {
				// Email was skipped - create a pseudo-error for tracking
				result.err = fmt.Errorf("SKIPPED:%s", skipReason)
				result.originalData = job.emailData
				result.originalSize = len(job.emailData)
			}
			resultChan <- result
		}()

		select {
		case result := <-resultChan:
			results <- result
		case <-ctx.Done():
			results <- emailResult{
				emailIndex:   job.emailIndex,
				originalData: job.emailData,
				originalSize: len(job.emailData),
				err:          fmt.Errorf("timeout after %v", processTimeout),
				duration:     time.Since(start),
			}
		}
		cancel()
	}
}

func (g *GmailProcessor) distributeEmailJobs(path string, jobs chan<- emailJob) {
	defer close(jobs)

	f, err := os.Open(path)
	if err != nil {
		g.logger.Error("Failed to open file", "error", err)
		return
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	var content strings.Builder
	var inEmail bool
	emailIndex := 0

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "From ") {
			if inEmail && content.Len() > 0 {
				emailIndex++
				jobs <- emailJob{emailIndex: emailIndex, emailData: content.String()}
				content.Reset()
			}
			inEmail = true
		} else if inEmail {
			content.WriteString(line + "\n")
		}
	}

	if inEmail && content.Len() > 0 {
		emailIndex++
		jobs <- emailJob{emailIndex: emailIndex, emailData: content.String()}
	}
}

func (g *GmailProcessor) saveFailedEmails(failedPath string, failures []emailResult) {
	if err := os.MkdirAll(filepath.Dir(failedPath), 0o755); err != nil {
		g.logger.Warn("Could not create output directory", "error", err)
		return
	}

	f, err := os.Create(failedPath)
	if err != nil {
		g.logger.Warn("Could not create failed emails file", "error", err)
		return
	}
	defer func() { _ = f.Close() }()

	for _, failure := range failures {
		if failure.originalData != "" {
			_, _ = f.WriteString(failure.originalData)
		}
	}

	if len(failures) > 0 {
		g.logger.Info("Saved failed emails", "count", len(failures), "file", failedPath)
	} else {
		g.logger.Info("Created empty failed emails file", "file", failedPath)
	}
}

func (g *GmailProcessor) loadExistingSenders(filePath string) (map[string]bool, error) {
	type senderCategorization struct {
		Included []SenderDetail `json:"included"`
		Excluded []SenderDetail `json:"excluded"`
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var categorization senderCategorization
	if err := json.Unmarshal(data, &categorization); err != nil {
		return nil, err
	}

	// Simple inclusion logic: Only senders in "included" array are processed
	// - If sender is in "included" → PROCESS
	// - If sender is in "excluded" OR deleted from "included" → SKIP
	keepers := make(map[string]bool)
	for _, sender := range categorization.Included {
		keepers[sender.Email] = true
	}

	g.logger.Info("Loaded sender preferences",
		"included", len(categorization.Included),
		"excluded", len(categorization.Excluded))

	return keepers, nil
}

func (g *GmailProcessor) analyzeSenders(emails []*emailWithMeta, userEmails []string) ([]SenderDetail, []SenderDetail, map[string]bool) {
	senderCounts := make(map[string]int)
	recipientsOfMyEmails := make(map[string]bool)

	// Count senders and track interactions
	for _, email := range emails {
		seenInThisEmail := make(map[string]bool)
		for _, from := range email.email.Headers.From {
			if from != nil && from.Address != "" {
				normalizedAddr := normalizeGmailAddress(from.Address)
				if !seenInThisEmail[normalizedAddr] {
					senderCounts[normalizedAddr]++
					seenInThisEmail[normalizedAddr] = true
				}
			}
		}

		if g.isUserEmail(email.email, userEmails) {
			for _, to := range email.email.Headers.To {
				if to != nil && to.Address != "" {
					recipientsOfMyEmails[normalizeGmailAddress(to.Address)] = true
				}
			}
			for _, cc := range email.email.Headers.Cc {
				if cc != nil && cc.Address != "" {
					recipientsOfMyEmails[normalizeGmailAddress(cc.Address)] = true
				}
			}
		}
	}

	// Categorize senders
	const minEmailCount = 5
	var included, excluded []SenderDetail
	keepers := make(map[string]bool)

	for email, count := range senderCounts {
		interaction := recipientsOfMyEmails[email]
		detail := SenderDetail{
			Email:       email,
			Count:       count,
			Interaction: interaction,
		}

		if count > minEmailCount || interaction {
			keepers[email] = true
			included = append(included, detail)
		} else {
			detail.Reason = fmt.Sprintf("Low count (<=%d) and no interaction", minEmailCount)
			excluded = append(excluded, detail)
		}
	}

	// Sort by count descending
	sort.Slice(included, func(i, j int) bool { return included[i].Count > included[j].Count })
	sort.Slice(excluded, func(i, j int) bool { return excluded[i].Count > excluded[j].Count })

	return included, excluded, keepers
}

func (g *GmailProcessor) saveSenderAnalysis(filePath string, included, excluded []SenderDetail, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}

	// Create JSON with included first for easier editing
	data := map[string]interface{}{
		"_instructions": "CURATION RULES: Only senders in 'included' will be processed. Move senders between 'included'/'excluded' or delete them entirely to control filtering.",
		"included":      included,
		"excluded":      excluded,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, jsonData, 0o644)
}

func (g *GmailProcessor) filterEmailsBySenders(emails []*emailWithMeta, keepers map[string]bool) []*emailWithMeta {
	var filtered []*emailWithMeta
	for _, email := range emails {
		keep := false
		for _, from := range email.email.Headers.From {
			if from != nil && from.Address != "" {
				if keepers[normalizeGmailAddress(from.Address)] {
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

func (g *GmailProcessor) saveSkippedEmailsAnalysis(filePath string, skippedReasons []SkipReason) error {
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		g.logger.Warn("Could not create output directory", "error", err)
		return err
	}

	f, err := os.Create(filePath)
	if err != nil {
		g.logger.Warn("Could not create skipped emails analysis file", "error", err)
		return err
	}
	defer func() { _ = f.Close() }()

	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(skippedReasons); err != nil {
		g.logger.Warn("Failed to encode skipped emails analysis", "error", err)
		return err
	}

	g.logger.Info("Saved skipped emails analysis", "count", len(skippedReasons), "file", filePath)
	return nil
}

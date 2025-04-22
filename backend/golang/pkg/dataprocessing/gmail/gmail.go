package gmail

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/http"
	"net/mail"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	loghtml "golang.org/x/net/html"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/helpers"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/charmbracelet/log"
	"github.com/sirupsen/logrus"
)

type Gmail struct{}

func New() *Gmail {
	return &Gmail{}
}

func (g *Gmail) Name() string {
	return "gmail"
}

// countEmails efficiently counts emails in an mbox file
func countEmails(filepath string) (int, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return 0, fmt.Errorf("error opening file for counting: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			logrus.Printf("Error closing file: %v", err)
		}
	}()

	count := 0
	scanner := bufio.NewScanner(file)

	// Increase the buffer size to handle potentially long lines
	const maxCapacity = 1024 * 1024  // 1 MB; adjust if needed
	buf := make([]byte, maxCapacity) // Start with a large initial buffer
	scanner.Buffer(buf, maxCapacity)

	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "From ") {
			count++
		}
	}
	if err := scanner.Err(); err != nil {
		// Check if the error is specifically ErrTooLong
		if err == bufio.ErrTooLong {
			return 0, fmt.Errorf("line too long, increase maxCapacity in countEmails: %w", err)
		}
		return 0, fmt.Errorf("error scanning file for counting: %w", err)
	}
	return count, nil
}

type job struct {
	emailIndex int
	emailData  string
}

type result struct {
	emailIndex   int
	record       types.Record
	originalData string // Holds original data ONLY for failed/timed out emails
	originalSize int    // Size of the original email data
	err          error
	duration     time.Duration
}

// Define timeout for processing a single email
const processTimeout = 1 * time.Second

func (g *Gmail) ProcessFile(filepath string, userName string) ([]types.Record, error) {
	// Count emails first
	totalEmails, err := countEmails(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to count emails: %w", err)
	}
	if totalEmails == 0 {
		return nil, fmt.Errorf("no emails found in the file")
	}
	fmt.Printf("Found %d emails. Starting processing using %d workers...\n", totalEmails, runtime.NumCPU())

	// Setup Worker Pool
	numWorkers := runtime.NumCPU()
	jobs := make(chan job, numWorkers)        // Buffered channel for jobs
	results := make(chan result, totalEmails) // Buffer results for collection
	var wg sync.WaitGroup
	var processedCount atomic.Int64 // Count emails processed (OK or Error/Timeout)
	var failedCount atomic.Int64    // Counter for failed/timed out emails

	// Open file for failed emails in the output directory
	failedEmailFilepath := "output/failed_emails.mbox" // Changed path
	failedFile, err := os.Create(failedEmailFilepath)
	if err != nil {
		// Try to create the output directory if it doesn't exist
		if os.IsNotExist(err) {
			errMkdir := os.Mkdir("output", 0o755) // Use default permissions
			if errMkdir == nil {
				// Try creating the file again
				failedFile, err = os.Create(failedEmailFilepath)
			}
		}
		// If file still couldn't be created, log warning
		if failedFile == nil {
			fmt.Fprintf(os.Stderr, "\nWARNING: Could not create %s: %v. Failed emails will not be separated.\n", failedEmailFilepath, err)
		}
	}
	if failedFile != nil {
		defer func() {
			if err := failedFile.Close(); err != nil {
				logrus.Printf("Error closing failed emails file: %v", err)
			}
		}()
	}

	// Start workers with timeout logic
	for w := 1; w <= numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := range jobs {
				// Note: Size check happens here before detailed logging/processing
				originalSize := len(j.emailData) // Get size early

				processResultChan := make(chan result, 1)
				ctx, cancel := context.WithTimeout(context.Background(), processTimeout)

				startTime := time.Now()
				go func() {
					record, err := g.processEmail(j.emailData, userName)
					duration := time.Since(startTime)
					originalDataOnError := ""
					if err != nil {
						originalDataOnError = j.emailData
					}
					processResultChan <- result{
						emailIndex:   j.emailIndex,
						record:       record,
						originalData: originalDataOnError,
						originalSize: originalSize, // Pass size along
						err:          err,
						duration:     duration,
					}
				}()

				select {
				case res := <-processResultChan:
					results <- res
				case <-ctx.Done():
					duration := time.Since(startTime)
					timeoutErr := fmt.Errorf("processing timed out after %v", processTimeout)
					results <- result{
						emailIndex:   j.emailIndex,
						record:       types.Record{},
						originalData: j.emailData,
						originalSize: originalSize, // Pass size along on timeout too
						err:          timeoutErr,
						duration:     duration,
					}
				}
				cancel()
			}
		}(w)
	}

	// Distribute Work
	go func() {
		file, err := os.Open(filepath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nERROR opening file for processing: %v\n", err)
			close(jobs)
			return
		}
		defer func() {
			if err := file.Close(); err != nil {
				logrus.Printf("Error closing file: %v", err)
			}
		}()

		reader := bufio.NewReader(file)
		var emailBuffer bytes.Buffer
		var inEmail bool
		emailIndex := 0

		for {
			line, err := reader.ReadString('\n')
			if err == io.EOF {
				if inEmail {
					emailIndex++
					// Send all emails to workers
					jobs <- job{emailIndex: emailIndex, emailData: emailBuffer.String()}
				}
				break
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "\nERROR reading file: %v\n", err)
				break
			}

			if strings.HasPrefix(line, "From ") {
				if inEmail {
					emailIndex++
					// Send all emails to workers
					jobs <- job{emailIndex: emailIndex, emailData: emailBuffer.String()}
					emailBuffer.Reset()
				}
				inEmail = true
			}

			if inEmail {
				emailBuffer.WriteString(line)
			}
		}
		close(jobs)
	}()

	// Collect Results, Separate Failed Emails, & Report Progress
	recordsMap := make(map[int]types.Record)

	// Goroutine to wait for workers and close results channel
	go func() {
		wg.Wait()
		close(results)
	}()

	// Reporting setup (1% interval)
	var lastReportCount int64 = 0
	reportInterval := int64(float64(totalEmails) * 0.01)
	if reportInterval < 1 {
		reportInterval = 1
	} else if reportInterval < 10 {
		reportInterval = 10
	}

	const progressBarWidth = 50 // Width of the visual progress bar

	for res := range results {
		currentProcessed := processedCount.Add(1) // Count all attempts

		if res.err != nil {
			// Handle error or timeout -> move to failed file
			failedCount.Add(1)

			// Differentiate log prefix and include size
			logPrefix := "ERROR"
			if strings.Contains(res.err.Error(), "processing timed out") {
				logPrefix = "TIMEOUT"
			}
			fmt.Fprintf(os.Stderr, "%s - Email %d moved to %s (Size: %d bytes, %v, %dms)\n",
				logPrefix, res.emailIndex, failedEmailFilepath,
				res.originalSize, res.err, res.duration.Milliseconds())

			if failedFile != nil {
				_, writeErr := failedFile.WriteString(res.originalData)
				if writeErr != nil {
					// Log warning about write failure, but continue
					fmt.Fprintf(os.Stderr, "\nWARNING: Failed to write failed email %d to %s: %v\n", res.emailIndex, failedEmailFilepath, writeErr)
				}
			}
		} else {
			// Handle successful email
			recordsMap[res.emailIndex] = res.record
		}

		// Percentage milestone report
		if currentProcessed-lastReportCount >= reportInterval || int(currentProcessed) == totalEmails {
			percent := float64(currentProcessed) * 100.0 / float64(totalEmails)
			filledWidth := int(float64(progressBarWidth) * percent / 100.0)
			bar := strings.Repeat("#", filledWidth) + strings.Repeat("-", progressBarWidth-filledWidth)

			fmt.Fprintf(os.Stderr, "\r[%s] %.2f%% [Failed: %d]        ",
				bar, percent, failedCount.Load())
			lastReportCount = currentProcessed
		}
	}

	// --- 5. Final Summary & Cleanup ---
	// Ensure the progress line is cleared and we start fresh for stdout
	fmt.Fprint(os.Stderr, "\r"+strings.Repeat(" ", progressBarWidth+30)+"\r") // Clear the progress line on stderr
	if finalFailedCount := failedCount.Load(); finalFailedCount > 0 {
		fmt.Printf("%d emails failed (error or >%v timeout) and were moved to %s.\n",
			finalFailedCount, processTimeout, failedEmailFilepath)
	}
	fmt.Println("Processing finished.")

	// Convert map to slice in original order (best effort)
	finalRecords := make([]types.Record, 0, len(recordsMap))
	for i := 1; i <= totalEmails; i++ {
		if record, ok := recordsMap[i]; ok {
			finalRecords = append(finalRecords, record)
		}
	}

	return finalRecords, nil
}

func (g *Gmail) processEmail(content string, userName string) (types.Record, error) {
	msg, err := mail.ReadMessage(strings.NewReader(content))
	if err != nil {
		return types.Record{}, err
	}

	header := msg.Header

	// Parse date
	date, err := mail.ParseDate(header.Get("Date"))
	if err != nil {
		// Fallback to current time if date parsing fails
		date = time.Now()
	}

	// Extract email data
	data := map[string]interface{}{
		"from":      header.Get("From"),
		"to":        header.Get("To"),
		"subject":   header.Get("Subject"),
		"myMessage": strings.EqualFold(header.Get("From"), userName),
	}

	// Get Content-Type
	mediaType, params, err := mime.ParseMediaType(header.Get("Content-Type"))
	if err == nil && strings.HasPrefix(mediaType, "multipart/") {
		mr := multipart.NewReader(msg.Body, params["boundary"])
		var contentBuilder strings.Builder
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				continue
			}
			defer func() {
				if err := part.Close(); err != nil {
					logrus.Printf("Error closing part: %v", err)
				}
			}()

			partType, _, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
			if err != nil {
				continue
			}
			encoding := strings.ToLower(part.Header.Get("Content-Transfer-Encoding"))

			var partReader io.Reader = part
			if encoding == "quoted-printable" {
				partReader = quotedprintable.NewReader(partReader)
			}

			switch {
			case strings.HasPrefix(partType, "text/plain"):
				bodyBytes, err := io.ReadAll(partReader)
				if err == nil {
					contentBuilder.Write(bodyBytes)
					contentBuilder.WriteByte('\n')
				}
			case strings.HasPrefix(partType, "text/html"):
				bodyBytes, err := io.ReadAll(partReader)
				if err == nil {
					textContent := extractTextFromHTML(string(bodyBytes))
					if textContent != "" {
						contentBuilder.WriteString(textContent)
						contentBuilder.WriteByte('\n')
					}
				}
			}
		}
		data["content"] = strings.TrimSpace(contentBuilder.String())
	} else {
		encoding := strings.ToLower(header.Get("Content-Transfer-Encoding"))
		bodyReader := msg.Body
		if encoding == "quoted-printable" {
			bodyReader = quotedprintable.NewReader(bodyReader)
		}

		bodyBytes, err := io.ReadAll(bodyReader)
		if err == nil {
			content := string(bodyBytes)
			if strings.Contains(strings.ToLower(mediaType), "html") {
				content = extractTextFromHTML(content)
			}
			data["content"] = strings.TrimSpace(content)
		}
	}

	return types.Record{
		Data:      data,
		Timestamp: date,
		Source:    g.Name(),
	}, nil
}

// extractTextFromHTML extracts readable text content from HTML
func extractTextFromHTML(htmlContent string) string {
	// NOTE: Quoted-printable decoding is now handled *before* this function is called,
	// by checking Content-Transfer-Encoding and using quotedprintable.Reader.
	// We remove the manual QP decoding steps here.

	// Decode common HTML entities and remaining UTF-8 sequences (like =E2=80=99)
	// Keep this part for entities and potentially mis-encoded sequences not handled by QP reader
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

	// Try to decode any remaining specific UTF-8 sequences (that might not be QP encoded)
	if decodedContent, err := decodeUTF8Sequences(htmlContent); err == nil {
		htmlContent = decodedContent
	}

	// Unescape basic HTML entities not covered by replacer
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
			// Skip style, script, and other non-content tags
			switch strings.ToLower(n.Data) {
			case "style", "script", "noscript", "iframe", "head", "meta", "link":
				return
			}
		case loghtml.TextNode:
			text := strings.TrimSpace(n.Data)
			if text != "" {
				// Add spacing between text nodes if needed
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

	// Clean up the final text
	result := textBuilder.String()
	// Remove extra whitespace
	result = strings.Join(strings.Fields(result), " ")
	// Remove duplicate newlines
	for strings.Contains(result, "\n\n") {
		result = strings.ReplaceAll(result, "\n\n", "\n")
	}
	return strings.TrimSpace(result)
}

// decodeUTF8Sequences attempts to decode remaining UTF-8 sequences in the text
func decodeUTF8Sequences(text string) (string, error) {
	// Find sequences like =E2=80=99 and try to decode them
	re := regexp.MustCompile(`=([0-9A-F]{2})(=([0-9A-F]{2}))?(=([0-9A-F]{2}))?`)
	return re.ReplaceAllStringFunc(text, func(match string) string {
		parts := re.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}

		// Convert hex strings to bytes
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

		// Try to decode as UTF-8
		if str := string(bytes); utf8.ValidString(str) {
			return str
		}
		return match
	}), nil
}

func (g *Gmail) ProcessDirectory(dirPath string, userName string) ([]types.Record, error) {
	var allRecords []types.Record
	var mu sync.Mutex // To protect allRecords slice

	// Walk through the directory recursively
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if the file is named emails.mbox
		if strings.Contains(info.Name(), ".mbox") {
			// Process the file
			records, err := g.ProcessFile(path, userName)
			if err != nil {
				logrus.Printf("Error processing file %s: %v", path, err)
				return nil // Continue processing other files
			}

			// Safely append records to the shared slice
			mu.Lock()
			allRecords = append(allRecords, records...)
			mu.Unlock()
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error walking directory: %w", err)
	}

	return allRecords, nil
}

func ToDocuments(path string) ([]memory.TextDocument, error) {
	records, err := helpers.ReadJSONL[types.Record](path)
	if err != nil {
		return nil, err
	}

	documents := make([]memory.TextDocument, 0, len(records))
	for _, record := range records {
		// Helper function to safely get string value
		getString := func(key string) string {
			if val, ok := record.Data[key]; ok {
				if strVal, ok := val.(string); ok {
					return strVal
				}
			}
			return ""
		}

		content := getString("content")
		from := getString("from")
		to := getString("to")
		subject := getString("subject")

		documents = append(documents, memory.TextDocument{
			Content:   content,
			Timestamp: &record.Timestamp,
			Tags:      []string{"google", "email"},
			Metadata: map[string]string{
				"from":    from,
				"to":      to,
				"subject": subject,
			},
		})
	}
	return documents, nil
}

func (g *Gmail) Sync(ctx context.Context) ([]types.Record, error) {
	store, err := db.NewStore(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create store: %w", err)
	}

	// Get OAuth tokens for Google
	tokens, err := store.GetOAuthTokens(ctx, "google")
	if err != nil {
		return nil, fmt.Errorf("failed to get OAuth tokens: %w", err)
	}
	if tokens == nil {
		return nil, fmt.Errorf("no OAuth tokens found for Google")
	}

	// Create HTTP client with OAuth token
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Get the latest emails from Gmail API
	req, err := http.NewRequestWithContext(
		ctx,
		"GET",
		"https://gmail.googleapis.com/gmail/v1/users/me/messages",
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokens.AccessToken))
	q := req.URL.Query()
	q.Set("maxResults", "50") // Get last 50 emails
	q.Set("q", "in:inbox")    // Only get inbox emails
	req.URL.RawQuery = q.Encode()

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch emails: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch emails. Status: %d, Response: %s", resp.StatusCode, string(body))
	}

	var messageList struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&messageList); err != nil {
		return nil, fmt.Errorf("failed to decode message list: %w", err)
	}

	var records []types.Record
	for _, msg := range messageList.Messages {
		// Get full message details
		msgReq, err := http.NewRequestWithContext(
			ctx,
			"GET",
			fmt.Sprintf("https://gmail.googleapis.com/gmail/v1/users/me/messages/%s", msg.ID),
			nil,
		)
		if err != nil {
			log.Printf("Failed to create message request: %v", err)
			continue
		}

		msgReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokens.AccessToken))
		msgResp, err := client.Do(msgReq)
		if err != nil {
			log.Printf("Failed to fetch message: %v", err)
			continue
		}

		if msgResp.StatusCode != http.StatusOK {
			msgResp.Body.Close()
			log.Printf("Failed to fetch message. Status: %d", msgResp.StatusCode)
			continue
		}

		var message struct {
			Payload struct {
				Headers []struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				} `json:"headers"`
				Body struct {
					Data string `json:"data"`
				} `json:"body"`
				Parts []struct {
					Body struct {
						Data string `json:"data"`
					} `json:"body"`
					MimeType string `json:"mimeType"`
				} `json:"parts"`
			} `json:"payload"`
		}

		if err := json.NewDecoder(msgResp.Body).Decode(&message); err != nil {
			msgResp.Body.Close()
			log.Printf("Failed to decode message: %v", err)
			continue
		}
		msgResp.Body.Close()

		// Extract headers
		headers := make(map[string]string)
		for _, h := range message.Payload.Headers {
			headers[h.Name] = h.Value
		}

		// Extract content
		var content string
		if len(message.Payload.Parts) > 0 {
			for _, part := range message.Payload.Parts {
				if part.MimeType == "text/plain" {
					decoded, err := decodeBase64URL(part.Body.Data)
					if err == nil {
						content = decoded
						break
					}
				}
			}
		} else if message.Payload.Body.Data != "" {
			decoded, err := decodeBase64URL(message.Payload.Body.Data)
			if err == nil {
				content = decoded
			}
		}

		// Parse date
		date, err := mail.ParseDate(headers["Date"])
		if err != nil {
			date = time.Now()
		}

		records = append(records, types.Record{
			Data: map[string]interface{}{
				"from":      headers["From"],
				"to":        headers["To"],
				"subject":   headers["Subject"],
				"content":   content,
				"myMessage": false,
			},
			Timestamp: date,
			Source:    g.Name(),
		})
	}

	return records, nil
}

// decodeBase64URL decodes base64url encoded strings
func decodeBase64URL(s string) (string, error) {
	// Add padding if needed
	if len(s)%4 != 0 {
		s += strings.Repeat("=", 4-len(s)%4)
	}
	// Replace URL-safe characters
	s = strings.ReplaceAll(s, "-", "+")
	s = strings.ReplaceAll(s, "_", "/")

	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

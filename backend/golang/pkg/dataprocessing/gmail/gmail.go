// owner: slimane@eternis.ai

package gmail

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
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

	"github.com/charmbracelet/log"
	"github.com/jaytaylor/html2text"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/processor"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
	"github.com/EternisAI/enchanted-twin/pkg/db"
)

type GmailProcessor struct {
	store  *db.Store
	logger *log.Logger
}

func NewGmailProcessor(store *db.Store, logger *log.Logger) (processor.Processor, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger is nil")
	}

	if store == nil {
		return nil, fmt.Errorf("store is nil")
	}

	return &GmailProcessor{store: store, logger: logger}, nil
}

func (g *GmailProcessor) Name() string { return "gmail" }

func countEmails(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close() //nolint:errcheck

	const maxCap = 1024 * 1024
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, maxCap), maxCap)

	n := 0
	for sc.Scan() {
		if strings.HasPrefix(sc.Text(), "From ") {
			n++
		}
	}
	if err := sc.Err(); err != nil {
		return 0, err
	}
	return n, nil
}

type (
	job struct {
		idx int
		raw string
	}
	result struct {
		idx   int
		rec   types.Record
		raw   string
		size  int
		err   error
		elaps time.Duration
	}
)

const processTimeout = time.Second

func (g *GmailProcessor) ProcessFile(ctx context.Context, path string) ([]types.Record, error) {
	userEmail, err := DetectUserEmailFromMbox(path)
	if err != nil {
		g.logger.Warn("Could not detect user email", "error", err)
		userEmail = ""
	} else {
		g.logger.Info("Detected user email", "userEmail", userEmail)

		if err := g.extractAndStoreUserEmail(ctx, userEmail); err != nil {
			g.logger.Warn("Failed to store user email in database", "error", err)
		}
	}

	total, err := countEmails(path)
	if err != nil {
		return nil, err
	}
	if total == 0 {
		return nil, fmt.Errorf("no emails in %s", path)
	}
	g.logger.Info("Found emails", "total", total, "workers", runtime.NumCPU())

	jobs := make(chan job, runtime.NumCPU())
	results := make(chan result, total)

	var wg sync.WaitGroup
	var seen, fails atomic.Int64

	failPath := "output/failed_emails.mbox"
	failF, _ := os.Create(failPath)
	if failF != nil {
		defer failF.Close() //nolint:errcheck
	}

	for w := 0; w < cap(jobs); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				ctx, cancel := context.WithTimeout(context.Background(), processTimeout)
				start := time.Now()

				done := make(chan struct {
					r   types.Record
					err error
				})
				go func(raw string) {
					rec, e := g.processEmail(raw, userEmail)
					done <- struct {
						r   types.Record
						err error
					}{rec, e}
				}(j.raw)

				var out result
				select {
				case v := <-done:
					out = result{
						idx:   j.idx,
						rec:   v.r,
						err:   v.err,
						size:  len(j.raw),
						elaps: time.Since(start),
					}
					if v.err != nil {
						out.raw = j.raw
					}
				case <-ctx.Done():
					out = result{
						idx:   j.idx,
						err:   fmt.Errorf("timeout after %s", processTimeout),
						raw:   j.raw,
						size:  len(j.raw),
						elaps: time.Since(start),
					}
				}
				cancel()
				results <- out
			}
		}()
	}

	go func() {
		f, err := os.Open(path)
		if err != nil {
			log.Errorf("open %s: %v", path, err)
			close(jobs)
			return
		}

		defer f.Close() //nolint:errcheck

		var buf bytes.Buffer
		r := bufio.NewReader(f)
		idx := 0
		in := false
		for {
			line, err := r.ReadString('\n')
			if err == io.EOF {
				if in {
					jobs <- job{idx: idx, raw: buf.String()}
				}
				break
			}
			if err != nil {
				log.Errorf("read: %v", err)
				break
			}
			if strings.HasPrefix(line, "From ") {
				if in {
					jobs <- job{idx: idx, raw: buf.String()}
					buf.Reset()
				}
				in = true
				idx++
			}
			if in {
				buf.WriteString(line)
			}
		}
		close(jobs)
	}()

	go func() { wg.Wait(); close(results) }()

	records := make(map[int]types.Record)

	for res := range results {
		seen.Add(1)
		if res.err != nil {
			fails.Add(1)
			if failF != nil {
				_, _ = failF.WriteString(res.raw)
			}
		} else {
			records[res.idx] = res.rec
		}
	}

	out := make([]types.Record, 0, len(records))
	for i := 1; i <= total; i++ {
		if r, ok := records[i]; ok {
			out = append(out, r)
		}
	}
	if fc := fails.Load(); fc > 0 {
		g.logger.Warn("Messages failed", "count", fc, "path", failPath)
	}
	return out, nil
}

func (g *GmailProcessor) processEmail(raw, userEmail string) (types.Record, error) {
	msg, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		return types.Record{}, err
	}

	h := msg.Header
	date, _ := mail.ParseDate(h.Get("Date"))

	fromAddr := extractEmailAddress(h.Get("From"))
	toAddr := extractEmailAddress(h.Get("To"))
	deliveredToAddr := extractEmailAddress(h.Get("Delivered-To"))

	var userRole string
	var isUserEmail bool
	if userEmail != "" {
		userEmailLower := strings.ToLower(userEmail)
		if strings.ToLower(fromAddr) == userEmailLower {
			userRole = "sender"
			isUserEmail = true
		} else if strings.ToLower(toAddr) == userEmailLower || strings.ToLower(deliveredToAddr) == userEmailLower {
			userRole = "recipient"
			isUserEmail = true
		} else {
			userRole = "unknown"
			isUserEmail = false
		}
	}

	data := map[string]interface{}{
		"from":         h.Get("From"),
		"to":           h.Get("To"),
		"subject":      h.Get("Subject"),
		"delivered_to": h.Get("Delivered-To"),
	}

	// Add user-specific metadata if user email is known
	if userEmail != "" {
		data["user_email"] = userEmail
		data["user_role"] = userRole
		data["is_user_email"] = isUserEmail
	}

	// Add extracted email addresses for easier filtering
	if fromAddr != "" {
		data["from_address"] = fromAddr
	}
	if toAddr != "" {
		data["to_address"] = toAddr
	}
	if deliveredToAddr != "" {
		data["delivered_to_address"] = deliveredToAddr
	}

	mt, params, _ := mime.ParseMediaType(h.Get("Content-Type"))
	var final string

	if strings.HasPrefix(mt, "multipart/") {
		mr := multipart.NewReader(msg.Body, params["boundary"])
		var plain, html string
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				continue
			}
			pt, _, _ := mime.ParseMediaType(p.Header.Get("Content-Type"))
			enc := strings.ToLower(p.Header.Get("Content-Transfer-Encoding"))
			var r io.Reader = p
			if enc == "quoted-printable" {
				r = quotedprintable.NewReader(r)
			}
			b, _ := io.ReadAll(r)

			switch {
			case strings.HasPrefix(pt, "text/plain") && plain == "":
				plain = string(b)
			case strings.HasPrefix(pt, "text/html") && html == "":
				if t, err := html2text.FromString(string(b), html2text.Options{OmitLinks: true, TextOnly: true}); err == nil {
					html = t
				}
			}
			p.Close() //nolint:errcheck
		}
		if plain != "" {
			final = plain
		} else {
			final = html
		}
	} else {
		enc := strings.ToLower(h.Get("Content-Transfer-Encoding"))
		r := msg.Body
		if enc == "quoted-printable" {
			r = quotedprintable.NewReader(r)
		}
		b, err := io.ReadAll(r)
		if err != nil {
			return types.Record{}, err
		}

		if strings.Contains(strings.ToLower(mt), "html") {
			html, _ := html2text.FromString(string(b), html2text.Options{OmitLinks: true, TextOnly: true})
			final = html
		} else {
			final = string(b)
		}
	}

	data["content"] = strings.TrimSpace(cleanEmailText(final))

	return types.Record{
		Data:      data,
		Timestamp: date,
		Source:    g.Name(),
	}, nil
}

/* ────────────────────────────────────────────  Gmail API sync  ───────────────────────────────────────── */

func (g *GmailProcessor) Sync(ctx context.Context, token string) ([]types.Record, bool, error) {
	c := &http.Client{Timeout: 30 * time.Second}

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://gmail.googleapis.com/gmail/v1/users/me/messages", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	q := req.URL.Query()
	q.Set("maxResults", "50")
	q.Set("q", "in:inbox")
	req.URL.RawQuery = q.Encode()

	resp, err := c.Do(req)
	if err != nil {
		return nil, false, err
	}

	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, false, fmt.Errorf("gmail list: %d %s", resp.StatusCode, b)
	}

	var list struct {
		Messages []struct{ ID string } `json:"messages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, false, err
	}

	var out []types.Record
	for _, m := range list.Messages {
		rec, err := FetchMessage(ctx, c, token, m.ID)
		if err != nil {
			log.Errorf("message %s: %v", m.ID, err)
			continue
		}
		out = append(out, rec)
	}
	return out, true, nil
}

func SyncWithDateRange(ctx context.Context, token, startDate, endDate string, maxResults int, pageToken string) ([]types.Record, bool, string, error) {
	if maxResults <= 0 {
		maxResults = 100
	}

	c := &http.Client{Timeout: 30 * time.Second}

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://gmail.googleapis.com/gmail/v1/users/me/messages", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	q := req.URL.Query()
	q.Set("maxResults", fmt.Sprintf("%d", maxResults))

	queryParams := []string{"in:inbox"}
	if startDate != "" {
		queryParams = append(queryParams, fmt.Sprintf("after:%s", startDate))
	}
	if endDate != "" {
		queryParams = append(queryParams, fmt.Sprintf("before:%s", endDate))
	}
	q.Set("q", strings.Join(queryParams, " "))

	if pageToken != "" {
		q.Set("pageToken", pageToken)
	}

	req.URL.RawQuery = q.Encode()

	resp, err := c.Do(req)
	if err != nil {
		return nil, false, "", err
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Errorf("failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, false, "", fmt.Errorf("gmail list: %d %s", resp.StatusCode, b)
	}

	var list struct {
		Messages      []struct{ ID string } `json:"messages"`
		NextPageToken string                `json:"nextPageToken"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, false, "", err
	}

	var out []types.Record
	for _, m := range list.Messages {
		rec, err := FetchMessage(ctx, c, token, m.ID)
		if err != nil {
			log.Errorf("message %s: %v", m.ID, err)
			continue
		}
		out = append(out, rec)
	}

	hasMore := list.NextPageToken != ""
	return out, hasMore, list.NextPageToken, nil
}

func FetchMessage(
	ctx context.Context,
	c *http.Client,
	token, id string,
) (types.Record, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("https://gmail.googleapis.com/gmail/v1/users/me/messages/%s", id), nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.Do(req)
	if err != nil {
		return types.Record{}, err
	}

	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return types.Record{}, fmt.Errorf("fetch %s: %d", id, resp.StatusCode)
	}

	var msg struct {
		Payload struct {
			MimeType string `json:"mimeType"`
			Headers  []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"headers"`
			Body  struct{ Data string } `json:"body"`
			Parts []struct {
				MimeType string                `json:"mimeType"`
				Body     struct{ Data string } `json:"body"`
			} `json:"parts"`
		} `json:"payload"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&msg); err != nil {
		return types.Record{}, err
	}

	h := map[string]string{}
	for _, v := range msg.Payload.Headers {
		h[v.Name] = v.Value
	}
	date, _ := mail.ParseDate(h["Date"])

	var plain, html string

	for _, p := range msg.Payload.Parts {
		switch {
		case p.MimeType == "text/plain" && plain == "":
			plain, _ = decodeBase64URL(p.Body.Data)
		case strings.HasPrefix(p.MimeType, "text/html") && html == "":
			raw, _ := decodeBase64URL(p.Body.Data)
			if t, err := html2text.FromString(raw, html2text.Options{OmitLinks: true, TextOnly: true}); err == nil {
				html = t
			}
		}
	}

	if plain == "" && html == "" && msg.Payload.Body.Data != "" {
		switch {
		case strings.HasPrefix(msg.Payload.MimeType, "text/plain"):
			plain, _ = decodeBase64URL(msg.Payload.Body.Data)
		case strings.HasPrefix(msg.Payload.MimeType, "text/html"):
			raw, _ := decodeBase64URL(msg.Payload.Body.Data)
			html, _ = html2text.FromString(raw, html2text.Options{OmitLinks: true, TextOnly: true})
		}
	}

	content := plain
	if content == "" {
		content = html
	}
	content = strings.TrimSpace(cleanEmailText(content))

	return types.Record{
		Data: map[string]interface{}{
			"from":       h["From"],
			"to":         h["To"],
			"subject":    h["Subject"],
			"content":    content,
			"message_id": id,
		},
		Timestamp: date,
		Source:    "gmail",
	}, nil
}

func decodeBase64URL(s string) (string, error) {
	if m := len(s) % 4; m != 0 {
		s += strings.Repeat("=", 4-m)
	}
	s = strings.ReplaceAll(s, "-", "+")
	s = strings.ReplaceAll(s, "_", "/")
	b, err := base64.StdEncoding.DecodeString(s)
	return string(b), err
}

func (g *GmailProcessor) ProcessDirectory(ctx context.Context, dir string) ([]types.Record, error) {
	var all []types.Record
	var mu sync.Mutex
	err := filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() || !strings.Contains(fi.Name(), ".mbox") {
			return err
		}
		recs, err := g.ProcessFile(ctx, p)
		if err != nil {
			g.logger.Error("process", "path", p, "error", err)
			return nil
		}
		mu.Lock()
		all = append(all, recs...)
		mu.Unlock()
		return nil
	})
	return all, err
}

func (g *GmailProcessor) ToDocuments(ctx context.Context, recs []types.Record) ([]memory.Document, error) {
	out := []memory.ConversationDocument{}
	for _, r := range recs {
		get := func(k string) string {
			if v, ok := r.Data[k]; ok {
				if s, ok := v.(string); ok {
					return s
				}
			}
			return ""
		}
		if get("content") == "" {
			continue
		}
		user := ""

		sourceUsername, err := g.store.GetSourceUsername(ctx, "gmail")
		if err != nil {
			g.logger.Error("get source username", "error", err)
		}
		if sourceUsername != nil {
			user = sourceUsername.Username
		}

		people := []string{user, get("from"), get("to")}

		idComponents := fmt.Sprintf("%d-%s-%s-%s",
			r.Timestamp.Unix(),
			get("from"),
			get("to"),
			get("subject"))
		hasher := sha256.New()
		hasher.Write([]byte(idComponents))
		emailHash := fmt.Sprintf("%x", hasher.Sum(nil))[:16]
		documentID := fmt.Sprintf("gmail-email-%s", emailHash)

		out = append(out, memory.ConversationDocument{
			FieldID:     documentID,
			User:        user,
			People:      people,
			FieldSource: "gmail",
			FieldTags:   []string{"email"},
			FieldMetadata: map[string]string{
				"from":    get("from"),
				"to":      get("to"),
				"subject": get("subject"),
			},
			Conversation: []memory.ConversationMessage{
				{
					Content: get("content"),
					Speaker: extractEmailAddress(get("from")),
					Time:    r.Timestamp,
				},
			},
		})
	}
	var documents []memory.Document
	for _, document := range out {
		documents = append(documents, &document)
	}

	return documents, nil
}

func cleanEmailText(content string) string {
	content = regexp.MustCompile(`=0[AD]`).ReplaceAllString(content, " ")
	content = regexp.MustCompile(`=\n`).ReplaceAllString(content, "")
	content = regexp.MustCompile(`=3D`).ReplaceAllString(content, "=")
	content = regexp.MustCompile(`=E2=9A=BD=EF=B8=8F`).ReplaceAllString(content, "⚽️")

	content = regexp.MustCompile(`https?://[^\s<>"]+`).ReplaceAllString(content, "")

	lines := strings.Split(content, "\n")
	var cleanedLines []string
	footerStarted := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		lowerLine := strings.ToLower(line)
		if !footerStarted && (strings.Contains(lowerLine, "unsubscribe") ||
			strings.Contains(lowerLine, "privacy policy") ||
			strings.Contains(lowerLine, "email notification") ||
			strings.Contains(lowerLine, "manage your") ||
			strings.Contains(lowerLine, "copyright") ||
			strings.Contains(lowerLine, "©") ||
			strings.Contains(lowerLine, "this email was sent") ||
			regexp.MustCompile(`--+`).MatchString(line)) {
			footerStarted = true
			continue
		}

		if footerStarted {
			continue
		}

		line = regexp.MustCompile(`<[^>]*>`).ReplaceAllString(line, "")
		line = regexp.MustCompile(`&[a-zA-Z0-9#]+;`).ReplaceAllString(line, "")
		line = regexp.MustCompile(`\s+`).ReplaceAllString(line, " ")
		line = strings.TrimSpace(line)

		if regexp.MustCompile(`^[=\s\d\-\+\(\)\[\]<>'"]+$`).MatchString(line) {
			continue
		}

		if line != "" {
			cleanedLines = append(cleanedLines, line)
		}
	}

	result := strings.Join(cleanedLines, "\n")
	return strings.TrimSpace(result)
}

// extractAndStoreUserEmail stores the detected user email in the database.
func (g *GmailProcessor) extractAndStoreUserEmail(ctx context.Context, userEmail string) error {
	if userEmail == "" {
		return fmt.Errorf("user email is empty")
	}

	sourceUsername := db.SourceUsername{
		Source:   g.Name(),
		Username: userEmail,
	}

	g.logger.Info("Saving email to database", "sourceUsername", sourceUsername)

	if err := g.store.SetSourceUsername(ctx, sourceUsername); err != nil {
		g.logger.Warn("Failed to save email to database", "error", err)
		return err
	}

	return nil
}

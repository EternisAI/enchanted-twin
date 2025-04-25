package gmail

import (
	"bufio"
	"bytes"
	"context"
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
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
	"github.com/charmbracelet/log"
	"github.com/jaytaylor/html2text"
	"github.com/sirupsen/logrus"
)

type Gmail struct{}

func New() *Gmail             { return &Gmail{} }
func (g *Gmail) Name() string { return "gmail" }

/* ────────────────────────────────────────────  MBOX helpers  ─────────────────────────────────────────── */

func countEmails(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

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

func (g *Gmail) ProcessFile(path, user string) ([]types.Record, error) {
	total, err := countEmails(path)
	if err != nil {
		return nil, err
	}
	if total == 0 {
		return nil, fmt.Errorf("no emails in %s", path)
	}
	fmt.Printf("Found %d emails, processing with %d workers …\n", total, runtime.NumCPU())

	jobs := make(chan job, runtime.NumCPU())
	results := make(chan result, total)

	var wg sync.WaitGroup
	var seen, fails atomic.Int64

	// failed-email sink
	failPath := "output/failed_emails.mbox"
	failF, _ := os.Create(failPath)
	if failF != nil {
		defer failF.Close()
	}

	// workers
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
					rec, e := g.processEmail(raw, user)
					done <- struct {
						r   types.Record
						err error
					}{rec, e}
				}(j.raw)

				var out result
				select {
				case v := <-done:
					out = result{idx: j.idx, rec: v.r, err: v.err, size: len(j.raw), elaps: time.Since(start)}
					if v.err != nil {
						out.raw = j.raw
					}
				case <-ctx.Done():
					out = result{idx: j.idx, err: fmt.Errorf("timeout after %s", processTimeout), raw: j.raw, size: len(j.raw), elaps: time.Since(start)}
				}
				cancel()
				results <- out
			}
		}()
	}

	// feed jobs
	go func() {
		f, err := os.Open(path)
		if err != nil {
			log.Errorf("open %s: %v", path, err)
			close(jobs)
			return
		}
		defer f.Close()

		var buf bytes.Buffer
		r := bufio.NewReader(f)
		idx := 0
		in := false
		for {
			line, err := r.ReadString('\n')
			if err == io.EOF {
				if in {
					jobs <- job{idx: idx + 1, raw: buf.String()}
				}
				break
			}
			if err != nil {
				log.Errorf("read: %v", err)
				break
			}
			if strings.HasPrefix(line, "From ") {
				if in {
					jobs <- job{idx: idx + 1, raw: buf.String()}
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

	// collect
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
		fmt.Printf("%d messages failed (see %s)\n", fc, failPath)
	}
	return out, nil
}

/* ────────────────────────────────────────────  single-email helper  ─────────────────────────────────── */

func (g *Gmail) processEmail(raw, user string) (types.Record, error) {
	msg, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		return types.Record{}, err
	}

	h := msg.Header
	date, _ := mail.ParseDate(h.Get("Date"))

	data := map[string]interface{}{
		"from":      h.Get("From"),
		"to":        h.Get("To"),
		"subject":   h.Get("Subject"),
		"myMessage": strings.EqualFold(h.Get("From"), user),
	}

	mt, params, _ := mime.ParseMediaType(h.Get("Content-Type"))
	var final string

	// ── multipart ───────────────────────────────────────
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
			r := io.Reader(p)
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
			p.Close()
		}
		if plain != "" {
			final = plain
		} else {
			final = html
		}

	} else { // ── single part ─────────────────────────────
		enc := strings.ToLower(h.Get("Content-Transfer-Encoding"))
		r := io.Reader(msg.Body)
		if enc == "quoted-printable" {
			r = quotedprintable.NewReader(r)
		}
		b, _ := io.ReadAll(r)

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

func (g *Gmail) Sync(ctx context.Context, token string) ([]types.Record, error) {
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
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gmail list: %d %s", resp.StatusCode, b)
	}

	var list struct {
		Messages []struct{ ID string } `json:"messages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, err
	}

	var out []types.Record
	for _, m := range list.Messages {
		rec, err := g.fetchMessage(ctx, c, token, m.ID)
		if err != nil {
			log.Errorf("message %s: %v", m.ID, err)
			continue
		}
		out = append(out, rec)
	}
	return out, nil
}

func (g *Gmail) fetchMessage(ctx context.Context, c *http.Client, token, id string) (types.Record, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("https://gmail.googleapis.com/gmail/v1/users/me/messages/%s", id), nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.Do(req)
	if err != nil {
		return types.Record{}, err
	}
	defer resp.Body.Close()
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

	// headers → map
	h := map[string]string{}
	for _, v := range msg.Payload.Headers {
		h[v.Name] = v.Value
	}
	date, _ := mail.ParseDate(h["Date"])

	// ── extract preferred body ───────────────────────────
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

	// single-part fallback
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
			"from":      h["From"],
			"to":        h["To"],
			"subject":   h["Subject"],
			"content":   content,
			"myMessage": false,
		},
		Timestamp: date,
		Source:    g.Name(),
	}, nil
}

/* ────────────────────────────────────────────  misc helpers  ────────────────────────────────────────── */

func decodeBase64URL(s string) (string, error) {
	if m := len(s) % 4; m != 0 {
		s += strings.Repeat("=", 4-m)
	}
	s = strings.ReplaceAll(s, "-", "+")
	s = strings.ReplaceAll(s, "_", "/")
	b, err := base64.StdEncoding.DecodeString(s)
	return string(b), err
}

func (g *Gmail) ProcessDirectory(dir, user string) ([]types.Record, error) {
	var all []types.Record
	var mu sync.Mutex
	err := filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() || !strings.Contains(fi.Name(), ".mbox") {
			return err
		}
		recs, err := g.ProcessFile(p, user)
		if err != nil {
			logrus.Errorf("process %s: %v", p, err)
			return nil
		}
		mu.Lock()
		all = append(all, recs...)
		mu.Unlock()
		return nil
	})
	return all, err
}

func ToDocuments(recs []types.Record) ([]memory.TextDocument, error) {
	out := []memory.TextDocument{}
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
		out = append(out, memory.TextDocument{
			Content:   get("content"),
			Timestamp: &r.Timestamp,
			Tags:      []string{"google", "email"},
			Metadata: map[string]string{
				"source":  "email",
				"from":    get("from"),
				"to":      get("to"),
				"subject": get("subject"),
			},
		})
	}
	return out, nil
}

// cleanEmailText normalises line-breaks, zaps zero-width & NBSP chars,
// collapses excess whitespace, and chops common footer / unsubscribe sections.
func cleanEmailText(s string) string {
	// 1) normalise breaks + common “invisible” chars
	repl := strings.NewReplacer(
		"\r\n", "\n", "\r", "\n",
		"\u00a0", " ", // NBSP
		"\u200c", "", // zero-width
		"\u2007", " ",
	)
	s = repl.Replace(s)

	// 2) per-line cleanup, stop at first footer clue
	var out []string
	for _, ln := range strings.Split(s, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		lc := strings.ToLower(ln)
		if strings.HasPrefix(lc, "unsubscribe") ||
			strings.Contains(lc, "to unsubscribe") ||
			strings.HasPrefix(lc, "update your preferences") ||
			strings.HasPrefix(lc, "©") ||
			strings.HasPrefix(lc, "google llc") ||
			strings.HasPrefix(lc, "this email was sent") {
			break // discard everything after the first footer hit
		}
		out = append(out, ln)
	}

	return strings.Join(out, "\n")
}

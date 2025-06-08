package main

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/log"
)

var allowedBaseURLs = map[string]string{
	"https://openrouter.ai/api/v1":                 os.Getenv("OPENROUTER_API_KEY"),
	"https://api.openai.com":                       os.Getenv("OPENAI_API_KEY"),
	"https://qwen2-5-72b.model.tinfoil.sh/v1":      os.Getenv("TINFOIL_API_KEY"),
	"https://nomic-embed-text.model.tinfoil.sh/v1": os.Getenv("TINFOIL_API_KEY"),
}

func main() {
	logger := log.NewWithOptions(os.Stdout, log.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
		Level:           log.DebugLevel,
		TimeFormat:      time.Kitchen,
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract X-BASE-URL from header
		baseURL := r.Header.Get("X-BASE-URL")
		if baseURL == "" {
			http.Error(w, "X-BASE-URL header is required", http.StatusBadRequest)
			return
		}

		// Check if base URL is in our allowed dictionary
		apiKey, exists := allowedBaseURLs[baseURL]
		if !exists {
			logger.Error("Unauthorized base URL", "base_url", baseURL)
			http.Error(w, "Unauthorized base URL", http.StatusForbidden)
			return
		}

		// Parse the target URL
		target, err := url.Parse(baseURL)
		if err != nil {
			logger.Error("Invalid URL format", "base_url", baseURL)
			http.Error(w, "Invalid URL format", http.StatusBadRequest)
			return
		}

		// Create reverse proxy for this specific target
		p := httputil.NewSingleHostReverseProxy(target)

		orig := p.Director
		p.Director = func(r *http.Request) {
			orig(r)
			logger.Info("📤 Forwarding request", "method", r.Method, "host", r.Host, "uri", r.RequestURI, "target", target.String()+r.RequestURI)

			r.Host = target.Host

			// Set Authorization header with Bearer token
			r.Header.Set("Authorization", "Bearer "+apiKey)

			// Handle User-Agent header
			userAgent := r.Header.Get("User-Agent")
			if !strings.Contains(userAgent, "OpenAI/Go") {
				r.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
			}

			// Clean up proxy headers
			r.Header.Del("X-Forwarded-For")
			r.Header.Del("X-Real-Ip")
			r.Header.Del("X-BASE-URL") // Remove our custom header before forwarding
		}

		p.ServeHTTP(w, r)
	})

	srv := &http.Server{
		Addr:    ":12000",
		Handler: handler,
	}

	logger.Info("🔁  proxy listening on :12000")
	logger.Info("✅  allowed base URLs", "paths", getKeys(allowedBaseURLs))
	log.Fatal(srv.ListenAndServe())
}

// Helper function to get keys from map for logging
func getKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

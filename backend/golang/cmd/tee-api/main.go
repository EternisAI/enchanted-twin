package main

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/eternisai/enchanted-twin/backend/golang/cmd/tee-api/config"
	"github.com/eternisai/enchanted-twin/backend/golang/cmd/tee-api/handlers"
	"github.com/eternisai/enchanted-twin/backend/golang/cmd/tee-api/services"
	"github.com/gin-gonic/gin"
)

var allowedBaseURLs = map[string]string{
	"https://openrouter.ai/api/v1":                 os.Getenv("OPENROUTER_API_KEY"),
	"https://api.openai.com/v1":                    os.Getenv("OPENAI_API_KEY"),
	"https://qwen2-5-72b.model.tinfoil.sh/v1":      os.Getenv("TINFOIL_API_KEY"),
	"https://nomic-embed-text.model.tinfoil.sh/v1": os.Getenv("TINFOIL_API_KEY"),
}

func getAPIKey(baseURL string, config *config.Config) string {
	switch baseURL {
	case "https://openrouter.ai/api/v1":
		return config.OpenRouterAPIKey
	case "https://api.openai.com/v1":
		return config.OpenAIAPIKey
	case "https://qwen2-5-72b.model.tinfoil.sh/v1":
		return config.TinfoilAPIKey
	case "https://nomic-embed-text.model.tinfoil.sh/v1":
		return config.TinfoilAPIKey
	}
	return ""
}

func main() {
	config.LoadConfig()

	logger := log.NewWithOptions(os.Stdout, log.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
		Level:           log.DebugLevel,
		TimeFormat:      time.Kitchen,
	})

	// Set Gin mode
	logger.Info("Setting Gin mode", "mode", config.AppConfig.GinMode)
	gin.SetMode(config.AppConfig.GinMode)

	// Initialize database
	db, err := config.InitDatabase()
	if err != nil {
		logger.Fatal("Failed to initialize database", "error", err)
	}

	// Initialize services
	oauthService := services.NewOAuthService()
	composioService := services.NewComposioService()
	inviteCodeService := services.NewInviteCodeService(db)

	// Initialize handlers
	oauthHandler := handlers.NewOAuthHandler(oauthService)
	composioHandler := handlers.NewComposioHandler(composioService)
	inviteCodeHandler := handlers.NewInviteCodeHandler(inviteCodeService)

	// Initialize Gin router
	router := gin.Default()

	// Add CORS middleware
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-BASE-URL")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// OAuth API routes
	auth := router.Group("/auth")
	{
		auth.POST("/exchange", oauthHandler.ExchangeToken)
		auth.POST("/refresh", oauthHandler.RefreshToken)
	}

	// Compose API routes
	compose := router.Group("/composio")
	{
		compose.POST("/auth", composioHandler.CreateConnectedAccount)
		compose.GET("/account", composioHandler.GetConnectedAccount)
		compose.GET("/refresh", composioHandler.RefreshToken)
	}

	// Invite code API routes
	api := router.Group("/api/v1")
	{
		invites := api.Group("/invites")
		{
			invites.GET("/:email/whitelist", inviteCodeHandler.CheckEmailWhitelist)
			invites.POST("/:code/redeem", inviteCodeHandler.RedeemInviteCode)
			invites.GET("/reset/:code", inviteCodeHandler.ResetInviteCode)
			invites.DELETE("/:id", inviteCodeHandler.DeleteInviteCode)
		}
	}

	router.Any("/chat/completions", proxyHandler)
	router.Any("/embeddings", proxyHandler)

	port := ":" + config.AppConfig.Port

	logger.Info("🔁  proxy listening on " + port)
	logger.Info("✅  allowed base URLs", "paths", getKeys(allowedBaseURLs))
	log.Fatal(router.Run(port))
}

// Helper function to get keys from map for logging
func getKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func proxyHandler(c *gin.Context) {
	// Extract X-BASE-URL from header

	baseURL := c.GetHeader("X-BASE-URL")
	if baseURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-BASE-URL header is required"})
		return
	}

	// Check if base URL is in our allowed dictionary
	apiKey := getAPIKey(baseURL, config.AppConfig)
	if apiKey == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized base URL"})
		return
	}

	// Parse the target URL
	target, err := url.Parse(baseURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid URL format"})
		return
	}

	// Create reverse proxy for this specific target
	proxy := httputil.NewSingleHostReverseProxy(target)

	orig := proxy.Director
	proxy.Director = func(r *http.Request) {
		orig(r)
		log.Printf("🔁 Forwarding request to %s", target.String()+r.RequestURI)
		log.Printf("📤 Forwarding %s %s%s to %s", r.Method, r.Host, r.RequestURI, target.String()+r.RequestURI)

		r.Host = target.Host

		// Set Authorization header with Bearer token
		r.Header.Set("Authorization", "Bearer "+apiKey)

		// Handle User-Agent header
		if userAgent := r.Header.Get("User-Agent"); strings.Contains(userAgent, "OpenAI/Go") {
		} else {
			r.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		}

		// Clean up proxy headers
		r.Header.Del("X-Forwarded-For")
		r.Header.Del("X-Real-Ip")
		r.Header.Del("X-BASE-URL") // Remove our custom header before forwarding
	}

	proxy.ServeHTTP(c.Writer, c.Request)
}

// Owner: claude@anthropic.com
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	"github.com/go-chi/chi"
	"github.com/pkg/errors"
	"github.com/rs/cors"
	"github.com/openai/openai-go"

	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/ai/coreml"
)

type Server struct {
	coremlService *coreml.Service
	openaiService *ai.Service
	logger        *log.Logger
	useLocal      bool
}

// Simple request/response structures for HTTP API
type CompletionRequest struct {
	Messages []Message `json:"messages"`
	Model    string    `json:"model,omitempty"`
	Stream   bool      `json:"stream,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type CompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Model   string   `json:"model"`
	Created int64    `json:"created"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func main() {
	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
		Level:           log.DebugLevel,
		TimeFormat:      time.Kitchen,
	})

	// Load minimal config - only OpenAI keys for fallback
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("COMPLETIONS_API_KEY")
	}
	logger.Debug("Config loaded for CoreML inference server", "has_openai_key", apiKey != "")

	// Determine model path - check if models are packaged with binary
	modelPath := "bin/models/test_coreml"
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		// Fallback to source directory during development
		modelPath = "test_coreml"
	}

	// Convert to absolute path
	absModelPath, err := filepath.Abs(modelPath)
	if err != nil {
		logger.Error("Failed to get absolute model path", "error", err)
		panic(errors.Wrap(err, "Failed to get absolute model path"))
	}

	logger.Info("Loading CoreML model", "path", absModelPath)

	// Initialize CoreML service
	coremlService, err := coreml.NewService(absModelPath, logger)
	if err != nil {
		logger.Error("Failed to initialize CoreML service", "error", err)
		// Continue without CoreML - fallback to OpenAI only
		coremlService = nil
	} else {
		logger.Info("CoreML service initialized successfully")
		defer coremlService.Close()
	}

	// Initialize OpenAI service as fallback
	var openaiService *ai.Service
	if apiKey != "" {
		baseURL := os.Getenv("OPENAI_BASE_URL")
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		openaiService = ai.NewOpenAIService(logger, apiKey, baseURL)
		logger.Info("OpenAI service initialized as fallback")
	}

	if coremlService == nil && openaiService == nil {
		logger.Error("Neither CoreML nor OpenAI service could be initialized")
		panic(errors.New("No AI service available"))
	}

	server := &Server{
		coremlService: coremlService,
		openaiService: openaiService,
		logger:        logger,
		useLocal:      coremlService != nil, // Prefer local if available
	}

	router := server.setupRouter()

	// Start HTTP server
	httpServer := &http.Server{
		Addr:    ":8081",
		Handler: router,
	}

	go func() {
		logger.Info("Starting CoreML Inference Server", "address", "http://localhost:8081")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error", "error", err)
			panic(errors.Wrap(err, "Unable to start server"))
		}
	}()

	// Wait for interrupt signal
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	<-signalChan
	logger.Info("CoreML Inference Server shutting down...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error("Server shutdown error", "error", err)
	}
}

func (s *Server) setupRouter() *chi.Mux {
	router := chi.NewRouter()
	
	// CORS middleware
	router.Use(cors.New(cors.Options{
		AllowCredentials: true,
		AllowedOrigins:   []string{"*"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "Accept"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		Debug:            false,
	}).Handler)

	// Health check endpoint
	router.Get("/health", s.healthHandler)
	router.Get("/", s.healthHandler)

	// AI completion endpoints (OpenAI compatible)
	router.Post("/v1/chat/completions", s.completionsHandler)
	router.Post("/chat/completions", s.completionsHandler)

	// Model switching endpoints
	router.Post("/switch/local", s.switchToLocalHandler)
	router.Post("/switch/remote", s.switchToRemoteHandler)
	router.Get("/status", s.statusHandler)

	return router
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"status": "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"services": map[string]bool{
			"coreml": s.coremlService != nil,
			"openai": s.openaiService != nil,
		},
		"current_mode": s.getCurrentMode(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (s *Server) completionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.logger.Error("Failed to decode request", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	
	// Convert to OpenAI format
	messages := s.convertToOpenAIMessages(req.Messages)
	model := req.Model
	if model == "" {
		model = "gpt-3.5-turbo"
	}

	// Choose service based on current mode
	var response CompletionResponse
	var err error

	if s.useLocal && s.coremlService != nil {
		s.logger.Debug("Using CoreML for inference")
		msg, err := s.coremlService.Completions(ctx, messages, nil, model)
		if err != nil && s.openaiService != nil {
			s.logger.Warn("CoreML inference failed, falling back to OpenAI", "error", err)
			msg, err = s.openaiService.Completions(ctx, messages, nil, model)
		}
		
		if err == nil {
			response = CompletionResponse{
				ID:      fmt.Sprintf("coreml-%d", time.Now().Unix()),
				Object:  "chat.completion",
				Model:   "coreml-local",
				Created: time.Now().Unix(),
				Choices: []Choice{
					{
						Index: 0,
						Message: Message{
							Role:    string(msg.Role),
							Content: msg.Content,
						},
						FinishReason: "stop",
					},
				},
				Usage: Usage{
					PromptTokens:     s.estimateTokens(s.formatMessages(req.Messages)),
					CompletionTokens: s.estimateTokens(msg.Content),
					TotalTokens:      s.estimateTokens(s.formatMessages(req.Messages)) + s.estimateTokens(msg.Content),
				},
			}
		}
	} else if s.openaiService != nil {
		s.logger.Debug("Using OpenAI for inference")
		msg, err := s.openaiService.Completions(ctx, messages, nil, model)
		if err == nil {
			response = CompletionResponse{
				ID:      fmt.Sprintf("openai-%d", time.Now().Unix()),
				Object:  "chat.completion", 
				Model:   model,
				Created: time.Now().Unix(),
				Choices: []Choice{
					{
						Index: 0,
						Message: Message{
							Role:    string(msg.Role),
							Content: msg.Content,
						},
						FinishReason: "stop",
					},
				},
				Usage: Usage{
					PromptTokens:     s.estimateTokens(s.formatMessages(req.Messages)),
					CompletionTokens: s.estimateTokens(msg.Content),
					TotalTokens:      s.estimateTokens(s.formatMessages(req.Messages)) + s.estimateTokens(msg.Content),
				},
			}
		}
	} else {
		err = errors.New("no AI service available")
	}

	if err != nil {
		s.logger.Error("Inference failed", "error", err)
		http.Error(w, fmt.Sprintf("Inference error: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) switchToLocalHandler(w http.ResponseWriter, r *http.Request) {
	if s.coremlService == nil {
		http.Error(w, "CoreML service not available", http.StatusServiceUnavailable)
		return
	}

	s.useLocal = true
	s.logger.Info("Switched to local CoreML inference")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "switched to local",
		"mode":   s.getCurrentMode(),
	})
}

func (s *Server) switchToRemoteHandler(w http.ResponseWriter, r *http.Request) {
	if s.openaiService == nil {
		http.Error(w, "OpenAI service not available", http.StatusServiceUnavailable)
		return
	}

	s.useLocal = false
	s.logger.Info("Switched to remote OpenAI inference")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "switched to remote",
		"mode":   s.getCurrentMode(),
	})
}

func (s *Server) statusHandler(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"current_mode": s.getCurrentMode(),
		"available_services": map[string]bool{
			"coreml": s.coremlService != nil,
			"openai": s.openaiService != nil,
		},
		"using_local": s.useLocal,
		"timestamp":   time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (s *Server) getCurrentMode() string {
	if s.useLocal && s.coremlService != nil {
		return "local_coreml"
	} else if s.openaiService != nil {
		return "remote_openai"
	}
	return "none"
}

// Helper methods
func (s *Server) convertToOpenAIMessages(messages []Message) []openai.ChatCompletionMessageParamUnion {
	var result []openai.ChatCompletionMessageParamUnion
	
	for _, msg := range messages {
		switch msg.Role {
		case "user":
			result = append(result, openai.UserMessage(msg.Content))
		case "assistant":
			result = append(result, openai.AssistantMessage(msg.Content))
		case "system":
			result = append(result, openai.SystemMessage(msg.Content))
		}
	}
	
	return result
}

func (s *Server) formatMessages(messages []Message) string {
	var prompt string
	for _, msg := range messages {
		prompt += fmt.Sprintf("%s: %s\n", msg.Role, msg.Content)
	}
	return prompt
}

func (s *Server) estimateTokens(text string) int {
	return len(text) / 4 // Rough estimation
}
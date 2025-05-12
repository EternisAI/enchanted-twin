package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gorilla/mux"
)

type TelegramAPIService struct {
	logger     *log.Logger
	token      string
	httpClient *http.Client
	dataDir    string
}

func main() {
	logger := log.New(os.Stdout, "[TELEGRAM-API-SERVICE] ", log.LstdFlags)
	logger.Println("Starting Telegram API service...")

	token := os.Getenv("TELEGRAM_TOKEN")
	if token == "" {
		logger.Fatal("TELEGRAM_TOKEN environment variable must be set")
	}

	dataDir := os.Getenv("TELEGRAM_DATA_DIR")
	if dataDir == "" {
		dataDir = "/data"
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		logger.Fatalf("Failed to create data directory: %v", err)
	}

	service := &TelegramAPIService{
		logger:     logger,
		token:      token,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		dataDir:    dataDir,
	}

	router := mux.NewRouter()
	router.HandleFunc("/health", service.healthHandler).Methods("GET")
	router.HandleFunc("/api/getMe", service.getMeHandler).Methods("GET")
	router.HandleFunc("/api/getChats", service.getChatsHandler).Methods("GET")
	router.HandleFunc("/api/sendMessage", service.sendMessageHandler).Methods("POST")

	port := os.Getenv("PORT")
	if port == "" {
		port = "9090"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		logger.Printf("Telegram API service listening on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Failed to start server: %v", err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatalf("Server shutdown failed: %v", err)
	}
	logger.Println("Server gracefully stopped")
}

func (s *TelegramAPIService) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (s *TelegramAPIService) getMeHandler(w http.ResponseWriter, r *http.Request) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getMe", s.token)
	
	resp, err := s.httpClient.Get(url)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get user info: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		http.Error(w, fmt.Sprintf("Telegram API returned status code: %d", resp.StatusCode), http.StatusInternalServerError)
		return
	}
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		http.Error(w, fmt.Sprintf("Failed to decode response: %v", err), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *TelegramAPIService) getChatsHandler(w http.ResponseWriter, r *http.Request) {
	limit := 100
	limitStr := r.URL.Query().Get("limit")
	if limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}
	
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?limit=%d", s.token, limit)
	
	resp, err := s.httpClient.Get(url)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get chats: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		http.Error(w, fmt.Sprintf("Telegram API returned status code: %d", resp.StatusCode), http.StatusInternalServerError)
		return
	}
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		http.Error(w, fmt.Sprintf("Failed to decode response: %v", err), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *TelegramAPIService) sendMessageHandler(w http.ResponseWriter, r *http.Request) {
	var request struct {
		ChatID  int64  `json:"chat_id"`
		Message string `json:"message"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}
	
	if request.ChatID == 0 || request.Message == "" {
		http.Error(w, "chat_id and message are required", http.StatusBadRequest)
		return
	}
	
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", s.token)
	
	requestBody := map[string]interface{}{
		"chat_id":    request.ChatID,
		"text":       request.Message,
		"parse_mode": "HTML",
	}
	
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to marshal request: %v", err), http.StatusInternalServerError)
		return
	}
	
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create request: %v", err), http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := s.httpClient.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to send message: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		http.Error(w, fmt.Sprintf("Telegram API returned status code: %d", resp.StatusCode), http.StatusInternalServerError)
		return
	}
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		http.Error(w, fmt.Sprintf("Failed to decode response: %v", err), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

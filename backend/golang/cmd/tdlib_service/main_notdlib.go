
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
)

type MockTDLibService struct {
	logger       *log.Logger
	authState    string
	authStateMux sync.RWMutex
	phoneNumber  string
	authCode     string
	password     string
	isAuthorized bool
	user         map[string]interface{}
	chats        []map[string]interface{}
}

func main() {
	logger := log.New(os.Stdout, "[MOCK-TDLIB-SERVICE] ", log.LstdFlags)
	logger.Println("Starting Mock TDLib service (built with notdlib tag)...")

	apiIDStr := os.Getenv("TELEGRAM_TDLIB_API_ID")
	apiHash := os.Getenv("TELEGRAM_TDLIB_API_HASH")

	if apiIDStr == "" || apiHash == "" {
		logger.Fatal("TELEGRAM_TDLIB_API_ID and TELEGRAM_TDLIB_API_HASH environment variables must be set")
	}

	service := &MockTDLibService{
		logger:    logger,
		authState: "waiting_for_phone_number",
		user: map[string]interface{}{
			"id":         12345,
			"first_name": "Mock",
			"last_name":  "User",
			"username":   "mock_user",
			"phone":      "",
		},
		chats: []map[string]interface{}{
			{
				"id":    67890,
				"title": "Mock Chat 1",
				"type":  "private",
			},
			{
				"id":    67891,
				"title": "Mock Chat 2",
				"type":  "group",
			},
		},
	}

	router := mux.NewRouter()
	router.HandleFunc("/health", service.healthHandler).Methods("GET")
	router.HandleFunc("/api/getMe", service.getMeHandler).Methods("GET")
	router.HandleFunc("/api/getChats", service.getChatsHandler).Methods("GET")
	router.HandleFunc("/api/sendMessage", service.sendMessageHandler).Methods("POST")
	router.HandleFunc("/api/authState", service.authStateHandler).Methods("GET")
	router.HandleFunc("/api/setPhoneNumber", service.setPhoneNumberHandler).Methods("POST")
	router.HandleFunc("/api/setAuthCode", service.setAuthCodeHandler).Methods("POST")
	router.HandleFunc("/api/setPassword", service.setPasswordHandler).Methods("POST")

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
		logger.Printf("Mock TDLib service listening on port %s", port)
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

func (s *MockTDLibService) setAuthState(state string) {
	s.authStateMux.Lock()
	defer s.authStateMux.Unlock()
	s.authState = state
	s.logger.Printf("Auth state changed to: %s", state)
}

func (s *MockTDLibService) getAuthState() string {
	s.authStateMux.RLock()
	defer s.authStateMux.RUnlock()
	return s.authState
}

func (s *MockTDLibService) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (s *MockTDLibService) authStateHandler(w http.ResponseWriter, r *http.Request) {
	state := s.getAuthState()
	
	response := struct {
		State string `json:"state"`
	}{
		State: state,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *MockTDLibService) setPhoneNumberHandler(w http.ResponseWriter, r *http.Request) {
	var request struct {
		PhoneNumber string `json:"phone_number"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	if request.PhoneNumber == "" {
		http.Error(w, "phone_number is required", http.StatusBadRequest)
		return
	}

	phoneNumber := strings.TrimSpace(request.PhoneNumber)
	s.phoneNumber = phoneNumber
	s.user["phone"] = phoneNumber
	
	s.setAuthState("waiting_for_code")
	
	response := struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}{
		Success: true,
		Message: "Phone number received, waiting for code",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *MockTDLibService) setAuthCodeHandler(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Code string `json:"code"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	if request.Code == "" {
		http.Error(w, "code is required", http.StatusBadRequest)
		return
	}

	code := strings.TrimSpace(request.Code)
	s.authCode = code
	
	if code == "2fa" {
		s.setAuthState("waiting_for_password")
	} else {
		s.isAuthorized = true
		s.setAuthState("authorized")
	}
	
	response := struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}{
		Success: true,
		Message: "Authentication code received",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *MockTDLibService) setPasswordHandler(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	if request.Password == "" {
		http.Error(w, "password is required", http.StatusBadRequest)
		return
	}

	password := strings.TrimSpace(request.Password)
	s.password = password
	
	s.isAuthorized = true
	s.setAuthState("authorized")
	
	response := struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}{
		Success: true,
		Message: "Password received, authentication successful",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *MockTDLibService) getMeHandler(w http.ResponseWriter, r *http.Request) {
	if !s.isAuthorized {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.user)
}

func (s *MockTDLibService) getChatsHandler(w http.ResponseWriter, r *http.Request) {
	if !s.isAuthorized {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	limit := 100
	limitStr := r.URL.Query().Get("limit")
	if limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err == nil && parsedLimit > 0 && parsedLimit < len(s.chats) {
			limit = parsedLimit
		}
	}

	result := map[string]interface{}{
		"total_count": len(s.chats),
		"chat_ids":    s.chats[:limit],
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *MockTDLibService) sendMessageHandler(w http.ResponseWriter, r *http.Request) {
	if !s.isAuthorized {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

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

	message := map[string]interface{}{
		"id":        time.Now().Unix(),
		"chat_id":   request.ChatID,
		"sender_id": s.user["id"],
		"content": map[string]interface{}{
			"text": map[string]interface{}{
				"text": request.Message,
			},
		},
		"date": time.Now().Unix(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(message)
}

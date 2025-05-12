
package main

import (
	"bufio"
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
	tdlibclient "github.com/zelenin/go-tdlib/client"
)

type TDLibService struct {
	client       *tdlibclient.Client
	logger       *log.Logger
	authState    string
	authStateMux sync.RWMutex
	phoneNumber  string
	authCode     string
	authChan     chan string
}

func main() {
	logger := log.New(os.Stdout, "[TDLIB-SERVICE] ", log.LstdFlags)
	logger.Println("Starting TDLib service...")

	apiIDStr := os.Getenv("TELEGRAM_TDLIB_API_ID")
	apiHash := os.Getenv("TELEGRAM_TDLIB_API_HASH")

	if apiIDStr == "" || apiHash == "" {
		logger.Fatal("TELEGRAM_TDLIB_API_ID and TELEGRAM_TDLIB_API_HASH environment variables must be set")
	}

	apiID, err := strconv.Atoi(apiIDStr)
	if err != nil {
		logger.Fatalf("Invalid TELEGRAM_TDLIB_API_ID: %v", err)
	}

	service := &TDLibService{
		logger:    logger,
		authState: "waiting_for_parameters",
		authChan:  make(chan string, 1),
	}

	go func() {
		client, err := initializeTDLib(logger, int32(apiID), apiHash, service)
		if err != nil {
			logger.Fatalf("Failed to initialize TDLib: %v", err)
		}
		service.client = client
		service.setAuthState("ready")
		logger.Println("TDLib client initialized successfully")
	}()

	router := mux.NewRouter()
	router.HandleFunc("/health", service.healthHandler).Methods("GET")
	router.HandleFunc("/api/getMe", service.getMeHandler).Methods("GET")
	router.HandleFunc("/api/getChats", service.getChatsHandler).Methods("GET")
	router.HandleFunc("/api/sendMessage", service.sendMessageHandler).Methods("POST")
	router.HandleFunc("/api/authState", service.authStateHandler).Methods("GET")
	router.HandleFunc("/api/setPhoneNumber", service.setPhoneNumberHandler).Methods("POST")
	router.HandleFunc("/api/setAuthCode", service.setAuthCodeHandler).Methods("POST")

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
		logger.Printf("TDLib service listening on port %s", port)
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

func (s *TDLibService) setAuthState(state string) {
	s.authStateMux.Lock()
	defer s.authStateMux.Unlock()
	s.authState = state
	s.logger.Printf("Auth state changed to: %s", state)
}

func (s *TDLibService) getAuthState() string {
	s.authStateMux.RLock()
	defer s.authStateMux.RUnlock()
	return s.authState
}

func initializeTDLib(logger *log.Logger, apiID int32, apiHash string, service *TDLibService) (*tdlibclient.Client, error) {
	logger.Println("Initializing TDLib client...")

	dbDir := os.Getenv("TELEGRAM_TDLIB_DB_DIR")
	if dbDir == "" {
		dbDir = "/tdlib/db"
	}

	filesDir := os.Getenv("TELEGRAM_TDLIB_FILES_DIR")
	if filesDir == "" {
		filesDir = "/tdlib/files"
	}

	os.MkdirAll(dbDir, 0755)
	os.MkdirAll(filesDir, 0755)

	_, err := tdlibclient.SetLogVerbosityLevel(&tdlibclient.SetLogVerbosityLevelRequest{
		NewVerbosityLevel: 2,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to set log verbosity level: %w", err)
	}

	tdlibParameters := &tdlibclient.SetTdlibParametersRequest{
		UseTestDc:           false,
		DatabaseDirectory:   dbDir,
		FilesDirectory:      filesDir,
		UseFileDatabase:     true,
		UseChatInfoDatabase: true,
		UseMessageDatabase:  true,
		UseSecretChats:      false,
		ApiId:               apiID,
		ApiHash:             apiHash,
		SystemLanguageCode:  "en",
		DeviceModel:         "Server",
		SystemVersion:       "1.0.0",
		ApplicationVersion:  "0.1.0",
	}

	authorizer := func() (tdlibclient.AuthorizationState, error) {
		_, err := tdlibclient.SetTdlibParameters(tdlibParameters)
		if err != nil {
			return nil, err
		}

		authState, err := tdlibclient.CheckDatabaseEncryptionKey(&tdlibclient.CheckDatabaseEncryptionKeyRequest{
			EncryptionKey: "",
		})
		if err != nil {
			return nil, err
		}

		for {
			state := authState.AuthorizationStateType()
			logger.Printf("Current authorization state: %s", state)

			switch state {
			case tdlibclient.TypeAuthorizationStateWaitTdlibParameters:
				service.setAuthState("waiting_for_parameters")
				authState, err = tdlibclient.SetTdlibParameters(tdlibParameters)
			case tdlibclient.TypeAuthorizationStateWaitEncryptionKey:
				service.setAuthState("waiting_for_encryption_key")
				authState, err = tdlibclient.CheckDatabaseEncryptionKey(&tdlibclient.CheckDatabaseEncryptionKeyRequest{
					EncryptionKey: "",
				})
			case tdlibclient.TypeAuthorizationStateWaitPhoneNumber:
				service.setAuthState("waiting_for_phone_number")
				logger.Println("Waiting for phone number...")
				
				phoneNumber := <-service.authChan
				logger.Printf("Received phone number: %s", phoneNumber)
				
				authState, err = tdlibclient.SetAuthenticationPhoneNumber(&tdlibclient.SetAuthenticationPhoneNumberRequest{
					PhoneNumber: phoneNumber,
				})
			case tdlibclient.TypeAuthorizationStateWaitCode:
				service.setAuthState("waiting_for_code")
				logger.Println("Waiting for authentication code...")
				
				code := <-service.authChan
				logger.Printf("Received authentication code: %s", code)
				
				authState, err = tdlibclient.CheckAuthenticationCode(&tdlibclient.CheckAuthenticationCodeRequest{
					Code: code,
				})
			case tdlibclient.TypeAuthorizationStateWaitPassword:
				service.setAuthState("waiting_for_password")
				logger.Println("Waiting for password...")
				
				password := <-service.authChan
				logger.Printf("Received password")
				
				authState, err = tdlibclient.CheckAuthenticationPassword(&tdlibclient.CheckAuthenticationPasswordRequest{
					Password: password,
				})
			case tdlibclient.TypeAuthorizationStateReady:
				service.setAuthState("authorized")
				logger.Println("Authorization complete!")
				return authState, nil
			case tdlibclient.TypeAuthorizationStateLoggingOut:
				service.setAuthState("logging_out")
				logger.Println("Logging out...")
			case tdlibclient.TypeAuthorizationStateClosing:
				service.setAuthState("closing")
				logger.Println("Closing...")
			case tdlibclient.TypeAuthorizationStateClosed:
				service.setAuthState("closed")
				logger.Println("Closed.")
			default:
				service.setAuthState("unknown")
				logger.Printf("Unknown authorization state: %s", state)
			}

			if err != nil {
				return nil, err
			}
		}
	}

	clientCh := make(chan struct {
		client *tdlibclient.Client
		err    error
	}, 1)

	go func() {
		client, err := tdlibclient.NewClient(authorizer)
		clientCh <- struct {
			client *tdlibclient.Client
			err    error
		}{client, err}
	}()

	select {
	case result := <-clientCh:
		if result.err != nil {
			return nil, fmt.Errorf("failed to create TDLib client: %w", result.err)
		}
		return result.client, nil
	case <-time.After(600 * time.Second): // Longer timeout for authentication
		return nil, fmt.Errorf("client creation timed out")
	}
}

func (s *TDLibService) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (s *TDLibService) authStateHandler(w http.ResponseWriter, r *http.Request) {
	state := s.getAuthState()
	
	response := struct {
		State string `json:"state"`
	}{
		State: state,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *TDLibService) setPhoneNumberHandler(w http.ResponseWriter, r *http.Request) {
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
	
	if s.getAuthState() == "waiting_for_phone_number" {
		s.authChan <- phoneNumber
	} else {
		http.Error(w, "Not waiting for phone number", http.StatusBadRequest)
		return
	}

	response := struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}{
		Success: true,
		Message: "Phone number received",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *TDLibService) setAuthCodeHandler(w http.ResponseWriter, r *http.Request) {
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
	
	if s.getAuthState() == "waiting_for_code" {
		s.authChan <- code
	} else {
		http.Error(w, "Not waiting for authentication code", http.StatusBadRequest)
		return
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

func (s *TDLibService) getMeHandler(w http.ResponseWriter, r *http.Request) {
	if s.client == nil {
		http.Error(w, "TDLib client not initialized", http.StatusServiceUnavailable)
		return
	}

	me, err := s.client.GetMe()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get user info: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(me)
}

func (s *TDLibService) getChatsHandler(w http.ResponseWriter, r *http.Request) {
	if s.client == nil {
		http.Error(w, "TDLib client not initialized", http.StatusServiceUnavailable)
		return
	}

	limit := 100
	limitStr := r.URL.Query().Get("limit")
	if limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	mainChatList := tdlibclient.ChatListMain()

	chats, err := s.client.GetChats(&tdlibclient.GetChatsRequest{
		ChatList: mainChatList,
		Limit:    int32(limit),
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get chats: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(chats)
}

func (s *TDLibService) sendMessageHandler(w http.ResponseWriter, r *http.Request) {
	if s.client == nil {
		http.Error(w, "TDLib client not initialized", http.StatusServiceUnavailable)
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

	content := tdlibclient.InputMessageText{
		Text: tdlibclient.FormattedText{
			Text: request.Message,
		},
	}

	message, err := s.client.SendMessage(&tdlibclient.SendMessageRequest{
		ChatId:              request.ChatID,
		InputMessageContent: &content,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to send message: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(message)
}

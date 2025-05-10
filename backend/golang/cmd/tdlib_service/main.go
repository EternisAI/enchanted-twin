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
	"syscall"
	"time"

	"github.com/gorilla/mux"
	tdlibclient "github.com/zelenin/go-tdlib/client"
)

type TDLibService struct {
	client *tdlibclient.Client
	logger *log.Logger
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

	client, err := initializeTDLib(logger, int32(apiID), apiHash)
	if err != nil {
		logger.Fatalf("Failed to initialize TDLib: %v", err)
	}
	defer client.Close()

	service := &TDLibService{
		client: client,
		logger: logger,
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

func initializeTDLib(logger *log.Logger, apiID int32, apiHash string) (*tdlibclient.Client, error) {
	logger.Println("Initializing TDLib client...")

	dbDir := os.Getenv("TELEGRAM_TDLIB_DB_DIR")
	if dbDir == "" {
		dbDir = "/tdlib/db"
	}

	filesDir := os.Getenv("TELEGRAM_TDLIB_FILES_DIR")
	if filesDir == "" {
		filesDir = "/tdlib/files"
	}

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

	authorizer := tdlibclient.ClientAuthorizer(tdlibParameters)

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
	case <-time.After(60 * time.Second):
		return nil, fmt.Errorf("client creation timed out")
	}
}

func (s *TDLibService) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (s *TDLibService) getMeHandler(w http.ResponseWriter, r *http.Request) {
	me, err := s.client.GetMe()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get user info: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(me)
}

func (s *TDLibService) getChatsHandler(w http.ResponseWriter, r *http.Request) {
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

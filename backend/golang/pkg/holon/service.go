package holon

import (
	"context"
	"fmt"
	"os"
	"time"

	clog "github.com/charmbracelet/log"
	"github.com/google/uuid"

	"github.com/EternisAI/enchanted-twin/graph/model"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/db"
)

// Service provides higher-level operations over the holon data store.
type Service struct {
	store  *db.Store
	repo   *Repository
	logger *clog.Logger

	// Configuration
	holonAPIURL string

	// Remote authentication info
	participantID   *int
	displayName     *string
	isAuthenticated bool

	fetcherService      *FetcherService
	threadProcessor     *ThreadProcessor
	backgroundProcessor *BackgroundProcessor
	memoryService       evolvingmemory.MemoryStorage
	aiService           *ai.Service
	completionsModel    string
}

// NewServiceWithLogger creates a new holon service with a logger.
func NewServiceWithLogger(store *db.Store, logger *clog.Logger) *Service {
	return NewServiceWithConfig(store, logger, "")
}

// NewServiceWithConfig creates a new holon service with logger and API URL.
func NewServiceWithConfig(store *db.Store, logger *clog.Logger, holonAPIURL string) *Service {
	// Use default URL if not provided
	if holonAPIURL == "" {
		if value := os.Getenv("HOLON_API_URL"); value != "" {
			holonAPIURL = value
		} else {
			holonAPIURL = "http://localhost:8080"
		}
	}

	service := &Service{
		store:       store,
		repo:        NewRepository(store.DB()),
		logger:      logger,
		holonAPIURL: holonAPIURL,
	}

	// Attempt remote authentication during initialization if Google OAuth token is available
	ctx := context.Background()
	if err := service.performRemoteAuth(ctx); err != nil {
		if logger != nil {
			logger.Debug("Remote authentication not performed during service initialization", "error", err)
		}
		// Don't fail service creation if remote auth fails
	}

	return service
}

// performRemoteAuth attempts to authenticate with the holon network using Google OAuth token.
func (s *Service) performRemoteAuth(ctx context.Context) error {
	// Check if we already have authentication info
	if s.isAuthenticated {
		return nil
	}

	// Use configured API URL
	apiURL := s.holonAPIURL

	// Attempt to authenticate with HolonZero API
	authResp, err := AuthenticateWithHolonZero(ctx, apiURL, s.store, s.logger)
	if err != nil {
		return fmt.Errorf("failed to authenticate with holon network: %w", err)
	}

	// Store authentication info
	s.participantID = &authResp.ID
	s.displayName = &authResp.DisplayName
	s.isAuthenticated = true

	if s.logger != nil {
		s.logger.Info("Successfully authenticated with holon network",
			"participantID", authResp.ID,
			"displayName", authResp.DisplayName,
			"email", authResp.Email)
	}

	return nil
}

// GetParticipantInfo returns the authenticated participant information.
func (s *Service) GetParticipantInfo() (participantID *int, displayName *string, isAuthenticated bool) {
	return s.participantID, s.displayName, s.isAuthenticated
}

// EnsureAuthenticated attempts to authenticate if not already done.
func (s *Service) EnsureAuthenticated(ctx context.Context) error {
	if s.isAuthenticated {
		return nil
	}
	return s.performRemoteAuth(ctx)
}

func (s *Service) GetHolons(ctx context.Context, userID string) ([]string, error) {
	return s.repo.GetHolons(ctx, userID)
}

func (s *Service) GetThreads(ctx context.Context, first int32, offset int32) ([]*model.Thread, error) {
	return s.repo.GetDisplayThreads(ctx, first, offset)
}

// GetAllThreads returns all threads regardless of state (for internal use).
func (s *Service) GetAllThreads(ctx context.Context, first int32, offset int32) ([]*model.Thread, error) {
	return s.repo.GetThreads(ctx, first, offset)
}

func (s *Service) GetThread(ctx context.Context, threadID string) (*model.Thread, error) {
	thread, err := s.repo.GetThread(ctx, threadID)
	if err != nil {
		return nil, err
	}

	if err := s.repo.IncrementThreadViews(ctx, threadID); err != nil {
		fmt.Println("failed to increment thread views", err)
	}

	return thread, nil
}

func (s *Service) JoinHolonNetwork(ctx context.Context, userID string, networkName string) error {
	isInHolon, err := s.repo.IsUserInHolon(ctx, userID, networkName)
	if err != nil {
		return fmt.Errorf("failed to check holon membership: %w", err)
	}

	_, err = s.repo.CreateOrUpdateAuthor(ctx, userID, userID)
	if err != nil {
		return fmt.Errorf("failed to create/update author: %w", err)
	}

	if !isInHolon {
		err = s.repo.AddUserToHolon(ctx, userID, networkName)
		if err != nil {
			return fmt.Errorf("failed to add user to holon network: %w", err)
		}
	}

	return nil
}

func (s *Service) SendToHolon(ctx context.Context, threadPreviewID, title, content, authorIdentity string, imageURLs []string, actions []string) (*model.Thread, error) {
	threadID := "published-" + threadPreviewID

	// Use standardized local user identity for locally created threads
	localAuthorIdentity := s.resolveLocalAuthorIdentity(authorIdentity)

	publishedThread, err := s.repo.CreateThread(ctx, threadID, title, content, localAuthorIdentity, imageURLs, actions, nil, "pending", "")
	if err != nil {
		return nil, fmt.Errorf("failed to create thread: %w", err)
	}

	return publishedThread, nil
}

func (s *Service) CreateThreadMessage(ctx context.Context, threadID, authorIdentity, content string, actions []string, isDelivered *bool) (*model.ThreadMessage, error) {
	messageID := uuid.New().String()

	// Use standardized local user identity for locally created messages
	localAuthorIdentity := s.resolveLocalAuthorIdentity(authorIdentity)

	return s.repo.CreateThreadMessage(ctx, messageID, threadID, localAuthorIdentity, content, actions, isDelivered)
}

func (s *Service) AddMessageToThread(ctx context.Context, threadID, message, authorIdentity string, imageURLs []string) (*model.ThreadMessage, error) {
	actions := []string{}
	isDelivered := false

	// Use standardized local user identity for locally created messages
	localAuthorIdentity := s.resolveLocalAuthorIdentity(authorIdentity)

	return s.CreateThreadMessage(ctx, threadID, localAuthorIdentity, message, actions, &isDelivered)
}

func (s *Service) GetTotalHolonCount() (int, error) {
	ctx := context.Background()
	repo := NewRepository(s.store.DB())

	// Get count of threads as a proxy for holons
	count, err := repo.GetThreadCount(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get thread count: %w", err)
	}

	return count, nil
}

func (s *Service) GetLastSyncTime() (*time.Time, error) {
	// In a real implementation, you'd store this in the database
	// For now, we'll just return nil which indicates no sync has occurred
	return nil, nil
}

func (s *Service) GetFetcherStatus() (*SyncStatus, error) {
	if s.fetcherService == nil {
		return nil, fmt.Errorf("fetcher service is not initialized")
	}
	return s.fetcherService.GetSyncStatus(context.Background())
}

// RefreshAuthentication re-authenticates with updated OAuth token.
func (s *Service) RefreshAuthentication(ctx context.Context) error {
	if s.logger != nil {
		s.logger.Debug("Refreshing holon service authentication with updated OAuth token")
	}

	// Reset authentication state
	s.participantID = nil
	s.displayName = nil
	s.isAuthenticated = false

	// Re-authenticate with fresh token
	if err := s.performRemoteAuth(ctx); err != nil {
		if s.logger != nil {
			s.logger.Error("Failed to refresh holon service authentication", "error", err)
		}
		return fmt.Errorf("failed to refresh authentication: %w", err)
	}

	if s.logger != nil {
		s.logger.Debug("Successfully refreshed holon service authentication")
	}
	return nil
}

// getLocalUserIdentity returns the standardized local user identity.
func (s *Service) getLocalUserIdentity() string {
	// Use the same identity as the fetcher service for consistency
	return "local-user"
}

// resolveLocalAuthorIdentity ensures we use consistent local user identity
// This should be used for all locally created content.
func (s *Service) resolveLocalAuthorIdentity(providedIdentity string) string {
	// For local content creation, always use the standardized local user identity
	// regardless of what was provided (e.g., email, username, etc.)
	return s.getLocalUserIdentity()
}

// InitializeThreadProcessor sets up the thread processor with AI and memory services.
func (s *Service) InitializeThreadProcessor(aiService *ai.Service, completionsModel string, memoryService evolvingmemory.MemoryStorage) {
	s.aiService = aiService
	s.completionsModel = completionsModel
	s.memoryService = memoryService

	if aiService != nil && completionsModel != "" {
		s.threadProcessor = NewThreadProcessor(
			s.logger,
			aiService,
			completionsModel,
			s.repo,
			memoryService,
		)
		if s.logger != nil {
			s.logger.Info("Thread processor initialized successfully")
		}
	} else {
		if s.logger != nil {
			s.logger.Warn("Thread processor not initialized - missing AI service or completions model")
		}
	}
}

// ProcessReceivedThreads processes all threads with 'received' state using LLM evaluation.
func (s *Service) ProcessReceivedThreads(ctx context.Context) error {
	if s.threadProcessor == nil {
		return fmt.Errorf("thread processor not initialized")
	}

	return s.threadProcessor.ProcessReceivedThreads(ctx)
}

// ProcessSingleReceivedThread processes a specific thread that has been received.
func (s *Service) ProcessSingleReceivedThread(ctx context.Context, threadID string) error {
	if s.threadProcessor == nil {
		return fmt.Errorf("thread processor not initialized")
	}

	// Get the thread
	thread, err := s.repo.GetThread(ctx, threadID)
	if err != nil {
		return fmt.Errorf("failed to get thread: %w", err)
	}

	// Only process if it's in 'received' state
	if len(thread.Messages) == 0 {
		// For threads without messages, we need to check the thread state directly
		// This is a simplified check - in a real implementation you'd query the state from DB
		if s.logger != nil {
			s.logger.Debug("Thread has no messages, processing anyway", "thread_id", threadID)
		}
	}

	return s.threadProcessor.ProcessSingleThread(ctx, thread)
}

// BootstrapProcessAllReceivedThreads is called during system bootstrap to process any existing received threads.
func (s *Service) BootstrapProcessAllReceivedThreads(ctx context.Context) error {
	if s.threadProcessor == nil {
		if s.logger != nil {
			s.logger.Warn("Thread processor not initialized, skipping bootstrap processing")
		}
		return nil
	}

	if s.logger != nil {
		s.logger.Info("Starting bootstrap processing of received threads")
	}

	if err := s.ProcessReceivedThreads(ctx); err != nil {
		if s.logger != nil {
			s.logger.Error("Failed to process received threads during bootstrap", "error", err)
		}
		return err
	}

	if s.logger != nil {
		s.logger.Info("Bootstrap processing of received threads completed")
	}
	return nil
}

// GetThreadsByState returns threads filtered by state.
func (s *Service) GetThreadsByState(ctx context.Context, state string) ([]*model.Thread, error) {
	return s.repo.GetThreadsByState(ctx, state)
}

// IsThreadProcessorReady returns true if the thread processor is initialized and ready.
func (s *Service) IsThreadProcessorReady() bool {
	return s.threadProcessor != nil
}

// InitializeBackgroundProcessor sets up the background processor for automatic thread processing.
func (s *Service) InitializeBackgroundProcessor(processingInterval time.Duration) {
	if s.backgroundProcessor != nil {
		if s.logger != nil {
			s.logger.Warn("Background processor already initialized")
		}
		return
	}

	s.backgroundProcessor = NewBackgroundProcessor(s, s.logger, processingInterval)
	if s.logger != nil {
		s.logger.Info("Background processor initialized", "interval", processingInterval)
	}
}

// StartBackgroundProcessing starts the background thread processing.
func (s *Service) StartBackgroundProcessing(ctx context.Context) error {
	if s.backgroundProcessor == nil {
		return fmt.Errorf("background processor not initialized")
	}

	return s.backgroundProcessor.Start(ctx)
}

// StopBackgroundProcessing stops the background thread processing.
func (s *Service) StopBackgroundProcessing() {
	if s.backgroundProcessor != nil {
		s.backgroundProcessor.Stop()
	}
}

// ProcessNewThreadImmediately processes a newly arrived thread immediately.
func (s *Service) ProcessNewThreadImmediately(ctx context.Context, threadID string) error {
	if s.backgroundProcessor == nil {
		return fmt.Errorf("background processor not initialized")
	}

	return s.backgroundProcessor.ProcessSingleThreadNow(ctx, threadID)
}

// GetThreadProcessingStatus returns the current status of thread processing.
func (s *Service) GetThreadProcessingStatus() map[string]interface{} {
	status := map[string]interface{}{
		"processor_ready": s.IsThreadProcessorReady(),
	}

	if s.backgroundProcessor != nil {
		backgroundStatus := s.backgroundProcessor.GetStatus()
		for k, v := range backgroundStatus {
			status["background_"+k] = v
		}
	} else {
		status["background_initialized"] = false
	}

	return status
}

// GetThreadEvaluationData returns evaluation data for a specific thread.
func (s *Service) GetThreadEvaluationData(ctx context.Context, threadID string) (*ThreadEvaluationData, error) {
	return s.repo.GetThreadEvaluationData(ctx, threadID)
}

// GetThreadsWithEvaluationStats returns threads with their evaluation statistics.
func (s *Service) GetThreadsWithEvaluationStats(ctx context.Context, limit int) ([]*ThreadWithEvaluation, error) {
	if limit <= 0 {
		limit = 50 // Default limit
	}
	return s.repo.GetThreadsWithEvaluationStats(ctx, limit)
}

// GetEvaluationStatistics returns overall evaluation statistics.
func (s *Service) GetEvaluationStatistics(ctx context.Context) (map[string]interface{}, error) {
	// Get recent evaluations
	evaluations, err := s.repo.GetThreadsWithEvaluationStats(ctx, 1000)
	if err != nil {
		return nil, fmt.Errorf("failed to get evaluation stats: %w", err)
	}

	totalEvaluated := len(evaluations)
	if totalEvaluated == 0 {
		return map[string]interface{}{
			"total_evaluated":    0,
			"visible_count":      0,
			"hidden_count":       0,
			"average_confidence": 0.0,
			"visible_percentage": 0.0,
		}, nil
	}

	visibleCount := 0
	hiddenCount := 0
	totalConfidence := 0.0

	for _, eval := range evaluations {
		if eval.EvaluationConfidence != nil {
			totalConfidence += *eval.EvaluationConfidence
		}

		switch eval.State {
		case "visible":
			visibleCount++
		case "hidden":
			hiddenCount++
		}
	}

	avgConfidence := totalConfidence / float64(totalEvaluated)
	visiblePercentage := float64(visibleCount) / float64(totalEvaluated) * 100

	return map[string]interface{}{
		"total_evaluated":    totalEvaluated,
		"visible_count":      visibleCount,
		"hidden_count":       hiddenCount,
		"average_confidence": avgConfidence,
		"visible_percentage": visiblePercentage,
	}, nil
}

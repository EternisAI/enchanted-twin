package holon

import (
	"context"
	"fmt"
	"os"
	"time"

	clog "github.com/charmbracelet/log"
	"github.com/google/uuid"

	"github.com/EternisAI/enchanted-twin/graph/model"
	"github.com/EternisAI/enchanted-twin/pkg/db"
)

// Service provides higher-level operations over the holon data store
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
}

// NewService creates a new holon service
func NewService(store *db.Store) *Service {
	return NewServiceWithLogger(store, nil)
}

// NewServiceWithLogger creates a new holon service with a logger
func NewServiceWithLogger(store *db.Store, logger *clog.Logger) *Service {
	return NewServiceWithConfig(store, logger, "")
}

// NewServiceWithConfig creates a new holon service with logger and API URL
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

// performRemoteAuth attempts to authenticate with the holon network using Google OAuth token
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

// GetParticipantInfo returns the authenticated participant information
func (s *Service) GetParticipantInfo() (participantID *int, displayName *string, isAuthenticated bool) {
	return s.participantID, s.displayName, s.isAuthenticated
}

// EnsureAuthenticated attempts to authenticate if not already done
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

func extractTitleFromContent(content string) string {
	if len(content) == 0 {
		return "Untitled Thread"
	}

	for i, char := range content {
		if char == '\n' {
			if i > 0 {
				return content[:i]
			}
			break
		}
	}

	if len(content) > 50 {
		return content[:47] + "..."
	}

	return content
}

// getLocalUserIdentity returns the standardized local user identity
func (s *Service) getLocalUserIdentity() string {
	// Use the same identity as the fetcher service for consistency
	return "local-user"
}

// resolveLocalAuthorIdentity ensures we use consistent local user identity
// This should be used for all locally created content
func (s *Service) resolveLocalAuthorIdentity(providedIdentity string) string {
	// For local content creation, always use the standardized local user identity
	// regardless of what was provided (e.g., email, username, etc.)
	return s.getLocalUserIdentity()
}

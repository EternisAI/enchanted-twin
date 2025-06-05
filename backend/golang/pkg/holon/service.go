package holon

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/EternisAI/enchanted-twin/graph/model"
	"github.com/EternisAI/enchanted-twin/pkg/db"
)

// Service provides higher-level operations over the holon data store
type Service struct {
	store *db.Store
	repo  *Repository
}

// NewService creates a new holon service
func NewService(store *db.Store) *Service {
	return &Service{
		store: store,
		repo:  NewRepository(store.DB()),
	}
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

	s.repo.IncrementThreadViews(ctx, threadID)

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

	publishedThread, err := s.repo.CreateThread(ctx, threadID, title, content, authorIdentity, imageURLs, actions, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create thread: %w", err)
	}

	return publishedThread, nil
}

func (s *Service) CreateThreadMessage(ctx context.Context, threadID, authorIdentity, content string, actions []string, isDelivered *bool) (*model.ThreadMessage, error) {
	messageID := uuid.New().String()

	return s.repo.CreateThreadMessage(ctx, messageID, threadID, authorIdentity, content, actions, isDelivered)
}

func (s *Service) AddMessageToThread(ctx context.Context, threadID, message, authorIdentity string, imageURLs []string) (*model.ThreadMessage, error) {
	actions := []string{}
	isDelivered := false

	return s.CreateThreadMessage(ctx, threadID, authorIdentity, message, actions, &isDelivered)
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

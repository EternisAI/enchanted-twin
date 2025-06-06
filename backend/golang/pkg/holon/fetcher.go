package holon

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/db"
	clog "github.com/charmbracelet/log"
)

// HolonZeroClient defines the interface for interacting with the HolonZero API
type HolonZeroClient interface {
	GetHealth(ctx context.Context) (*HealthResponse, error)
	ListParticipants(ctx context.Context) ([]Participant, error)
	ListThreads(ctx context.Context, query *ThreadsQuery) ([]Thread, error)
	ListThreadsPaginated(ctx context.Context, query *ThreadsQuery) (*PaginatedThreadsResponse, error)
	GetThreadRepliesPaginated(ctx context.Context, threadID int, query *RepliesQuery) (*PaginatedRepliesResponse, error)
	GetSyncMetadata(ctx context.Context) (*SyncMetadataResponse, error)
}

// FetcherService handles syncing data from HolonZero API to local database
type FetcherService struct {
	client     HolonZeroClient
	repository *Repository
	config     FetcherConfig
	logger     *clog.Logger
	stopChan   chan struct{}
	running    bool
}

// FetcherConfig holds configuration for the fetcher service
type FetcherConfig struct {
	APIBaseURL    string
	FetchInterval time.Duration
	BatchSize     int
	MaxRetries    int
	RetryDelay    time.Duration
	EnableLogging bool
}

// DefaultFetcherConfig returns a sensible default configuration
func DefaultFetcherConfig() FetcherConfig {
	return FetcherConfig{
		APIBaseURL:    "http://localhost:8080",
		FetchInterval: 5 * time.Minute,
		BatchSize:     50,
		MaxRetries:    3,
		RetryDelay:    30 * time.Second,
		EnableLogging: true,
	}
}

// NewFetcherService creates a new HolonZero API fetcher service
func NewFetcherService(store *db.Store, config FetcherConfig, logger *clog.Logger) *FetcherService {
	client := NewAPIClient(
		config.APIBaseURL,
		WithTimeout(30*time.Second),
		WithLogger(logger),
	)

	return &FetcherService{
		client:     client,
		repository: NewRepository(store.DB()),
		config:     config,
		logger:     logger,
		stopChan:   make(chan struct{}),
		running:    false,
	}
}

// Start begins the periodic fetching process
func (f *FetcherService) Start(ctx context.Context) error {
	if f.running {
		return fmt.Errorf("fetcher service is already running")
	}

	f.running = true
	f.logInfo("Starting HolonZero API fetcher service")

	// Perform initial fetch
	if err := f.performSync(ctx); err != nil {
		f.logError("Initial sync failed", err)
	}

	// Start periodic fetching
	ticker := time.NewTicker(f.config.FetchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			f.logInfo("Context cancelled, stopping fetcher service")
			f.running = false
			return ctx.Err()
		case <-f.stopChan:
			f.logInfo("Stop signal received, stopping fetcher service")
			f.running = false
			return nil
		case <-ticker.C:
			if err := f.performSync(ctx); err != nil {
				f.logError("Periodic sync failed", err)
			}
		}
	}
}

// Stop gracefully stops the fetching process
func (f *FetcherService) Stop() {
	if f.running {
		f.logInfo("Stopping HolonZero API fetcher service")
		close(f.stopChan)
	}
}

// IsRunning returns whether the fetcher service is currently running
func (f *FetcherService) IsRunning() bool {
	return f.running
}

// performSync executes a complete sync operation with retry logic
func (f *FetcherService) performSync(ctx context.Context) error {
	var lastErr error

	for attempt := 1; attempt <= f.config.MaxRetries; attempt++ {
		if err := f.syncData(ctx); err != nil {
			lastErr = err
			f.logError(fmt.Sprintf("Sync attempt %d failed", attempt), err)

			if attempt < f.config.MaxRetries {
				f.logInfo(fmt.Sprintf("Retrying in %v...", f.config.RetryDelay))
				time.Sleep(f.config.RetryDelay)
				continue
			}
		} else {
			if attempt > 1 {
				f.logInfo(fmt.Sprintf("Sync succeeded on attempt %d", attempt))
			}
			return nil
		}
	}

	return fmt.Errorf("sync failed after %d attempts: %w", f.config.MaxRetries, lastErr)
}

// syncData performs the actual data synchronization
func (f *FetcherService) syncData(ctx context.Context) error {
	f.logInfo("Starting data synchronization from HolonZero API")

	// First check API health
	if err := f.checkAPIHealth(ctx); err != nil {
		return fmt.Errorf("API health check failed: %w", err)
	}

	// Sync participants first
	if err := f.syncParticipants(ctx); err != nil {
		return fmt.Errorf("failed to sync participants: %w", err)
	}

	// Sync threads
	if err := f.syncThreads(ctx); err != nil {
		return fmt.Errorf("failed to sync threads: %w", err)
	}

	// Sync replies
	if err := f.syncReplies(ctx); err != nil {
		return fmt.Errorf("failed to sync replies: %w", err)
	}

	f.logInfo("Data synchronization completed successfully")
	return nil
}

// checkAPIHealth verifies the API is accessible
func (f *FetcherService) checkAPIHealth(ctx context.Context) error {
	health, err := f.client.GetHealth(ctx)
	if err != nil {
		return err
	}

	if health.Status != "ok" && health.Status != "healthy" {
		return fmt.Errorf("API is not healthy: %s", health.Status)
	}

	f.logInfo("API health check passed")
	return nil
}

// syncParticipants fetches and stores participants
func (f *FetcherService) syncParticipants(ctx context.Context) error {
	participants, err := f.client.ListParticipants(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch participants: %w", err)
	}

	f.logInfo(fmt.Sprintf("Fetched %d participants from API", len(participants)))

	for _, participant := range participants {
		// Create or update author in the database
		alias := participant.DisplayName
		if alias == "" {
			alias = participant.Name
		}

		_, err := f.repository.CreateOrUpdateAuthor(ctx, strconv.Itoa(participant.ID), alias)
		if err != nil {
			f.logError(fmt.Sprintf("Failed to create/update participant %d", participant.ID), err)
			continue
		}
	}

	f.logInfo(fmt.Sprintf("Successfully synced %d participants", len(participants)))
	return nil
}

// syncThreads fetches and stores threads
func (f *FetcherService) syncThreads(ctx context.Context) error {
	var allThreads []Thread
	page := 1

	// Fetch all threads with pagination
	for {
		threadsResp, err := f.client.ListThreadsPaginated(ctx, &ThreadsQuery{
			Page:  page,
			Limit: f.config.BatchSize,
		})
		if err != nil {
			return fmt.Errorf("failed to fetch threads page %d: %w", page, err)
		}

		allThreads = append(allThreads, threadsResp.Threads...)
		f.logInfo(fmt.Sprintf("Fetched page %d: %d threads", page, len(threadsResp.Threads)))

		if !threadsResp.HasMore || len(threadsResp.Threads) == 0 {
			break
		}
		page++
	}

	f.logInfo(fmt.Sprintf("Fetched %d total threads from API", len(allThreads)))

	// Store threads in database
	for _, thread := range allThreads {
		threadID := fmt.Sprintf("holon-%d", thread.ID)
		authorIdentity := strconv.Itoa(thread.CreatorID)

		// Convert time format
		var expiresAt *string = nil
		// Note: HolonZero threads don't have expiration in the current schema

		// Convert to local thread format
		imageURLs := []string{} // HolonZero doesn't have image URLs in current schema
		actions := []string{}   // HolonZero doesn't have actions in current schema

		_, err := f.repository.CreateThread(
			ctx,
			threadID,
			thread.Title,
			thread.Content,
			authorIdentity,
			imageURLs,
			actions,
			expiresAt,
		)
		if err != nil {
			// If thread already exists, that's okay - we could implement update logic here
			f.logInfo(fmt.Sprintf("Thread %s may already exist or failed to create: %v", threadID, err))
			continue
		}
	}

	f.logInfo(fmt.Sprintf("Successfully synced %d threads", len(allThreads)))
	return nil
}

// syncReplies fetches and stores replies for all threads
func (f *FetcherService) syncReplies(ctx context.Context) error {
	// First, get all threads to fetch replies for
	threads, err := f.client.ListThreads(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to fetch threads for reply sync: %w", err)
	}

	totalReplies := 0

	for _, thread := range threads {
		// Fetch replies for this thread with pagination
		var allReplies []Reply
		page := 1

		for {
			repliesResp, err := f.client.GetThreadRepliesPaginated(ctx, thread.ID, &RepliesQuery{
				Page:  page,
				Limit: f.config.BatchSize,
			})
			if err != nil {
				f.logError(fmt.Sprintf("Failed to fetch replies for thread %d, page %d", thread.ID, page), err)
				break
			}

			allReplies = append(allReplies, repliesResp.Replies...)

			if !repliesResp.HasMore || len(repliesResp.Replies) == 0 {
				break
			}
			page++
		}

		// Store replies in database
		for _, reply := range allReplies {
			threadID := fmt.Sprintf("holon-%d", reply.ThreadID)
			authorIdentity := strconv.Itoa(reply.ParticipantID)
			messageID := fmt.Sprintf("holon-reply-%d", reply.ID)

			actions := []string{} // HolonZero doesn't have actions in current schema
			isDelivered := true   // Assume fetched replies are delivered

			_, err := f.repository.CreateThreadMessage(
				ctx,
				messageID,
				threadID,
				authorIdentity,
				reply.Content,
				actions,
				&isDelivered,
			)
			if err != nil {
				// If reply already exists, that's okay
				f.logInfo(fmt.Sprintf("Reply %s may already exist or failed to create: %v", messageID, err))
				continue
			}
		}

		totalReplies += len(allReplies)
		if len(allReplies) > 0 {
			f.logInfo(fmt.Sprintf("Synced %d replies for thread %d", len(allReplies), thread.ID))
		}
	}

	f.logInfo(fmt.Sprintf("Successfully synced %d total replies", totalReplies))
	return nil
}

// SyncParticipants fetches and syncs participants for direct activity usage
func (f *FetcherService) SyncParticipants(ctx context.Context) ([]Participant, error) {
	participants, err := f.client.ListParticipants(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch participants: %w", err)
	}

	f.logInfo(fmt.Sprintf("Fetched %d participants from API", len(participants)))

	for _, participant := range participants {
		// Create or update author in the database
		alias := participant.DisplayName
		if alias == "" {
			alias = participant.Name
		}

		_, err := f.repository.CreateOrUpdateAuthor(ctx, strconv.Itoa(participant.ID), alias)
		if err != nil {
			f.logError(fmt.Sprintf("Failed to create/update participant %d", participant.ID), err)
			continue
		}
	}

	f.logInfo(fmt.Sprintf("Successfully synced %d participants", len(participants)))
	return participants, nil
}

// SyncThreads fetches and syncs threads for direct activity usage
func (f *FetcherService) SyncThreads(ctx context.Context) ([]Thread, error) {
	var allThreads []Thread
	page := 1

	// Fetch all threads with pagination
	for {
		threadsResp, err := f.client.ListThreadsPaginated(ctx, &ThreadsQuery{
			Page:  page,
			Limit: f.config.BatchSize,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to fetch threads page %d: %w", page, err)
		}

		allThreads = append(allThreads, threadsResp.Threads...)
		f.logInfo(fmt.Sprintf("Fetched page %d: %d threads", page, len(threadsResp.Threads)))

		if !threadsResp.HasMore || len(threadsResp.Threads) == 0 {
			break
		}
		page++
	}

	f.logInfo(fmt.Sprintf("Fetched %d total threads from API", len(allThreads)))

	// Store threads in database
	for _, thread := range allThreads {
		threadID := fmt.Sprintf("holon-%d", thread.ID)
		authorIdentity := strconv.Itoa(thread.CreatorID)

		// Convert time format
		var expiresAt *string = nil
		// Note: HolonZero threads don't have expiration in the current schema

		// Convert to local thread format
		imageURLs := []string{} // HolonZero doesn't have image URLs in current schema
		actions := []string{}   // HolonZero doesn't have actions in current schema

		_, err := f.repository.CreateThread(
			ctx,
			threadID,
			thread.Title,
			thread.Content,
			authorIdentity,
			imageURLs,
			actions,
			expiresAt,
		)
		if err != nil {
			// If thread already exists, that's okay - we could implement update logic here
			f.logInfo(fmt.Sprintf("Thread %s may already exist or failed to create: %v", threadID, err))
			continue
		}
	}

	f.logInfo(fmt.Sprintf("Successfully synced %d threads", len(allThreads)))
	return allThreads, nil
}

// SyncReplies fetches and syncs replies for direct activity usage
func (f *FetcherService) SyncReplies(ctx context.Context) ([]Reply, error) {
	// First, get all threads to fetch replies for
	threads, err := f.client.ListThreads(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch threads for reply sync: %w", err)
	}

	var allReplies []Reply
	totalReplies := 0

	for _, thread := range threads {
		// Fetch replies for this thread with pagination
		var threadReplies []Reply
		page := 1

		for {
			repliesResp, err := f.client.GetThreadRepliesPaginated(ctx, thread.ID, &RepliesQuery{
				Page:  page,
				Limit: f.config.BatchSize,
			})
			if err != nil {
				f.logError(fmt.Sprintf("Failed to fetch replies for thread %d, page %d", thread.ID, page), err)
				break
			}

			threadReplies = append(threadReplies, repliesResp.Replies...)
			allReplies = append(allReplies, repliesResp.Replies...)

			if !repliesResp.HasMore || len(repliesResp.Replies) == 0 {
				break
			}
			page++
		}

		// Store replies in database
		for _, reply := range threadReplies {
			threadID := fmt.Sprintf("holon-%d", reply.ThreadID)
			authorIdentity := strconv.Itoa(reply.ParticipantID)
			messageID := fmt.Sprintf("holon-reply-%d", reply.ID)

			actions := []string{} // HolonZero doesn't have actions in current schema
			isDelivered := true   // Assume fetched replies are delivered

			_, err := f.repository.CreateThreadMessage(
				ctx,
				messageID,
				threadID,
				authorIdentity,
				reply.Content,
				actions,
				&isDelivered,
			)
			if err != nil {
				// If reply already exists, that's okay
				f.logInfo(fmt.Sprintf("Reply %s may already exist or failed to create: %v", messageID, err))
				continue
			}
		}

		totalReplies += len(threadReplies)
		if len(threadReplies) > 0 {
			f.logInfo(fmt.Sprintf("Synced %d replies for thread %d", len(threadReplies), thread.ID))
		}
	}

	f.logInfo(fmt.Sprintf("Successfully synced %d total replies", totalReplies))
	return allReplies, nil
}

// SyncStatus represents the current status of the fetcher
type SyncStatus struct {
	Running    bool       `json:"running"`
	LastSync   *time.Time `json:"last_sync,omitempty"`
	NextSync   time.Time  `json:"next_sync"`
	SyncCount  int        `json:"sync_count"`
	ErrorCount int        `json:"error_count"`
	LastError  string     `json:"last_error,omitempty"`
	TotalItems int        `json:"total_items"`
}

// GetSyncStatus returns the current status of the fetcher service
func (f *FetcherService) GetSyncStatus(ctx context.Context) (*SyncStatus, error) {
	// If the fetcher is not running, return basic status
	if !f.running {
		return &SyncStatus{
			Running: false,
		}, nil
	}

	// Get last sync time and metadata
	metadata, err := f.client.GetSyncMetadata(ctx)
	if err != nil {
		return &SyncStatus{
			Running:   f.running,
			LastError: err.Error(),
		}, nil
	}

	// Calculate next sync time based on the current time and fetch interval
	nextSync := time.Now().Add(f.config.FetchInterval)

	return &SyncStatus{
		Running:    f.running,
		LastSync:   &metadata.ServerTime,
		NextSync:   nextSync,
		SyncCount:  metadata.TotalThreads,
		ErrorCount: 0,
		TotalItems: metadata.TotalThreads + metadata.TotalReplies,
	}, nil
}

// ForceSync triggers an immediate synchronization
func (f *FetcherService) ForceSync(ctx context.Context) error {
	f.logInfo("Manual sync requested")

	// If not running, return error
	if !f.running {
		return fmt.Errorf("fetcher service is not running")
	}

	// Perform sync directly
	if err := f.performSync(ctx); err != nil {
		f.logError("Manual sync failed", err)
		return fmt.Errorf("manual sync failed: %w", err)
	}

	f.logInfo("Manual sync completed successfully")
	return nil
}

// logInfo logs an informational message if logging is enabled
func (f *FetcherService) logInfo(msg string) {
	if f.config.EnableLogging && f.logger != nil {
		f.logger.Info(msg)
	}
}

// logError logs an error message if logging is enabled
func (f *FetcherService) logError(msg string, err error) {
	if f.config.EnableLogging && f.logger != nil {
		f.logger.Error(msg, "error", err)
	}
}

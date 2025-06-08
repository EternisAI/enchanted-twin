package holon

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
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
	CreateThread(ctx context.Context, req CreateThreadRequest) (*Thread, error)
}

// FetcherService handles syncing data from HolonZero API to local database
type FetcherService struct {
	client         HolonZeroClient
	repository     *Repository
	config         FetcherConfig
	logger         *clog.Logger
	stopChan       chan struct{}
	running        bool
	
	// Authentication info for deduplication
	participantID   *int
	isAuthenticated bool
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
		APIBaseURL:    getEnvOrDefault("HOLON_API_URL", "http://localhost:8080"),
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

	fetcher := &FetcherService{
		client:     client,
		repository: NewRepository(store.DB()),
		config:     config,
		logger:     logger,
		stopChan:   make(chan struct{}),
		running:    false,
	}
	
	// Attempt to authenticate and get participant ID for deduplication
	ctx := context.Background()
	if err := fetcher.authenticateForDeduplication(ctx, store); err != nil {
		if logger != nil {
			logger.Debug("Failed to authenticate for deduplication during initialization", "error", err)
		}
		// Don't fail service creation if authentication fails
	}
	
	return fetcher
}

// Start begins the periodic fetching process
func (f *FetcherService) Start(ctx context.Context) error {
	if f.running {
		return fmt.Errorf("fetcher service is already running")
	}

	f.running = true
	f.logDebug("Starting HolonZero API fetcher service")

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
			f.logDebug("Context cancelled, stopping fetcher service")
			f.running = false
			return ctx.Err()
		case <-f.stopChan:
			f.logDebug("Stop signal received, stopping fetcher service")
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
		f.logDebug("Stopping HolonZero API fetcher service")
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
				f.logDebug(fmt.Sprintf("Retrying in %v...", f.config.RetryDelay))
				time.Sleep(f.config.RetryDelay)
				continue
			}
		} else {
			if attempt > 1 {
				f.logDebug(fmt.Sprintf("Sync succeeded on attempt %d", attempt))
			}
			return nil
		}
	}

	return fmt.Errorf("sync failed after %d attempts: %w", f.config.MaxRetries, lastErr)
}

// syncData performs the actual data synchronization
func (f *FetcherService) syncData(ctx context.Context) error {
	f.logDebug("Starting data synchronization from HolonZero API")

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

	f.logDebug("Data synchronization completed successfully")
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

	f.logDebug("API health check passed")
	return nil
}

// syncParticipants fetches and stores participants
func (f *FetcherService) syncParticipants(ctx context.Context) error {
	participants, err := f.client.ListParticipants(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch participants: %w", err)
	}

	f.logDebug(fmt.Sprintf("Fetched %d participants from API", len(participants)))

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

	f.logDebug(fmt.Sprintf("Successfully synced %d participants", len(participants)))
	return nil
}

// syncThreads fetches and stores threads
func (f *FetcherService) syncThreads(ctx context.Context) error {
	_, err := f.SyncThreads(ctx)
	return err
}

// SyncThreads fetches and syncs threads for direct activity usage
func (f *FetcherService) SyncThreads(ctx context.Context) ([]Thread, error) {
	return f.syncThreadsInternal(ctx, true)
}

// syncThreadsInternal handles the core thread syncing logic
func (f *FetcherService) syncThreadsInternal(ctx context.Context, useMetadata bool) ([]Thread, error) {
	var metadata *SyncMetadataResponse
	var err error
	
	// Get sync metadata to check for updates (only if requested)
	if useMetadata {
		metadata, err = f.client.GetSyncMetadata(ctx)
		if err != nil {
			f.logError("Failed to get sync metadata", err)
			// Continue without metadata - fetch all threads
		}
	}

	// Fetch threads using paginated endpoint
	var allThreads []Thread
	page := 1
	pageSize := f.config.BatchSize

	for {
		query := &ThreadsQuery{
			Page:  page,
			Limit: pageSize,
		}

		// Use metadata to only fetch updated threads if available and requested
		if useMetadata && metadata != nil && !metadata.LastThreadUpdate.IsZero() {
			query.UpdatedAfter = metadata.LastThreadUpdate
		}

		threadsResp, err := f.client.ListThreadsPaginated(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch threads page %d: %w", page, err)
		}

		allThreads = append(allThreads, threadsResp.Threads...)
		f.logDebug(fmt.Sprintf("Fetched page %d: %d threads", page, len(threadsResp.Threads)))

		if !threadsResp.HasMore || len(threadsResp.Threads) == 0 {
			break
		}
		page++
	}

	f.logDebug(fmt.Sprintf("Fetched %d total threads from API", len(allThreads)))

	// Filter and store threads in database
	skippedCount := 0
	storedCount := 0
	
	for _, thread := range allThreads {
		// Skip threads that were originally created by this instance to avoid duplicates
		if f.shouldSkipThread(thread) {
			skippedCount++
			continue
		}
		
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
			"received",
		)
		if err != nil {
			// If thread already exists, that's okay - we could implement update logic here
			if !strings.Contains(err.Error(), "UNIQUE constraint failed") {
				f.logError(fmt.Sprintf("Failed to create/update thread %d", thread.ID), err)
			}
			continue
		}
		
		storedCount++
	}

	f.logDebug(fmt.Sprintf("Successfully synced %d threads (skipped %d duplicates)", storedCount, skippedCount))
	return allThreads, nil
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
				f.logDebug(fmt.Sprintf("Reply %s may already exist or failed to create: %v", messageID, err))
				continue
			}
		}

		totalReplies += len(allReplies)
		if len(allReplies) > 0 {
			f.logDebug(fmt.Sprintf("Synced %d replies for thread %d", len(allReplies), thread.ID))
		}
	}

	f.logDebug(fmt.Sprintf("Successfully synced %d total replies", totalReplies))
	return nil
}

// PushPendingThreads pushes all pending threads to the HolonZero API and updates their state
func (f *FetcherService) PushPendingThreads(ctx context.Context) error {
	f.logDebug("Starting to push pending threads to HolonZero API")

	// Get all pending threads from the repository
	pendingThreads, err := f.repository.GetPendingThreads(ctx)
	if err != nil {
		return fmt.Errorf("failed to get pending threads: %w", err)
	}

	if len(pendingThreads) == 0 {
		f.logDebug("No pending threads to push")
		return nil
	}

	f.logDebug(fmt.Sprintf("Found %d pending threads to push", len(pendingThreads)))

	// Push each pending thread to the API
	for _, thread := range pendingThreads {
		// Create the request payload for the HolonZero API
		createReq := CreateThreadRequest{
			Title:         thread.Title,
			Content:       thread.Content,
			DedupThreadID: thread.ID, // Use local thread ID as dedup ID
		}

		// Push thread to the HolonZero API
		apiThread, err := f.client.CreateThread(ctx, createReq)
		if err != nil {
			f.logError(fmt.Sprintf("Failed to push thread %s to API", thread.ID), err)
			// Continue with other threads even if one fails
			continue
		}

		// Update the thread state to 'broadcasted' on successful push
		err = f.repository.UpdateThreadState(ctx, thread.ID, "broadcasted")
		if err != nil {
			f.logError(fmt.Sprintf("Failed to update thread %s state to broadcasted", thread.ID), err)
			// Continue with other threads even if state update fails
			continue
		}

		f.logDebug(fmt.Sprintf("Successfully pushed thread %s (local) -> %d (API) and updated state to broadcasted",
			thread.ID, apiThread.ID))
	}

	f.logDebug(fmt.Sprintf("Completed pushing pending threads"))
	return nil
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
	f.logDebug("Manual sync requested")

	// If not running, return error
	if !f.running {
		return fmt.Errorf("fetcher service is not running")
	}

	// Perform sync directly
	if err := f.performSync(ctx); err != nil {
		f.logError("Manual sync failed", err)
		return fmt.Errorf("manual sync failed: %w", err)
	}

	f.logDebug("Manual sync completed successfully")
	return nil
}

// authenticateForDeduplication attempts to authenticate and store participant ID for deduplication
func (f *FetcherService) authenticateForDeduplication(ctx context.Context, store *db.Store) error {
	// Attempt to authenticate with HolonZero API
	authResp, err := AuthenticateWithHolonZero(ctx, f.config.APIBaseURL, store, f.logger)
	if err != nil {
		return fmt.Errorf("failed to authenticate with holon network: %w", err)
	}
	
	// Store authentication info for deduplication
	f.participantID = &authResp.ID
	f.isAuthenticated = true
	
	if f.logger != nil {
		f.logger.Debug("Authenticated for deduplication", 
			"participantID", authResp.ID, 
			"displayName", authResp.DisplayName)
	}
	
	return nil
}

// shouldSkipThread determines if a thread should be skipped during sync to avoid duplicates
func (f *FetcherService) shouldSkipThread(thread Thread) bool {
	// If we're not authenticated, we can't perform deduplication
	if !f.isAuthenticated || f.participantID == nil {
		return false
	}
	
	// Skip if this thread was created by us and has a dedup ID
	// (indicating it was originally created by this instance)
	if thread.CreatorID == *f.participantID && thread.DedupThreadID != "" {
		f.logDebug(fmt.Sprintf("Skipping thread %d (dedup ID: %s) - originally created by this instance", 
			thread.ID, thread.DedupThreadID))
		return true
	}
	
	return false
}

// SyncParticipants fetches and syncs participants for direct activity usage
func (f *FetcherService) SyncParticipants(ctx context.Context) ([]Participant, error) {
	participants, err := f.client.ListParticipants(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch participants: %w", err)
	}

	f.logDebug(fmt.Sprintf("Fetched %d participants from API", len(participants)))

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

	f.logDebug(fmt.Sprintf("Successfully synced %d participants", len(participants)))
	return participants, nil
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
				f.logDebug(fmt.Sprintf("Reply %s may already exist or failed to create: %v", messageID, err))
				continue
			}
		}

		totalReplies += len(threadReplies)
		if len(threadReplies) > 0 {
			f.logDebug(fmt.Sprintf("Synced %d replies for thread %d", len(threadReplies), thread.ID))
		}
	}

	f.logDebug(fmt.Sprintf("Successfully synced %d total replies", totalReplies))
	return allReplies, nil
}

// logDebug logs a debug message if logging is enabled
func (f *FetcherService) logDebug(msg string) {
	if f.config.EnableLogging && f.logger != nil {
		f.logger.Debug(msg)
	}
}

// logError logs an error message if logging is enabled
func (f *FetcherService) logError(msg string, err error) {
	if f.config.EnableLogging && f.logger != nil {
		f.logger.Error(msg, "error", err)
	}
}

// getEnvOrDefault returns the environment variable value or a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

package holon

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	clog "github.com/charmbracelet/log"

	"github.com/EternisAI/enchanted-twin/pkg/db"
)

// HolonZeroClient defines the interface for interacting with the HolonZero API.
type HolonZeroClient interface {
	GetHealth(ctx context.Context) (*HealthResponse, error)
	ListThreadsPaginated(ctx context.Context, query *ThreadsQuery) (*PaginatedThreadsResponse, error)
	GetThreadRepliesPaginated(ctx context.Context, threadID int, query *RepliesQuery) (*PaginatedRepliesResponse, error)
	GetSyncMetadata(ctx context.Context) (*SyncMetadataResponse, error)
	CreateThread(ctx context.Context, req CreateThreadRequest) (*Thread, error)
	CreateReply(ctx context.Context, req CreateReplyRequest) (*Reply, error)
}

// FetcherService handles syncing data from HolonZero API to local database.
type FetcherService struct {
	client     HolonZeroClient
	repository *Repository
	config     FetcherConfig
	logger     *clog.Logger
	stopChan   chan struct{}
	running    bool

	// Authentication info for deduplication
	participantID   *int
	isAuthenticated bool
}

// FetcherConfig holds configuration for the fetcher service.
type FetcherConfig struct {
	APIBaseURL    string
	FetchInterval time.Duration
	BatchSize     int
	MaxRetries    int
	RetryDelay    time.Duration
	EnableLogging bool
}

// NewFetcherService creates a new HolonZero API fetcher service.
func NewFetcherService(store *db.Store, config FetcherConfig, logger *clog.Logger) *FetcherService {
	ctx := context.Background()

	// Create authenticated API client with OAuth token
	client, err := NewAuthenticatedAPIClient(
		ctx,
		config.APIBaseURL,
		store,
		logger,
		WithTimeout(30*time.Second),
	)
	if err != nil {
		if logger != nil {
			logger.Error("Failed to create authenticated API client", "error", err)
		}
		// Fall back to basic client for health checks and read operations
		client = NewAPIClient(
			config.APIBaseURL,
			WithTimeout(30*time.Second),
			WithLogger(logger),
		)
	}

	fetcher := &FetcherService{
		client:     client,
		repository: NewRepository(store.DB()),
		config:     config,
		logger:     logger,
		stopChan:   make(chan struct{}),
		running:    false,
	}

	// Attempt to authenticate and get participant ID for deduplication
	if err := fetcher.authenticateForDeduplication(ctx, store); err != nil {
		if logger != nil {
			logger.Debug("Failed to authenticate for deduplication during initialization", "error", err)
		}
		// Don't fail service creation if authentication fails
	}

	return fetcher
}

// Start begins the periodic fetching process.
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
			f.logDebug("Context canceled, stopping fetcher service")
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

// Stop gracefully stops the fetching process.
func (f *FetcherService) Stop() {
	if f.running {
		f.logDebug("Stopping HolonZero API fetcher service")
		close(f.stopChan)
	}
}

// IsRunning returns whether the fetcher service is currently running.
func (f *FetcherService) IsRunning() bool {
	return f.running
}

// performSync executes a complete sync operation with retry logic.
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

// syncData performs the actual data synchronization.
func (f *FetcherService) syncData(ctx context.Context) error {
	f.logDebug("Starting data synchronization from HolonZero API")

	// First check API health
	if err := f.checkAPIHealth(ctx); err != nil {
		return fmt.Errorf("API health check failed: %w", err)
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

// checkAPIHealth verifies the API is accessible.
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

// syncThreads fetches and stores threads.
func (f *FetcherService) syncThreads(ctx context.Context) error {
	_, err := f.SyncThreads(ctx)
	return err
}

// SyncThreads fetches and syncs threads for direct activity usage.
func (f *FetcherService) SyncThreads(ctx context.Context) ([]Thread, error) {
	return f.syncThreadsInternal(ctx, true)
}

// syncThreadsInternal handles the core thread syncing logic.
func (f *FetcherService) syncThreadsInternal(ctx context.Context, useMetadata bool) ([]Thread, error) {
	var metadata *SyncMetadataResponse
	var err error

	// Check if we have any threads in database - if not, do a full sync
	threadCount, err := f.repository.GetThreadCount(ctx)
	if err != nil {
		f.logError("Failed to get thread count", err)
		threadCount = 0
	}

	// If we have no threads or very few, force a full sync regardless of metadata
	forceFullSync := threadCount < 5

	// Get sync metadata to check for updates (only if requested and not forcing full sync)
	if useMetadata && !forceFullSync {
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

		// Use metadata to only fetch updated threads if available, requested, and not forcing full sync
		if useMetadata && !forceFullSync && metadata != nil && !metadata.LastThreadUpdate.IsZero() {
			query.UpdatedAfter = metadata.LastThreadUpdate
			f.logDebug(fmt.Sprintf("Using incremental sync with updatedAfter: %v", metadata.LastThreadUpdate))
		} else if forceFullSync {
			f.logDebug("Performing full sync due to low thread count in database")
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

		// Validate thread creator data before processing
		if thread.CreatorID <= 0 {
			f.logError(fmt.Sprintf("Invalid CreatorID %d for thread %d, skipping", thread.CreatorID, thread.ID), nil)
			skippedCount++
			continue
		}

		// Create or update author from thread creator info
		authorIdentity, authorAlias, err := f.resolveAuthorIdentity(ctx, thread.CreatorID, thread.CreatorName)
		if err != nil {
			f.logError(fmt.Sprintf("Failed to resolve author identity for thread %d", thread.ID), err)
			skippedCount++
			continue
		}

		_, err = f.repository.CreateOrUpdateAuthor(ctx, authorIdentity, authorAlias)
		if err != nil {
			f.logError(fmt.Sprintf("Failed to create/update author %s", authorIdentity), err)
			// Continue even if author creation fails
			skippedCount++
			continue
		}

		threadID := fmt.Sprintf("holon-%d", thread.ID)

		// Convert time format
		var expiresAt *string = nil
		// Note: HolonZero threads don't have expiration in the current schema

		// Convert to local thread format
		imageURLs := thread.ImageURLs // Use image URLs from API response
		if imageURLs == nil {
			imageURLs = []string{} // Ensure we have an empty slice instead of nil
		}
		actions := []string{}   // HolonZero doesn't have actions in current schema

		_, err = f.repository.CreateThread(
			ctx,
			threadID,
			thread.Title,
			thread.Content,
			authorIdentity,
			imageURLs,
			actions,
			expiresAt,
			"received",
			thread.CreatedAt.Format(time.RFC3339), // Use server timestamp
		)
		if err != nil {
			// If thread already exists, that's okay - we could implement update logic here
			if !strings.Contains(err.Error(), "UNIQUE constraint failed") {
				f.logError(fmt.Sprintf("Failed to create/update thread %d", thread.ID), err)
			}
			continue
		}

		// Set the remote thread ID after creating the thread
		err = f.repository.UpdateThreadRemoteID(ctx, threadID, int32(thread.ID))
		if err != nil {
			// Check if this is a duplicate remote_thread_id error
			if strings.Contains(err.Error(), "UNIQUE constraint failed") && strings.Contains(err.Error(), "idx_threads_remote_id_unique") {
				f.logDebug(fmt.Sprintf("Thread with remote_thread_id %d already exists, skipping duplicate", thread.ID))
				continue
			}
			f.logError(fmt.Sprintf("Failed to update remote thread ID for thread %s", threadID), err)
			// Continue even if remote ID update fails
		}

		storedCount++
	}

	f.logDebug(fmt.Sprintf("Successfully synced %d threads (skipped %d duplicates)", storedCount, skippedCount))
	return allThreads, nil
}

// syncReplies fetches and stores replies for all threads.
func (f *FetcherService) syncReplies(ctx context.Context) error {
	// First, get all threads using paginated endpoint to fetch replies for
	var allThreads []Thread
	page := 1
	pageSize := f.config.BatchSize

	for {
		query := &ThreadsQuery{
			Page:  page,
			Limit: pageSize,
		}

		threadsResp, err := f.client.ListThreadsPaginated(ctx, query)
		if err != nil {
			return fmt.Errorf("failed to fetch threads for reply sync: %w", err)
		}

		allThreads = append(allThreads, threadsResp.Threads...)

		if !threadsResp.HasMore || len(threadsResp.Threads) == 0 {
			break
		}
		page++
	}

	totalReplies := 0

	for _, thread := range allThreads {
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
		skippedRepliesForThread := 0
		storedRepliesForThread := 0
		for _, reply := range allReplies {
			// Skip replies that were originally created by this instance to avoid duplicates
			if f.shouldSkipReply(reply) {
				skippedRepliesForThread++
				continue
			}

			// Validate reply data before processing
			if reply.ParticipantID <= 0 {
				f.logError(fmt.Sprintf("Invalid ParticipantID %d for reply %d, skipping", reply.ParticipantID, reply.ID), nil)
				skippedRepliesForThread++
				continue
			}

			// Create or update author from reply participant info
			authorIdentity, authorAlias, err := f.resolveAuthorIdentity(ctx, reply.ParticipantID, reply.ParticipantDisplayName)
			if err != nil {
				f.logError(fmt.Sprintf("Failed to resolve author identity for reply %d", reply.ID), err)
				continue
			}

			_, err = f.repository.CreateOrUpdateAuthor(ctx, authorIdentity, authorAlias)
			if err != nil {
				f.logError(fmt.Sprintf("Failed to create/update author %s", authorIdentity), err)
				// Continue even if author creation fails
				continue
			}

			threadID := fmt.Sprintf("holon-%d", reply.ThreadID)

			// Check if the thread exists locally before creating a reply
			// First check by remote_thread_id, then by local thread ID
			existingThread, err := f.repository.GetThreadByRemoteID(ctx, int32(reply.ThreadID))
			if err != nil || existingThread == nil {
				// If not found by remote ID, try by local thread ID
				existingThread, err = f.repository.GetThread(ctx, threadID)
				if err != nil || existingThread == nil {
					f.logDebug(fmt.Sprintf("Thread %s doesn't exist locally, creating it for reply %d", threadID, reply.ID))

					// Create the thread for this reply if it doesn't exist
					// We need to find the thread info from our allThreads list
					var parentThread *Thread
					for _, t := range allThreads {
						if t.ID == reply.ThreadID {
							parentThread = &t
							break
						}
					}

					if parentThread != nil {
						// Resolve author identity for the thread creator using the same logic
						threadAuthorIdentity, threadAuthorAlias, err := f.resolveAuthorIdentity(ctx, parentThread.CreatorID, parentThread.CreatorName)
						if err != nil {
							f.logError(fmt.Sprintf("Failed to resolve thread author identity for thread %d", parentThread.ID), err)
							continue
						}

						_, err = f.repository.CreateOrUpdateAuthor(ctx, threadAuthorIdentity, threadAuthorAlias)
						if err != nil {
							f.logError(fmt.Sprintf("Failed to create/update thread author %s", threadAuthorIdentity), err)
						} else {
							// Create the missing thread
							// Normalize image URLs
							imageURLs := parentThread.ImageURLs
							if imageURLs == nil {
								imageURLs = []string{}
							}
							_, err = f.repository.CreateThread(
								ctx,
								threadID,
								parentThread.Title,
								parentThread.Content,
								threadAuthorIdentity,
								imageURLs,               // Use normalized image URLs from parent thread
								[]string{}, // actions
								nil,        // expiresAt
								"received",
								parentThread.CreatedAt.Format(time.RFC3339), // Use server timestamp
							)
							if err != nil {
								f.logError(fmt.Sprintf("Failed to create missing thread %s for reply", threadID), err)
								continue // Skip this reply if we can't create the thread
							}

							// Set the remote thread ID
							err = f.repository.UpdateThreadRemoteID(ctx, threadID, int32(parentThread.ID))
							if err != nil {
								f.logError(fmt.Sprintf("Failed to update remote thread ID for created thread %s", threadID), err)
							}

							f.logDebug(fmt.Sprintf("Created missing thread %s for reply processing", threadID))
						}
					} else {
						f.logError(fmt.Sprintf("Could not find thread info for thread ID %d, skipping reply %d", reply.ThreadID, reply.ID), nil)
						continue
					}
				} else {
					// Found by local ID, use that thread ID
					threadID = existingThread.ID
				}
			} else {
				// Found by remote ID, use that thread ID
				threadID = existingThread.ID
			}

			messageID := fmt.Sprintf("holon-reply-%d", reply.ID)

			// Check if this reply already exists to avoid UNIQUE constraint failures
			existingMessage, err := f.repository.GetThreadMessage(ctx, messageID)
			if err != nil {
				f.logError(fmt.Sprintf("Failed to check if reply %s exists", messageID), err)
				continue
			}
			if existingMessage != nil {
				f.logDebug(fmt.Sprintf("Reply %s already exists, skipping", messageID))
				continue
			}

			actions := []string{} // HolonZero doesn't have actions in current schema
			isDelivered := true   // Assume fetched replies are delivered

			_, err = f.repository.CreateThreadMessageWithState(
				ctx,
				messageID,
				threadID,
				authorIdentity,
				reply.Content,
				actions,
				&isDelivered,
				"received",
				reply.CreatedAt.Format(time.RFC3339), // Use server timestamp
			)
			if err != nil {
				// If reply already exists, that's okay
				f.logDebug(fmt.Sprintf("Reply %s may already exist or failed to create: %v", messageID, err))
				continue
			}

			storedRepliesForThread++
		}

		totalReplies += len(allReplies)
		if len(allReplies) > 0 {
			f.logDebug(fmt.Sprintf("Synced %d replies for thread %d (stored %d new, skipped %d duplicates)", len(allReplies), thread.ID, storedRepliesForThread, skippedRepliesForThread))
		}
	}

	f.logDebug(fmt.Sprintf("Successfully synced %d total replies", totalReplies))
	return nil
}

// PushPendingThreads pushes all pending threads to the HolonZero API and updates their state.
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
			ImageURLs:     thread.ImageURLs, // Include image URLs from local thread
		}

		// Push thread to the HolonZero API
		apiThread, err := f.client.CreateThread(ctx, createReq)
		if err != nil {
			f.logError(fmt.Sprintf("Failed to push thread %s to API", thread.ID), err)
			// Continue with other threads even if one fails
			continue
		}

		// Update the remote thread ID after successful push
		err = f.repository.UpdateThreadRemoteID(ctx, thread.ID, int32(apiThread.ID))
		if err != nil {
			f.logError(fmt.Sprintf("Failed to update remote thread ID for thread %s", thread.ID), err)
			// Continue even if remote ID update fails
		}

		// Update the thread state to 'broadcasted' on successful push
		err = f.repository.UpdateThreadState(ctx, thread.ID, "broadcasted")
		if err != nil {
			f.logError(fmt.Sprintf("Failed to update thread %s state to broadcasted", thread.ID), err)
			// Continue with other threads even if state update fails
			continue
		}

		f.logDebug(fmt.Sprintf("Successfully pushed thread %s (local) -> %d (API), updated remote_id and state to broadcasted",
			thread.ID, apiThread.ID))
	}

	f.logDebug("Completed pushing pending threads")
	return nil
}

// PushPendingReplies pushes all pending thread messages (replies) to the HolonZero API and updates their state.
func (f *FetcherService) PushPendingReplies(ctx context.Context) error {
	// Create data fetcher to get pending replies with thread ID info
	dataFetcher := NewDataFetcher(f.repository)
	pendingReplies, err := dataFetcher.GetPendingReplies(ctx)
	if err != nil {
		return fmt.Errorf("failed to get pending thread messages: %w", err)
	}

	if len(pendingReplies) == 0 {
		f.logDebug("No pending replies to push")
		return nil
	}

	f.logDebug(fmt.Sprintf("Found %d pending replies to push", len(pendingReplies)))

	// Get participant ID if we don't have it yet
	participantID := f.participantID
	if participantID == nil {
		// Try to authenticate to get participant ID
		if err := f.authenticateForDeduplication(ctx, nil); err != nil {
			f.logError("Failed to authenticate for reply pushing", err)
			return fmt.Errorf("authentication required for pushing replies: %w", err)
		}
		participantID = f.participantID
	}

	if participantID == nil {
		return fmt.Errorf("participant ID required for pushing replies but not available")
	}

	for _, reply := range pendingReplies {
		// Find the corresponding thread to get the remote thread ID
		localThread, err := f.repository.GetThread(ctx, reply.ThreadID)
		if err != nil {
			f.logError(fmt.Sprintf("Failed to get thread %s for reply %s", reply.ThreadID, reply.ID), err)
			continue
		}

		if localThread == nil {
			f.logError(fmt.Sprintf("Thread %s not found for reply %s", reply.ThreadID, reply.ID), nil)
			continue
		}

		// Check if we have a remote thread ID - skip if we don't
		if localThread.RemoteThreadID == nil {
			f.logDebug(fmt.Sprintf("Skipping reply %s for thread %s - no remote thread ID available yet", reply.ID, localThread.ID))
			continue
		}

		remoteThreadID := *localThread.RemoteThreadID

		// Create the reply request
		createReq := CreateReplyRequest{
			ThreadID:      int(remoteThreadID),
			ParticipantID: *participantID,
			Content:       reply.Content,
			DedupReplyID:  reply.ID, // Use local ID as dedup ID
		}

		// Push reply to the HolonZero API
		apiReply, err := f.client.CreateReply(ctx, createReq)
		if err != nil {
			f.logError(fmt.Sprintf("Failed to push reply %s to API", reply.ID), err)
			// Continue with other replies even if one fails
			continue
		}

		// Update the reply state to 'broadcasted' on successful push
		err = f.repository.UpdateThreadMessageState(ctx, reply.ID, "broadcasted")
		if err != nil {
			f.logError(fmt.Sprintf("Failed to update reply %s state to broadcasted", reply.ID), err)
			// Continue with other replies even if state update fails
			continue
		}

		f.logDebug(fmt.Sprintf("Successfully pushed reply %s (local) -> %d (API) and updated state to broadcasted",
			reply.ID, apiReply.ID))
	}

	f.logDebug("Completed pushing pending replies")
	return nil
}

// PushPendingContent pushes both pending threads and replies to the HolonZero API.
func (f *FetcherService) PushPendingContent(ctx context.Context) error {
	f.logDebug("Starting to push all pending content to HolonZero API")

	// First push pending threads
	if err := f.PushPendingThreads(ctx); err != nil {
		f.logError("Failed to push pending threads", err)
		// Continue to try pushing replies even if threads fail
	}

	// Then push pending replies
	if err := f.PushPendingReplies(ctx); err != nil {
		f.logError("Failed to push pending replies", err)
		return fmt.Errorf("failed to push pending replies: %w", err)
	}

	f.logDebug("Completed pushing all pending content")
	return nil
}

// SyncStatus represents the current status of the fetcher.
type SyncStatus struct {
	Running    bool       `json:"running"`
	LastSync   *time.Time `json:"last_sync,omitempty"`
	NextSync   time.Time  `json:"next_sync"`
	SyncCount  int        `json:"sync_count"`
	ErrorCount int        `json:"error_count"`
	LastError  string     `json:"last_error,omitempty"`
	TotalItems int        `json:"total_items"`
}

// GetSyncStatus returns the current status of the fetcher service.
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

// ForceSync triggers an immediate synchronization.
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

// authenticateForDeduplication attempts to authenticate and store participant ID for deduplication.
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

// RefreshAuthentication re-authenticates with updated OAuth token.
func (f *FetcherService) RefreshAuthentication(ctx context.Context, store *db.Store) error {
	f.logDebug("Refreshing holon fetcher authentication with updated OAuth token")

	// Create new authenticated client with fresh token
	newClient, err := NewAuthenticatedAPIClient(
		ctx,
		f.config.APIBaseURL,
		store,
		f.logger,
		WithTimeout(30*time.Second),
	)
	if err != nil {
		f.logError("Failed to create new authenticated API client during refresh", err)
		return fmt.Errorf("failed to create authenticated client: %w", err)
	}

	// Update the client
	f.client = newClient

	// Re-authenticate for deduplication with new token
	if err := f.authenticateForDeduplication(ctx, store); err != nil {
		f.logError("Failed to re-authenticate for deduplication during refresh", err)
		return fmt.Errorf("failed to re-authenticate: %w", err)
	}

	f.logDebug("Successfully refreshed holon fetcher authentication")
	return nil
}

// shouldSkipThread determines if a thread should be skipped during sync to avoid duplicates.
func (f *FetcherService) shouldSkipThread(thread Thread) bool {
	// If we're not authenticated, we can't perform deduplication
	if !f.isAuthenticated || f.participantID == nil {
		return false
	}

	// Only skip if this thread was created by us AND we already have it in our database
	// AND the dedup ID matches an existing local thread
	if thread.CreatorID == *f.participantID && thread.DedupThreadID != "" {
		// Check if we already have a thread with this exact dedup ID in our local database
		ctx := context.Background()
		existingThread, err := f.repository.GetThread(ctx, thread.DedupThreadID)
		if err == nil && existingThread != nil {
			// Additional check: make sure this is actually a duplicate by comparing remote thread ID
			if existingThread.RemoteThreadID != nil && int32(thread.ID) == *existingThread.RemoteThreadID {
				f.logDebug(fmt.Sprintf("Skipping thread %d (dedup ID: %s) - already exists in local database with matching remote ID",
					thread.ID, thread.DedupThreadID))
				return true
			}
		}

		// If we get an error checking for the thread, log it but don't skip
		// This ensures we don't miss threads due to temporary database issues
		if err != nil {
			f.logDebug(fmt.Sprintf("Could not check if thread %s exists locally: %v, syncing anyway", thread.DedupThreadID, err))
		}
	}

	return false
}

// shouldSkipReply determines if a reply should be skipped during sync to avoid duplicates.
func (f *FetcherService) shouldSkipReply(reply Reply) bool {
	// If we're not authenticated, we can't perform deduplication
	if !f.isAuthenticated || f.participantID == nil {
		return false
	}

	// Only skip if this reply was created by us AND we already have it in our database
	// AND the dedup ID matches an existing local reply
	if reply.ParticipantID == *f.participantID && reply.DedupReplyID != "" {
		// Check if we already have a reply with this exact dedup ID in our local database
		ctx := context.Background()
		existingReply, err := f.repository.GetThreadMessage(ctx, reply.DedupReplyID)
		if err == nil && existingReply != nil {
			f.logDebug(fmt.Sprintf("Skipping reply %d (dedup ID: %s) - already exists in local database",
				reply.ID, reply.DedupReplyID))
			return true
		}

		// If we get an error checking for the reply, log it but don't skip
		// This ensures we don't miss replies due to temporary database issues
		if err != nil {
			f.logDebug(fmt.Sprintf("Could not check if reply %s exists locally: %v, syncing anyway", reply.DedupReplyID, err))
		}
	}

	return false
}

// SyncReplies fetches and syncs replies for direct activity usage.
func (f *FetcherService) SyncReplies(ctx context.Context) ([]Reply, error) {
	// First, get all threads using paginated endpoint to fetch replies for
	var allThreads []Thread
	page := 1
	pageSize := f.config.BatchSize

	for {
		query := &ThreadsQuery{
			Page:  page,
			Limit: pageSize,
		}

		threadsResp, err := f.client.ListThreadsPaginated(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch threads for reply sync: %w", err)
		}

		allThreads = append(allThreads, threadsResp.Threads...)

		if !threadsResp.HasMore || len(threadsResp.Threads) == 0 {
			break
		}
		page++
	}

	var allReplies []Reply
	totalReplies := 0

	for _, thread := range allThreads {
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
		skippedRepliesForThread := 0
		storedRepliesForThread := 0
		for _, reply := range threadReplies {
			// Skip replies that were originally created by this instance to avoid duplicates
			if f.shouldSkipReply(reply) {
				skippedRepliesForThread++
				continue
			}

			// Validate reply data before processing
			if reply.ParticipantID <= 0 {
				f.logError(fmt.Sprintf("Invalid ParticipantID %d for reply %d, skipping", reply.ParticipantID, reply.ID), nil)
				skippedRepliesForThread++
				continue
			}

			// Create or update author from reply participant info
			authorIdentity, authorAlias, err := f.resolveAuthorIdentity(ctx, reply.ParticipantID, reply.ParticipantDisplayName)
			if err != nil {
				f.logError(fmt.Sprintf("Failed to resolve author identity for reply %d", reply.ID), err)
				continue
			}

			_, err = f.repository.CreateOrUpdateAuthor(ctx, authorIdentity, authorAlias)
			if err != nil {
				f.logError(fmt.Sprintf("Failed to create/update author %s", authorIdentity), err)
				// Continue even if author creation fails
				continue
			}

			threadID := fmt.Sprintf("holon-%d", reply.ThreadID)

			// Check if the thread exists locally before creating a reply
			// First check by remote_thread_id, then by local thread ID
			existingThread, err := f.repository.GetThreadByRemoteID(ctx, int32(reply.ThreadID))
			if err != nil || existingThread == nil {
				// If not found by remote ID, try by local thread ID
				existingThread, err = f.repository.GetThread(ctx, threadID)
				if err != nil || existingThread == nil {
					f.logDebug(fmt.Sprintf("Thread %s doesn't exist locally, creating it for reply %d", threadID, reply.ID))

					// Create the thread for this reply if it doesn't exist
					// We need to find the thread info from our allThreads list
					var parentThread *Thread
					for _, t := range allThreads {
						if t.ID == reply.ThreadID {
							parentThread = &t
							break
						}
					}

					if parentThread != nil {
						// Resolve author identity for the thread creator using the same logic
						threadAuthorIdentity, threadAuthorAlias, err := f.resolveAuthorIdentity(ctx, parentThread.CreatorID, parentThread.CreatorName)
						if err != nil {
							f.logError(fmt.Sprintf("Failed to resolve thread author identity for thread %d", parentThread.ID), err)
							continue
						}

						_, err = f.repository.CreateOrUpdateAuthor(ctx, threadAuthorIdentity, threadAuthorAlias)
						if err != nil {
							f.logError(fmt.Sprintf("Failed to create/update thread author %s", threadAuthorIdentity), err)
						} else {
							// Create the missing thread
							// Normalize image URLs
							imageURLs := parentThread.ImageURLs
							if imageURLs == nil {
								imageURLs = []string{}
							}
							_, err = f.repository.CreateThread(
								ctx,
								threadID,
								parentThread.Title,
								parentThread.Content,
								threadAuthorIdentity,
								imageURLs,               // Use normalized image URLs from parent thread
								[]string{}, // actions
								nil,        // expiresAt
								"received",
								parentThread.CreatedAt.Format(time.RFC3339), // Use server timestamp
							)
							if err != nil {
								f.logError(fmt.Sprintf("Failed to create missing thread %s for reply", threadID), err)
								continue // Skip this reply if we can't create the thread
							}

							// Set the remote thread ID
							err = f.repository.UpdateThreadRemoteID(ctx, threadID, int32(parentThread.ID))
							if err != nil {
								f.logError(fmt.Sprintf("Failed to update remote thread ID for created thread %s", threadID), err)
							}

							f.logDebug(fmt.Sprintf("Created missing thread %s for reply processing", threadID))
						}
					} else {
						f.logError(fmt.Sprintf("Could not find thread info for thread ID %d, skipping reply %d", reply.ThreadID, reply.ID), nil)
						continue
					}
				} else {
					// Found by local ID, use that thread ID
					threadID = existingThread.ID
				}
			} else {
				// Found by remote ID, use that thread ID
				threadID = existingThread.ID
			}

			messageID := fmt.Sprintf("holon-reply-%d", reply.ID)

			// Check if this reply already exists to avoid UNIQUE constraint failures
			existingMessage, err := f.repository.GetThreadMessage(ctx, messageID)
			if err != nil {
				f.logError(fmt.Sprintf("Failed to check if reply %s exists", messageID), err)
				continue
			}
			if existingMessage != nil {
				f.logDebug(fmt.Sprintf("Reply %s already exists, skipping", messageID))
				continue
			}

			actions := []string{} // HolonZero doesn't have actions in current schema
			isDelivered := true   // Assume fetched replies are delivered

			_, err = f.repository.CreateThreadMessageWithState(
				ctx,
				messageID,
				threadID,
				authorIdentity,
				reply.Content,
				actions,
				&isDelivered,
				"received",
				reply.CreatedAt.Format(time.RFC3339), // Use server timestamp
			)
			if err != nil {
				// If reply already exists, that's okay
				f.logDebug(fmt.Sprintf("Reply %s may already exist or failed to create: %v", messageID, err))
				continue
			}

			storedRepliesForThread++
		}

		totalReplies += len(allReplies)
		if len(allReplies) > 0 {
			f.logDebug(fmt.Sprintf("Synced %d replies for thread %d (stored %d new, skipped %d duplicates)", len(allReplies), thread.ID, storedRepliesForThread, skippedRepliesForThread))
		}
	}

	f.logDebug(fmt.Sprintf("Successfully synced %d total replies", totalReplies))
	return allReplies, nil
}

// getLocalUserIdentity gets the local user identity from the user profile.
func (f *FetcherService) getLocalUserIdentity(ctx context.Context) (string, error) {
	// For now, we'll use a constant identity that represents the local user
	// In the future, this could be retrieved from user profile or config
	return "local-user", nil
}

// resolveAuthorIdentity determines the correct author identity to use
// If the remote creator/participant is our own participant ID, use local user identity
// Otherwise, use the remote participant ID.
func (f *FetcherService) resolveAuthorIdentity(ctx context.Context, remoteParticipantID int, remoteDisplayName string) (string, string, error) {
	// Check if this is our own content
	if f.isAuthenticated && f.participantID != nil && remoteParticipantID == *f.participantID {
		// This is our own content, use local user identity
		localIdentity, err := f.getLocalUserIdentity(ctx)
		if err != nil {
			return "", "", fmt.Errorf("failed to get local user identity: %w", err)
		}

		// Use the remote display name to update our local profile alias
		alias := remoteDisplayName
		if alias == "" {
			alias = "Me" // Default alias for local user
		}

		f.logDebug(fmt.Sprintf("Mapping remote participant %d to local user %s with alias %s",
			remoteParticipantID, localIdentity, alias))

		return localIdentity, alias, nil
	}

	// This is someone else's content, use remote participant ID
	authorIdentity := strconv.Itoa(remoteParticipantID)
	authorAlias := remoteDisplayName
	if authorAlias == "" {
		authorAlias = fmt.Sprintf("User %d", remoteParticipantID)
	}

	return authorIdentity, authorAlias, nil
}

// logDebug logs a debug message if logging is enabled.
func (f *FetcherService) logDebug(msg string) {
	if f.config.EnableLogging && f.logger != nil {
		f.logger.Debug(msg)
	}
}

// logError logs an error message if logging is enabled.
func (f *FetcherService) logError(msg string, err error) {
	if f.config.EnableLogging && f.logger != nil {
		f.logger.Error(msg, "error", err)
	}
}

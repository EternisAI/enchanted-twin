package holon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	clog "github.com/charmbracelet/log"
	"github.com/nats-io/nats.go"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/pkg/helpers"
)

// Manager coordinates the holon services including the API fetcher and Temporal integration.
type Manager struct {
	service         *Service
	fetcherService  *FetcherService
	syncActivities  *HolonSyncActivities
	store           *db.Store
	config          ManagerConfig
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	logger          *clog.Logger
	temporalClient  client.Client
	scheduleEnabled bool
	natsClient      *nats.Conn
	natsSub         *nats.Subscription
}

// ManagerConfig holds configuration for the holon manager.
type ManagerConfig struct {
	// HolonZero API configuration
	HolonAPIURL string

	// Fetcher configuration
	FetchInterval time.Duration
	BatchSize     int
	MaxRetries    int
	RetryDelay    time.Duration
	EnableLogging bool

	// Temporal configuration
	ScheduleID string
}

// DefaultManagerConfig returns a sensible default configuration.
func DefaultManagerConfig() ManagerConfig {
	// Use the getEnvOrDefault function from fetcher.go
	holonAPIURL := "http://localhost:8080"
	if value := os.Getenv("HOLON_API_URL"); value != "" {
		holonAPIURL = value
	}

	return ManagerConfig{
		HolonAPIURL:   holonAPIURL,
		FetchInterval: 30 * time.Second,
		BatchSize:     50,
		MaxRetries:    3,
		RetryDelay:    30 * time.Second,
		EnableLogging: true,
		ScheduleID:    "holon-sync-schedule",
	}
}

// NewManager creates a new holon manager with the given configuration.
func NewManager(store *db.Store, config ManagerConfig, logger *clog.Logger, temporalClient client.Client, worker worker.Worker) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	service := NewServiceWithLogger(store, logger)

	var fetcherService *FetcherService
	if config.HolonAPIURL != "" {
		fetcherConfig := FetcherConfig{
			APIBaseURL:    config.HolonAPIURL,
			FetchInterval: config.FetchInterval,
			BatchSize:     config.BatchSize,
			MaxRetries:    config.MaxRetries,
			RetryDelay:    config.RetryDelay,
			EnableLogging: config.EnableLogging,
		}
		fetcherService = NewFetcherService(store, fetcherConfig, logger)
	}

	// TODO: Re-enable NATS client after fixing circular dependency
	var natsClient *nats.Conn
	/*
	nc, err := bootstrap.NewNatsClient()
	if err != nil {
		logger.Error("Failed to create NATS client for holon manager", "error", err)
	} else {
		natsClient = nc
		logger.Debug("NATS client initialized for holon manager")
	}
	*/

	manager := &Manager{
		service:         service,
		fetcherService:  fetcherService,
		store:           store,
		config:          config,
		ctx:             ctx,
		cancel:          cancel,
		logger:          logger,
		temporalClient:  temporalClient,
		scheduleEnabled: temporalClient != nil,
		natsClient:      natsClient,
	}

	// Create sync activities if Temporal is enabled
	if manager.scheduleEnabled {
		manager.syncActivities = NewHolonSyncActivities(logger, manager)
	}

	return manager
}

// Start initializes and starts all holon services.
func (m *Manager) Start() error {
	m.logger.Debug("Starting Holon Manager...")

	// Setup NATS subscription for OAuth refresh events
	if err := m.setupNATSSubscription(); err != nil {
		m.logger.Error("Failed to setup NATS subscription", "error", err)
		// Don't fail completely, just log the error
	}

	// Setup Temporal schedule if enabled
	if m.scheduleEnabled {
		if err := m.setupTemporalSchedule(); err != nil {
			m.logger.Error("Failed to setup Temporal schedule", "error", err)
			// Don't fail completely, fall back to ticker-based sync
			m.scheduleEnabled = false
		} else {
			m.logger.Debug("Holon Temporal schedule configured successfully")
		}
	}

	// Start the fetcher service if enabled and Temporal schedule is not used
	if m.fetcherService != nil && !m.scheduleEnabled {
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			if err := m.fetcherService.Start(m.ctx); err != nil && err != context.Canceled {
				m.logger.Error("Fetcher service error", "error", err)
			}
		}()
		m.logger.Debug("HolonZero API fetcher service started (ticker-based)")
	} else if m.scheduleEnabled {
		m.logger.Debug("HolonZero API sync using Temporal schedule")
	} else {
		m.logger.Debug("HolonZero API fetcher service disabled or not configured")
	}

	// Set up graceful shutdown
	go m.handleShutdown()

	m.logger.Debug("Holon Manager started successfully")
	return nil
}

// setupTemporalSchedule creates a Temporal schedule for holon sync.
func (m *Manager) setupTemporalSchedule() error {
	if m.temporalClient == nil {
		return fmt.Errorf("temporal client is not available")
	}

	return helpers.CreateOrUpdateSchedule(
		m.logger,
		m.temporalClient,
		m.config.ScheduleID,
		m.config.FetchInterval,
		HolonSyncWorkflow,
		[]any{HolonSyncWorkflowInput{ForceSync: false}},
		true, // Override if different settings
	)
}

// setupNATSSubscription sets up the NATS subscription for OAuth token refresh events.
func (m *Manager) setupNATSSubscription() error {
	if m.natsClient == nil {
		return fmt.Errorf("nats client is not available")
	}

	// Subscribe to Google OAuth refresh events using the generic subject pattern
	sub, err := m.natsClient.Subscribe("oauth.google.token.refreshed", func(msg *nats.Msg) {
		var refreshEvent map[string]interface{}
		if err := json.Unmarshal(msg.Data, &refreshEvent); err != nil {
			m.logger.Error("Failed to unmarshal OAuth refresh event", "error", err)
			return
		}

		m.logger.Debug("Received OAuth token refresh event", "event", refreshEvent)

		// Extract token information from the event
		provider, _ := refreshEvent["provider"].(string)
		if provider != "google" {
			m.logger.Debug("Ignoring non-Google OAuth refresh event", "provider", provider)
			return
		}

		// Use the token directly from the NATS message instead of fetching from DB
		accessToken, _ := refreshEvent["access_token"].(string)
		tokenType, _ := refreshEvent["token_type"].(string)
		username, _ := refreshEvent["username"].(string)
		expiresAtStr, _ := refreshEvent["expires_at"].(string)

		if accessToken == "" {
			m.logger.Error("Received OAuth refresh event without access token")
			return
		}

		m.logger.Info("Processing OAuth token refresh for holon clients",
			"provider", provider,
			"username", username,
			"expires_at", expiresAtStr)

		// Refresh holon clients with the new token information
		ctx := context.Background()
		if err := m.refreshHolonClientsWithToken(ctx, accessToken, tokenType); err != nil {
			m.logger.Error("Failed to refresh holon clients with new token", "error", err)
		} else {
			m.logger.Info("Successfully refreshed holon clients with new OAuth token")
		}
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to OAuth refresh events: %w", err)
	}

	m.natsSub = sub
	m.logger.Debug("Successfully subscribed to OAuth token refresh events")
	return nil
}

// Stop gracefully shuts down the holon manager.
func (m *Manager) Stop() error {
	m.logger.Debug("Stopping Holon Manager...")

	// Cancel context to stop all services
	m.cancel()

	// Unsubscribe from NATS subject if subscribed
	if m.natsSub != nil {
		m.logger.Debug("Unsubscribing from NATS subject")
		if err := m.natsSub.Unsubscribe(); err != nil {
			m.logger.Error("Failed to unsubscribe from NATS subject", "error", err)
		} else {
			m.logger.Debug("Unsubscribed from NATS subject successfully")
		}
	}

	// Wait for all goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		m.logger.Debug("All holon services stopped gracefully")
	case <-time.After(30 * time.Second):
		m.logger.Warn("Timeout waiting for holon services to stop")
	}

	return nil
}

// handleShutdown sets up graceful shutdown handling.
func (m *Manager) handleShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		m.logger.Debug("Received shutdown signal", "signal", sig)
		if err := m.Stop(); err != nil {
			m.logger.Error("Failed to stop manager gracefully", "error", err)
		}
	case <-m.ctx.Done():
		// Context was canceled
		return
	}
}

// SyncStatus represents the current status of holon synchronization.
type ManagerSyncStatus struct {
	TotalHolons     int             `json:"total_holons"`
	LastSyncTime    *time.Time      `json:"last_sync_time"`
	ScheduleEnabled bool            `json:"schedule_enabled"`
	FetcherEnabled  bool            `json:"fetcher_enabled"`
	SyncInterval    time.Duration   `json:"sync_interval"`
	ScheduleStatus  *ScheduleStatus `json:"schedule_status,omitempty"`
}

// ScheduleStatus represents the status of the Temporal schedule.
type ScheduleStatus struct {
	Paused  bool                          `json:"paused"`
	LastRun []client.ScheduleActionResult `json:"last_run,omitempty"`
	NextRun []time.Time                   `json:"next_run,omitempty"`
}

// refreshHolonClientsWithToken refreshes holon clients using the provided token.
func (m *Manager) refreshHolonClientsWithToken(ctx context.Context, accessToken, tokenType string) error {
	var errors []error

	// Create new authenticated client with the provided token
	if m.fetcherService != nil {
		// Create new client with the fresh token
		newClient := NewAPIClient(
			m.config.HolonAPIURL,
			WithAuthToken(accessToken),
			WithTimeout(30*time.Second),
			WithLogger(m.logger),
		)

		// Update the fetcher service client
		m.fetcherService.client = newClient

		// Re-authenticate for deduplication with new token
		if err := m.fetcherService.authenticateForDeduplication(ctx, m.store); err != nil {
			m.logger.Error("Failed to re-authenticate fetcher service for deduplication", "error", err)
			errors = append(errors, fmt.Errorf("fetcher service deduplication: %w", err))
		} else {
			m.logger.Debug("Successfully refreshed fetcher service authentication")
		}
	}

	// Refresh holon service authentication (it will fetch fresh token from DB)
	if m.service != nil {
		if err := m.service.RefreshAuthentication(ctx); err != nil {
			m.logger.Error("Failed to refresh holon service authentication", "error", err)
			errors = append(errors, fmt.Errorf("holon service: %w", err))
		} else {
			m.logger.Debug("Successfully refreshed holon service authentication")
		}
	}

	// If any errors occurred, return them
	if len(errors) > 0 {
		return fmt.Errorf("failed to refresh some holon clients: %v", errors)
	}

	return nil
}

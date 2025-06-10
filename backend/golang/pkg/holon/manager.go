package holon

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	clog "github.com/charmbracelet/log"
	"go.temporal.io/sdk/client"
	temporal "go.temporal.io/sdk/client"
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

// TriggerSync manually triggers a holon sync workflow (replaces both TriggerSyncWorkflow and TriggerSync).
func (m *Manager) TriggerSync(forceSync bool) error {
	if !m.scheduleEnabled || m.temporalClient == nil {
		// Fall back to direct fetcher service if no Temporal
		if m.fetcherService == nil {
			return fmt.Errorf("neither temporal nor fetcher service is available")
		}
		return m.fetcherService.ForceSync(context.Background())
	}

	ctx := context.Background()
	workflowOptions := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("holon-manual-sync-%d", time.Now().Unix()),
		TaskQueue: "default",
	}

	input := HolonSyncWorkflowInput{ForceSync: forceSync}

	execution, err := m.temporalClient.ExecuteWorkflow(ctx, workflowOptions, HolonSyncWorkflow, input)
	if err != nil {
		return fmt.Errorf("failed to start holon sync workflow: %w", err)
	}

	m.logger.Debug("Manual holon sync workflow triggered", "workflowID", execution.GetID())
	return nil
}

// PauseSyncSchedule pauses the holon sync schedule if it exists.
func (m *Manager) PauseSyncSchedule() error {
	if m.temporalClient == nil || m.config.ScheduleID == "" {
		return fmt.Errorf("temporal client or schedule ID not configured")
	}

	m.logger.Debug("Pausing holon sync schedule", "scheduleID", m.config.ScheduleID)

	handle := m.temporalClient.ScheduleClient().GetHandle(context.Background(), m.config.ScheduleID)
	err := handle.Pause(context.Background(), temporal.SchedulePauseOptions{})
	if err != nil {
		return fmt.Errorf("failed to pause schedule: %w", err)
	}

	return nil
}

// ResumeSyncSchedule resumes the holon sync schedule if it exists.
func (m *Manager) ResumeSyncSchedule() error {
	if m.temporalClient == nil || m.config.ScheduleID == "" {
		return fmt.Errorf("temporal client or schedule ID not configured")
	}

	m.logger.Debug("Resuming holon sync schedule", "scheduleID", m.config.ScheduleID)

	handle := m.temporalClient.ScheduleClient().GetHandle(context.Background(), m.config.ScheduleID)
	err := handle.Unpause(context.Background(), temporal.ScheduleUnpauseOptions{})
	if err != nil {
		return fmt.Errorf("failed to resume schedule: %w", err)
	}

	return nil
}

// UpdateSyncInterval updates the sync schedule interval.
func (m *Manager) UpdateSyncInterval(interval time.Duration) error {
	if !m.scheduleEnabled || m.temporalClient == nil {
		return fmt.Errorf("temporal schedule is not enabled")
	}

	ctx := context.Background()
	scheduleHandle := m.temporalClient.ScheduleClient().GetHandle(ctx, m.config.ScheduleID)

	return scheduleHandle.Update(ctx, client.ScheduleUpdateOptions{
		DoUpdate: func(input client.ScheduleUpdateInput) (*client.ScheduleUpdate, error) {
			schedule := input.Description.Schedule
			schedule.Spec = &client.ScheduleSpec{
				Intervals: []client.ScheduleIntervalSpec{
					{
						Every: interval,
					},
				},
			}
			return &client.ScheduleUpdate{
				Schedule: &schedule,
			}, nil
		},
	})
}

// RecreateHolonSyncSchedule deletes and recreates the holon sync schedule with updated settings.
func (m *Manager) RecreateHolonSyncSchedule() error {
	if !m.scheduleEnabled || m.temporalClient == nil {
		return fmt.Errorf("temporal schedule is not enabled")
	}

	ctx := context.Background()
	scheduleHandle := m.temporalClient.ScheduleClient().GetHandle(ctx, m.config.ScheduleID)

	// Check if schedule exists
	_, err := scheduleHandle.Describe(ctx)
	if err == nil {
		// Schedule exists, delete it first
		m.logger.Debug("Deleting existing holon sync schedule", "scheduleID", m.config.ScheduleID)
		err = scheduleHandle.Delete(ctx)
		if err != nil {
			return fmt.Errorf("failed to delete existing schedule: %w", err)
		}
		m.logger.Debug("Existing schedule deleted successfully")
	}

	// Create new schedule with updated settings
	m.logger.Debug("Creating new holon sync schedule with updated settings",
		"scheduleID", m.config.ScheduleID,
		"interval", m.config.FetchInterval)

	return m.setupTemporalSchedule()
}

// GetSyncStatus returns the current sync status and statistics.
func (m *Manager) GetSyncStatus() (*ManagerSyncStatus, error) {
	// Get basic statistics
	totalHolons, err := m.service.GetTotalHolonCount()
	if err != nil {
		return nil, fmt.Errorf("failed to get total holon count: %w", err)
	}

	lastSyncTime, err := m.service.GetLastSyncTime()
	if err != nil {
		return nil, fmt.Errorf("failed to get last sync time: %w", err)
	}

	status := &ManagerSyncStatus{
		TotalHolons:     totalHolons,
		LastSyncTime:    lastSyncTime,
		ScheduleEnabled: m.scheduleEnabled,
		FetcherEnabled:  m.fetcherService != nil && !m.scheduleEnabled,
		SyncInterval:    m.config.FetchInterval,
	}

	// Get Temporal schedule status if available
	if m.scheduleEnabled && m.temporalClient != nil {
		ctx := context.Background()
		scheduleHandle := m.temporalClient.ScheduleClient().GetHandle(ctx, m.config.ScheduleID)

		desc, err := scheduleHandle.Describe(ctx)
		if err == nil {
			status.ScheduleStatus = &ScheduleStatus{
				Paused:  desc.Schedule.State.Paused,
				LastRun: desc.Info.RecentActions,
				NextRun: desc.Info.NextActionTimes,
			}
		}
	}

	return status, nil
}

// Stop gracefully shuts down the holon manager.
func (m *Manager) Stop() error {
	m.logger.Debug("Stopping Holon Manager...")

	// Cancel context to stop all services
	m.cancel()

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
		m.Stop()
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

// StartHolonServices demonstrates how to integrate the holon services into your main application.
func StartHolonServices(store *db.Store) (*Manager, error) {
	// Load configuration from environment or use defaults
	config := DefaultManagerConfig()

	// Create a logger
	logger := clog.NewWithOptions(os.Stdout, clog.Options{
		Level:           clog.InfoLevel,
		ReportTimestamp: true,
	})

	// Create and start the manager (without Temporal integration for this example)
	manager := NewManager(store, config, logger, nil, nil)
	if err := manager.Start(); err != nil {
		return nil, fmt.Errorf("failed to start holon services: %w", err)
	}

	return manager, nil
}

// HTTPHandlers provides HTTP endpoints for managing the holon fetcher.
type HTTPHandlers struct {
	manager *Manager
}

// NewHTTPHandlers creates HTTP handlers for holon management.
func NewHTTPHandlers(manager *Manager) *HTTPHandlers {
	return &HTTPHandlers{manager: manager}
}

// GetStatus returns the current fetcher status as JSON.
func (h *HTTPHandlers) GetStatus() (interface{}, error) {
	if h.manager.fetcherService == nil {
		return map[string]interface{}{
			"enabled": false,
			"message": "HolonZero fetcher is disabled",
		}, nil
	}

	status, err := h.manager.GetSyncStatus()
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"enabled": true,
		"status":  status,
	}, nil
}

// TriggerSync forces an immediate synchronization.
func (h *HTTPHandlers) TriggerSync() error {
	return h.manager.TriggerSync(true)
}

// Configuration returns the current fetcher configuration.
func (h *HTTPHandlers) GetConfiguration() map[string]interface{} {
	config := h.manager.config

	return map[string]interface{}{
		"holon_api_url":   config.HolonAPIURL,
		"fetcher_enabled": true,
		"fetch_interval":  config.FetchInterval.String(),
		"batch_size":      config.BatchSize,
		"max_retries":     config.MaxRetries,
		"retry_delay":     config.RetryDelay.String(),
		"logging_enabled": config.EnableLogging,
	}
}

// GetFetcherService returns the fetcher service.
func (m *Manager) GetFetcherService() *FetcherService {
	return m.fetcherService
}

// GetFetcherStatus returns the status of the fetcher service.
func (m *Manager) GetFetcherStatus() (*SyncStatus, error) {
	if m.fetcherService == nil {
		return nil, fmt.Errorf("fetcher service is not initialized")
	}
	return m.fetcherService.GetSyncStatus(context.Background())
}

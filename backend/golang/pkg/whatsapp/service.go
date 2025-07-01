package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/nats-io/nats.go"
	"github.com/samber/lo"
	"go.mau.fi/whatsmeow"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/config"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	waTools "github.com/EternisAI/enchanted-twin/pkg/whatsapp/tools"
)

// ServiceQREvent represents a QR code event within the service.
type ServiceQREvent struct {
	Event string
	Code  string
}

// ServiceSyncStatus represents sync status within the service.
type ServiceSyncStatus struct {
	Progress          float64
	TotalItems        int
	ProcessedItems    int
	StartTime         time.Time
	LastUpdateTime    time.Time
	StatusMessage     string
	EstimatedTimeLeft string
	IsSyncing         bool
	IsCompleted       bool
}

// ServiceContact represents a WhatsApp contact within the service.
type ServiceContact struct {
	Jid  string
	Name string
}

type Service struct {
	logger        *log.Logger
	nc            *nats.Conn
	dbsqlc        *db.DB
	memoryStorage memory.Storage
	envs          *config.Config
	aiService     *ai.Service
	toolRegistry  *tools.ToolMapRegistry
	client        *whatsmeow.Client
	currentQRCode *string
	isConnected   bool

	// Internal state management (no global state)
	qrChan        chan ServiceQREvent
	connectChan   chan struct{}
	clientChan    chan *whatsmeow.Client
	latestQREvent *ServiceQREvent
	syncStatus    ServiceSyncStatus
	contacts      []ServiceContact

	// Lifecycle management
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	mu      sync.RWMutex
	started bool
}

type ServiceConfig struct {
	Logger        *log.Logger
	NatsClient    *nats.Conn
	Database      *db.DB
	MemoryStorage memory.Storage
	Config        *config.Config
	AIService     *ai.Service
	ToolRegistry  *tools.ToolMapRegistry
}

func NewService(cfg ServiceConfig) *Service {
	ctx, cancel := context.WithCancel(context.Background())

	service := &Service{
		logger:        cfg.Logger,
		nc:            cfg.NatsClient,
		dbsqlc:        cfg.Database,
		memoryStorage: cfg.MemoryStorage,
		envs:          cfg.Config,
		aiService:     cfg.AIService,
		toolRegistry:  cfg.ToolRegistry,

		// Initialize internal channels and state
		qrChan:      make(chan ServiceQREvent, 100),
		connectChan: make(chan struct{}, 1),
		clientChan:  make(chan *whatsmeow.Client, 1),
		contacts:    make([]ServiceContact, 0),

		ctx:    ctx,
		cancel: cancel,
	}

	return service
}

func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return nil
	}

	s.logger.Info("Starting WhatsApp service...")

	s.wg.Add(4)
	go s.safeGoroutine("handleQRCodeEvents", s.handleQRCodeEvents)
	go s.safeGoroutine("bootstrapClient", s.bootstrapClient)
	go s.safeGoroutine("triggerAutoConnect", s.triggerAutoConnect)
	go s.safeGoroutine("registerToolsWhenReady", s.registerToolsWhenReady)

	s.started = true
	s.logger.Info("WhatsApp service started successfully")
	return nil
}

func (s *Service) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return nil
	}

	s.logger.Info("Stopping WhatsApp service...")

	s.cancel()

	if s.client != nil && s.client.IsConnected() {
		s.client.Disconnect()
	}

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("All WhatsApp service goroutines stopped")
	case <-time.After(5 * time.Second):
		s.logger.Warn("Timeout waiting for WhatsApp service goroutines to stop")
	}

	s.started = false
	s.logger.Info("WhatsApp service stopped")
	return nil
}

func (s *Service) safeGoroutine(name string, fn func()) {
	defer s.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("Panic in WhatsApp service goroutine", "goroutine", name, "panic", r)
		}
	}()
	fn()
}

// Public API methods with thread safety.
func (s *Service) GetCurrentQRCode() *string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentQRCode
}

func (s *Service) IsConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isConnected
}

func (s *Service) GetClient() *whatsmeow.Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.client
}

func (s *Service) GetSyncStatus() ServiceSyncStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.syncStatus
}

// QR Code management methods.
func (s *Service) GetQRChannel() chan ServiceQREvent {
	return s.qrChan
}

func (s *Service) PublishQREvent(event ServiceQREvent) {
	select {
	case s.qrChan <- event:
	case <-s.ctx.Done():
	default:
		s.logger.Warn("QR channel full, dropping event", "event", event.Event)
	}
}

func (s *Service) SetLatestQREvent(evt ServiceQREvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.latestQREvent = &evt
}

func (s *Service) GetLatestQREvent() *ServiceQREvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.latestQREvent
}

// Sync status management methods.
func (s *Service) StartSync() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.syncStatus = ServiceSyncStatus{
		IsSyncing:         true,
		Progress:          0,
		ProcessedItems:    0,
		TotalItems:        0,
		StartTime:         time.Now(),
		LastUpdateTime:    time.Now(),
		StatusMessage:     "Preparing to sync WhatsApp history...",
		EstimatedTimeLeft: "Calculating...",
	}
}

func (s *Service) UpdateSyncStatus(newStatus ServiceSyncStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()

	newStatus.LastUpdateTime = time.Now()

	if newStatus.TotalItems > 0 {
		newStatus.Progress = float64(newStatus.ProcessedItems) / float64(newStatus.TotalItems) * 100

		if newStatus.ProcessedItems > 0 && !newStatus.StartTime.IsZero() {
			elapsedTime := time.Since(newStatus.StartTime)
			itemsPerSecond := float64(newStatus.ProcessedItems) / elapsedTime.Seconds()
			if itemsPerSecond > 0 {
				remainingItems := newStatus.TotalItems - newStatus.ProcessedItems
				remainingSeconds := float64(remainingItems) / itemsPerSecond
				remainingDuration := time.Duration(remainingSeconds) * time.Second

				if remainingDuration > 1*time.Hour {
					newStatus.EstimatedTimeLeft = fmt.Sprintf("~%.1f hours", remainingDuration.Hours())
				} else if remainingDuration > 1*time.Minute {
					newStatus.EstimatedTimeLeft = fmt.Sprintf("~%.1f minutes", remainingDuration.Minutes())
				} else {
					newStatus.EstimatedTimeLeft = fmt.Sprintf("~%.0f seconds", remainingDuration.Seconds())
				}
			}
		}
	}

	if newStatus.IsSyncing && newStatus.ProcessedItems == 0 && newStatus.TotalItems > 0 && newStatus.StartTime.IsZero() {
		newStatus.StartTime = time.Now()
	}

	if newStatus.ProcessedItems >= newStatus.TotalItems && newStatus.TotalItems > 0 {
		newStatus.IsSyncing = false
		newStatus.IsCompleted = true
		newStatus.Progress = 100
		newStatus.EstimatedTimeLeft = "Complete"
	}

	s.syncStatus = newStatus
}

func (s *Service) PublishSyncStatus() error {
	status := s.GetSyncStatus()

	type syncStatusPublish struct {
		IsSyncing     bool    `json:"isSyncing"`
		IsCompleted   bool    `json:"isCompleted"`
		StatusMessage string  `json:"statusMessage"`
		Error         *string `json:"error"`
	}

	publishData := syncStatusPublish{
		IsSyncing:     status.IsSyncing,
		IsCompleted:   status.IsCompleted,
		StatusMessage: status.StatusMessage,
		Error:         nil,
	}

	data, err := json.Marshal(publishData)
	if err != nil {
		s.logger.Error("Failed to marshal WhatsApp sync status", "error", err)
		return err
	}

	err = s.nc.Publish("whatsapp.sync.status", data)
	if err != nil {
		s.logger.Error("Failed to publish WhatsApp sync status", "error", err)
		return err
	}

	s.logger.Debug("Published WhatsApp sync status",
		"syncing", status.IsSyncing,
		"completed", status.IsCompleted,
		"message", status.StatusMessage)

	return nil
}

// Contact management methods.
func (s *Service) AddContact(jid, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	exists := lo.ContainsBy(s.contacts, func(c ServiceContact) bool {
		return c.Jid == jid
	})

	if !exists {
		s.contacts = append(s.contacts, ServiceContact{
			Jid:  jid,
			Name: name,
		})
	}
}

func (s *Service) FindContactByJID(jid string) (ServiceContact, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	contact, found := lo.Find(s.contacts, func(c ServiceContact) bool {
		return c.Jid == jid
	})

	if found {
		return contact, true
	}

	// Try normalized JID lookup
	normJID := s.normalizeJID(jid)
	return lo.Find(s.contacts, func(c ServiceContact) bool {
		return s.normalizeJID(c.Jid) == normJID
	})
}

func (s *Service) normalizeJID(jid string) string {
	if idx := strings.Index(jid, "@"); idx > 0 {
		return jid[:idx]
	}
	return jid
}

// Internal goroutine methods.
func (s *Service) handleQRCodeEvents() {
	for {
		select {
		case <-s.ctx.Done():
			s.logger.Debug("QR code event handler stopping")
			return
		case evt := <-s.qrChan:
			switch evt.Event {
			case "code":
				s.handleQRCodeReceived(evt.Code)
			case "success":
				s.handleConnectionSuccess()
			}
		}
	}
}

func (s *Service) handleQRCodeReceived(qrCode string) {
	s.mu.Lock()
	s.currentQRCode = &qrCode
	s.isConnected = false
	s.mu.Unlock()

	s.logger.Info("Received new WhatsApp QR code", "length", len(qrCode))

	qrCodeUpdate := map[string]interface{}{
		"event":        "code",
		"qr_code_data": qrCode,
		"is_connected": false,
		"timestamp":    time.Now().Format(time.RFC3339),
	}

	s.publishToNATS("whatsapp.qr_code", qrCodeUpdate)
}

func (s *Service) handleConnectionSuccess() {
	s.mu.Lock()
	s.isConnected = true
	s.currentQRCode = nil
	s.mu.Unlock()

	s.logger.Info("WhatsApp connection successful")

	s.StartSync()
	s.UpdateSyncStatus(ServiceSyncStatus{
		IsSyncing:      true,
		IsCompleted:    false,
		ProcessedItems: 0,
		TotalItems:     0,
		StatusMessage:  "Waiting for history sync to begin",
	})

	if err := s.PublishSyncStatus(); err != nil {
		s.logger.Error("Failed to publish sync status", "error", err)
	}

	successUpdate := map[string]interface{}{
		"event":        "success",
		"qr_code_data": nil,
		"is_connected": true,
		"timestamp":    time.Now().Format(time.RFC3339),
	}

	s.publishToNATS("whatsapp.qr_code", successUpdate)
}

func (s *Service) publishToNATS(subject string, data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		s.logger.Error("Failed to marshal data for NATS", "error", err, "subject", subject)
		return
	}

	err = s.nc.Publish(subject, jsonData)
	if err != nil {
		s.logger.Error("Failed to publish to NATS", "error", err, "subject", subject)
		return
	}

	s.logger.Debug("Published to NATS", "subject", subject)
}

func (s *Service) bootstrapClient() {
	// Pass service instance to bootstrap function so it can use service-specific channels
	client := s.bootstrapWhatsAppClient()

	s.mu.Lock()
	s.client = client
	s.mu.Unlock()

	select {
	case s.clientChan <- client:
		s.logger.Debug("Client sent to registration channel")
	case <-s.ctx.Done():
		return
	default:
		s.logger.Debug("Client channel full, skipping send")
	}
}

func (s *Service) triggerAutoConnect() {
	select {
	case <-time.After(100 * time.Millisecond):
	case <-s.ctx.Done():
		return
	}

	select {
	case s.connectChan <- struct{}{}:
		s.logger.Info("Sent automatic WhatsApp connect signal on startup")
	case <-s.ctx.Done():
		return
	default:
		s.logger.Debug("WhatsApp connect channel already has a signal")
	}
}

func (s *Service) registerToolsWhenReady() {
	s.logger.Info("Waiting for WhatsApp client to register tool...")

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Debug("Tool registration handler stopping")
			return
		case client := <-s.clientChan:
			if client != nil {
				if err := s.toolRegistry.Register(waTools.NewWhatsAppTool(s.logger, client)); err != nil {
					s.logger.Error("Failed to register WhatsApp tool", "error", err)
				} else {
					s.logger.Info("WhatsApp tools registered")
				}
			}
		}
	}
}

// bootstrapWhatsAppClient creates a WhatsApp client with service-specific event handling.
func (s *Service) bootstrapWhatsAppClient() *whatsmeow.Client {
	// TODO: This still uses the global state version temporarily
	// Need to refactor BootstrapWhatsAppClient to accept service instance
	return BootstrapWhatsAppClient(
		s.memoryStorage,
		s.dbsqlc,
		s.logger,
		s.nc,
		s.envs.DBPath,
		s.envs,
		s.aiService,
	)
}

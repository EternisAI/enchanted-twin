package whatsapp

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/nats-io/nats.go"
	"go.mau.fi/whatsmeow"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/config"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	waTools "github.com/EternisAI/enchanted-twin/pkg/whatsapp/tools"
)

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
	qrChan        chan QRCodeEvent
	connectChan   chan struct{}
	clientChan    chan *whatsmeow.Client

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
		qrChan:        make(chan QRCodeEvent, 100),
		connectChan:   make(chan struct{}, 1),
		clientChan:    make(chan *whatsmeow.Client, 1),
		ctx:           ctx,
		cancel:        cancel,
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

	if s.client != nil {
		if s.client.IsConnected() {
			s.logger.Debug("Disconnecting WhatsApp client...")
			s.client.Disconnect()
		}
		s.client = nil
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
	case <-ctx.Done():
		s.logger.Warn("Context canceled while waiting for WhatsApp service goroutines to stop")
	}

	s.drainChannels()
	s.started = false
	s.logger.Info("WhatsApp service stopped")
	return nil
}

func (s *Service) drainChannels() {
	s.logger.Debug("Draining service channels...")

	clientDrained := 0
	qrDrained := 0
	connectDrained := 0

drainClientLoop:
	for {
		select {
		case <-s.clientChan:
			clientDrained++
		default:
			break drainClientLoop
		}
	}

drainQRLoop:
	for {
		select {
		case <-s.qrChan:
			qrDrained++
		default:
			break drainQRLoop
		}
	}

drainConnectLoop:
	for {
		select {
		case <-s.connectChan:
			connectDrained++
		default:
			break drainConnectLoop
		}
	}

	if clientDrained > 0 || qrDrained > 0 || connectDrained > 0 {
		s.logger.Debug("Drained buffered items from channels",
			"client_items", clientDrained,
			"qr_items", qrDrained,
			"connect_items", connectDrained)
	}
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

	StartSync()
	UpdateSyncStatus(SyncStatus{
		IsSyncing:      true,
		IsCompleted:    false,
		ProcessedItems: 0,
		TotalItems:     0,
		StatusMessage:  "Waiting for history sync to begin",
	})

	if err := PublishSyncStatus(s.nc, s.logger); err != nil {
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
	client := BootstrapWhatsAppClient(
		s.memoryStorage,
		s.dbsqlc,
		s.logger,
		s.nc,
		s.envs.DBPath,
		s.envs,
		s.aiService,
		s.connectChan,
		s.qrChan,
	)

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

// GetQRChannel returns the service's QR code channel.
func (s *Service) GetQRChannel() chan QRCodeEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.qrChan
}

// GetConnectChannel returns the service's connect channel.
func (s *Service) GetConnectChannel() chan struct{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.connectChan
}

// TriggerConnect sends a signal to start WhatsApp connection.
func (s *Service) TriggerConnect() {
	select {
	case s.connectChan <- struct{}{}:
		s.logger.Debug("Connection trigger sent successfully")
	default:
		s.logger.Debug("Connection trigger already pending")
	}
}

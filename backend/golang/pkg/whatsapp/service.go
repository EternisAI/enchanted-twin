package whatsapp

import (
	"context"
	"encoding/json"
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
	service := &Service{
		logger:        cfg.Logger,
		nc:            cfg.NatsClient,
		dbsqlc:        cfg.Database,
		memoryStorage: cfg.MemoryStorage,
		envs:          cfg.Config,
		aiService:     cfg.AIService,
		toolRegistry:  cfg.ToolRegistry,
		qrChan:        GetQRChannel(),
		connectChan:   GetConnectChannel(),
		clientChan:    make(chan *whatsmeow.Client),
	}

	return service
}

func (s *Service) Start(ctx context.Context) error {
	s.logger.Info("Starting WhatsApp service...")

	go s.handleQRCodeEvents(ctx)
	go s.bootstrapClient(ctx)
	go s.triggerAutoConnect()
	go s.registerToolsWhenReady(ctx)

	s.logger.Info("WhatsApp service started successfully")
	return nil
}

func (s *Service) Stop(ctx context.Context) error {
	s.logger.Info("Stopping WhatsApp service...")
	if s.client != nil && s.client.IsConnected() {
		s.client.Disconnect()
	}
	return nil
}

func (s *Service) GetCurrentQRCode() *string {
	return s.currentQRCode
}

func (s *Service) IsConnected() bool {
	return s.isConnected
}

func (s *Service) GetClient() *whatsmeow.Client {
	return s.client
}

func (s *Service) handleQRCodeEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
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
	s.currentQRCode = &qrCode
	s.isConnected = false
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
	s.isConnected = true
	s.currentQRCode = nil
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

func (s *Service) bootstrapClient(ctx context.Context) {
	client := BootstrapWhatsAppClient(
		s.memoryStorage,
		s.dbsqlc,
		s.logger,
		s.nc,
		s.envs.DBPath,
		s.envs,
		s.aiService,
	)

	s.client = client
	s.clientChan <- client
}

func (s *Service) triggerAutoConnect() {
	time.Sleep(100 * time.Millisecond)
	select {
	case s.connectChan <- struct{}{}:
		s.logger.Info("Sent automatic WhatsApp connect signal on startup")
	default:
		s.logger.Debug("WhatsApp connect channel already has a signal")
	}
}

func (s *Service) registerToolsWhenReady(ctx context.Context) {
	s.logger.Info("Waiting for WhatsApp client to register tool...")

	select {
	case <-ctx.Done():
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

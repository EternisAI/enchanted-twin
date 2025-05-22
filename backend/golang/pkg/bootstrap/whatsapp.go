package bootstrap

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/charmbracelet/log"
	"github.com/nats-io/nats.go"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/whatsapp"
)

func BootstrapWhatsAppClient(memoryStorage memory.Storage, logger *log.Logger, nc *nats.Conn, dbPath string) *whatsmeow.Client {
	dbLog := &whatsapp.WhatsmeowLoggerAdapter{Logger: logger, Module: "Database"}

	dbDir := filepath.Dir(dbPath)
	if dbDir != "." {
		if err := os.MkdirAll(dbDir, 0o755); err != nil {
			logger.Error("Failed to create WhatsApp database directory", "error", err)
			panic(err)
		}
	}

	container, err := sqlstore.New("sqlite3", "file:"+dbPath+"?_foreign_keys=on", dbLog)
	if err != nil {
		panic(err)
	}
	deviceStore, err := container.GetFirstDevice()
	if err != nil {
		panic(err)
	}
	clientLog := &whatsapp.WhatsmeowLoggerAdapter{Logger: logger, Module: "Client"}
	client := whatsmeow.NewClient(deviceStore, clientLog)
	client.AddEventHandler(whatsapp.EventHandler(memoryStorage, logger, nc))

	logger.Info("Waiting for WhatsApp connection signal...")
	connectChan := whatsapp.GetConnectChannel()
	<-connectChan
	logger.Info("Received signal to start WhatsApp connection")

	if client.Store.ID == nil {
		qrChan, _ := client.GetQRChannel(context.Background())
		err = client.Connect()
		if err != nil {
			logger.Error("Error connecting to WhatsApp", "error", err)
		}
		for evt := range qrChan {
			switch evt.Event {
			case "code":
				qrEvent := whatsapp.QRCodeEvent{
					Event: evt.Event,
					Code:  evt.Code,
				}
				whatsapp.SetLatestQREvent(qrEvent)
				whatsappQRChan := whatsapp.GetQRChannel()
				select {
				case whatsappQRChan <- qrEvent:
				default:
					logger.Warn("Warning: QR channel buffer full, dropping event")
				}
				logger.Info("Received new WhatsApp QR code", "qr_code", evt.Code)
			case "success":
				qrEvent := whatsapp.QRCodeEvent{
					Event: "success",
					Code:  "",
				}
				whatsapp.SetLatestQREvent(qrEvent)
				whatsapp.GetQRChannel() <- qrEvent
				logger.Info("WhatsApp connection successful")

				whatsapp.StartSync()
				whatsapp.UpdateSyncStatus(whatsapp.SyncStatus{
					IsSyncing:      true,
					IsCompleted:    false,
					ProcessedItems: 0,
					TotalItems:     0,
					StatusMessage:  "Waiting for history sync to begin",
				})
				err = whatsapp.PublishSyncStatus(nc, logger)
				if err != nil {
					logger.Error("Error publishing sync status", "error", err)
				}

			default:
				logger.Info("Login event", "event", evt.Event)
			}
		}
	} else {
		err = client.Connect()
		if err != nil {
			logger.Error("Error connecting to WhatsApp", "error", err)
		} else {
			qrEvent := whatsapp.QRCodeEvent{
				Event: "success",
				Code:  "",
			}
			whatsapp.SetLatestQREvent(qrEvent)
			whatsapp.GetQRChannel() <- qrEvent
			logger.Info("Already logged in, reusing session")

			whatsapp.StartSync()
			whatsapp.UpdateSyncStatus(whatsapp.SyncStatus{
				IsSyncing:      true,
				IsCompleted:    false,
				ProcessedItems: 0,
				TotalItems:     0,
				StatusMessage:  "Waiting for history sync to begin",
			})
			err = whatsapp.PublishSyncStatus(nc, logger)
			if err != nil {
				logger.Error("Error publishing sync status", "error", err)
			}
		}
	}

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		client.Disconnect()
	}()

	return client
}

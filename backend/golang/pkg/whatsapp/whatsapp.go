// Owner: slim@eternis.ai
package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	"github.com/nats-io/nats.go"
	"github.com/samber/lo"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/config"
	dataprocessing_whatsapp "github.com/EternisAI/enchanted-twin/pkg/dataprocessing/whatsapp"
)

// WhatsmeowLoggerAdapter adapts github.com/charmbracelet/log.Logger to whatsmeow's Logger interface.
type WhatsmeowLoggerAdapter struct {
	Logger *log.Logger
	Module string
}

func (a *WhatsmeowLoggerAdapter) Warnf(msg string, args ...interface{}) {
	a.Logger.Warnf("[WA:%s] "+msg, append([]interface{}{a.Module}, args...)...)
}

func (a *WhatsmeowLoggerAdapter) Errorf(msg string, args ...interface{}) {
	a.Logger.Errorf("[WA:%s] "+msg, append([]interface{}{a.Module}, args...)...)
}

func (a *WhatsmeowLoggerAdapter) Infof(msg string, args ...interface{}) {
	a.Logger.Infof("[WA:%s] "+msg, append([]interface{}{a.Module}, args...)...)
}

func (a *WhatsmeowLoggerAdapter) Debugf(msg string, args ...interface{}) {
	a.Logger.Debugf("[WA:%s] "+msg, append([]interface{}{a.Module}, args...)...)
}

func (a *WhatsmeowLoggerAdapter) Sub(module string) waLog.Logger {
	return &WhatsmeowLoggerAdapter{Logger: a.Logger, Module: a.Module + "/" + module}
}

type QRCodeEvent struct {
	Event string
	Code  string
}

type SyncStatus struct {
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

var (
	QRChan     chan QRCodeEvent
	QRChanOnce sync.Once

	latestQREvent     *QRCodeEvent
	latestQREventLock sync.RWMutex

	ConnectChan     chan struct{}
	ConnectChanOnce sync.Once

	allContacts     []WhatsappContact
	allContactsLock sync.RWMutex

	syncStatus     SyncStatus
	syncStatusLock sync.RWMutex
)

func GetQRChannel() chan QRCodeEvent {
	QRChanOnce.Do(func() {
		QRChan = make(chan QRCodeEvent, 100)
	})
	return QRChan
}

func GetLatestQREvent() *QRCodeEvent {
	latestQREventLock.RLock()
	defer latestQREventLock.RUnlock()
	return latestQREvent
}

func SetLatestQREvent(evt QRCodeEvent) {
	latestQREventLock.Lock()
	defer latestQREventLock.Unlock()
	latestQREvent = &evt
}

func GetConnectChannel() chan struct{} {
	ConnectChanOnce.Do(func() {
		ConnectChan = make(chan struct{}, 1)
	})
	return ConnectChan
}

func GetSyncStatus() SyncStatus {
	syncStatusLock.RLock()
	defer syncStatusLock.RUnlock()
	return syncStatus
}

func UpdateSyncStatus(newStatus SyncStatus) {
	syncStatusLock.Lock()
	defer syncStatusLock.Unlock()

	newStatus.LastUpdateTime = time.Now()

	if newStatus.TotalItems > 0 {
		newStatus.Progress = float64(newStatus.ProcessedItems) / float64(newStatus.TotalItems) * 100

		if newStatus.ProcessedItems > 0 && newStatus.StartTime.Unix() > 0 {
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

	syncStatus = newStatus
}

func StartSync() {
	syncStatusLock.Lock()
	defer syncStatusLock.Unlock()

	syncStatus = SyncStatus{
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

func PublishSyncStatus(nc *nats.Conn, logger *log.Logger) error {
	status := GetSyncStatus()

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
		logger.Error("Failed to marshal WhatsApp sync status", "error", err)
		return err
	}

	err = nc.Publish("whatsapp.sync.status", data)
	if err != nil {
		logger.Error("Failed to publish WhatsApp sync status", "error", err)
		return err
	}

	logger.Debug("Published WhatsApp sync status",
		"syncing", status.IsSyncing,
		"completed", status.IsCompleted,
		"message", status.StatusMessage)

	return nil
}

func IsSyncComplete() bool {
	status := GetSyncStatus()
	return !status.IsSyncing && status.TotalItems > 0 && status.ProcessedItems >= status.TotalItems
}

func IsSyncInProgress() bool {
	return GetSyncStatus().IsSyncing
}

func GetSyncProgress() float64 {
	return GetSyncStatus().Progress
}

func normalizeJID(jid string) string {
	if idx := strings.Index(jid, "@"); idx > 0 {
		return jid[:idx]
	}
	return jid
}

type WhatsappContact struct {
	Jid  string
	Name string
}

func addContact(jid, name string) {
	allContactsLock.Lock()
	defer allContactsLock.Unlock()

	exists := lo.ContainsBy(allContacts, func(c WhatsappContact) bool {
		return c.Jid == jid
	})

	if !exists {
		allContacts = append(allContacts, WhatsappContact{
			Jid:  jid,
			Name: name,
		})
	}
}

func findContactByJID(jid string) (WhatsappContact, bool) {
	allContactsLock.RLock()
	defer allContactsLock.RUnlock()

	contact, found := lo.Find(allContacts, func(c WhatsappContact) bool {
		return c.Jid == jid
	})

	if found {
		return contact, true
	}

	normJID := normalizeJID(jid)
	return lo.Find(allContacts, func(c WhatsappContact) bool {
		return normalizeJID(c.Jid) == normJID
	})
}

func formatSyncStatusMessage(contacts, messages int) string {
	if contacts == 0 && messages == 0 {
		return "Starting sync"
	} else if contacts == 0 {
		return fmt.Sprintf("Starting sync of %d messages", messages)
	} else if messages == 0 {
		return fmt.Sprintf("Starting sync of %d contacts", contacts)
	}
	return fmt.Sprintf("Starting sync of %d contacts and %d messages", contacts, messages)
}

func formatProgressStatusMessage(processed, totalContacts int) string {
	if totalContacts == 0 {
		return fmt.Sprintf("Processing data (%d items)", processed)
	}
	return fmt.Sprintf("Processed %d/%d contacts", processed, totalContacts)
}

func formatDetailedProgressMessage(totalContacts, processedMessages, totalMessages int) string {
	var parts []string

	if totalContacts > 0 {
		parts = append(parts, fmt.Sprintf("Processed %d/%d contacts", totalContacts, totalContacts))
	}

	if totalMessages > 0 {
		parts = append(parts, fmt.Sprintf("%d/%d messages", processedMessages, totalMessages))
	}

	if len(parts) == 0 {
		return "Processing data"
	}

	return strings.Join(parts, " and ")
}

func EventHandler(memoryStorage memory.Storage, logger *log.Logger, nc *nats.Conn, config *config.Config) func(interface{}) {
	return func(evt interface{}) {
		switch v := evt.(type) {
		case *events.HistorySync:

			StartSync()

			totalContacts := len(v.Data.Pushnames)
			totalMessages := 0
			for _, conversation := range v.Data.Conversations {
				totalMessages += len(conversation.Messages)
			}
			totalItems := totalContacts + totalMessages

			UpdateSyncStatus(SyncStatus{
				IsCompleted:       false,
				IsSyncing:         true,
				ProcessedItems:    0,
				TotalItems:        totalItems,
				StatusMessage:     formatSyncStatusMessage(totalContacts, totalMessages),
				EstimatedTimeLeft: "Calculating...",
			})
			err := PublishSyncStatus(nc, logger)
			if err != nil {
				logger.Error("Error publishing sync status", "error", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()

			processedItems := 0

			contactDocuments := []memory.TextDocument{}
			for _, pushname := range v.Data.Pushnames {
				if pushname.ID != nil && pushname.Pushname != nil {
					document, err := dataprocessing_whatsapp.ProcessNewContact(ctx, memoryStorage, *pushname.ID, *pushname.Pushname)
					if err != nil {
						logger.Error("Error processing WhatsApp contact", "error", err)
					} else {
						logger.Info("WhatsApp contact stored successfully", "pushname", *pushname.Pushname)
						contactDocuments = append(contactDocuments, document)
					}

					addContact(*pushname.ID, *pushname.Pushname)
				}

				processedItems++
				if processedItems%10 == 0 || processedItems == len(v.Data.Pushnames) {
					UpdateSyncStatus(SyncStatus{
						IsCompleted:    false,
						IsSyncing:      true,
						ProcessedItems: processedItems,
						TotalItems:     totalItems,
						StatusMessage:  formatProgressStatusMessage(processedItems, totalContacts),
					})
					err := PublishSyncStatus(nc, logger)
					if err != nil {
						logger.Error("Error publishing sync status", "error", err)
					}
				}
			}

			if len(contactDocuments) > 0 {
				logger.Info("Storing WhatsApp contacts using StoreRawData...")
				err = memoryStorage.StoreRawData(ctx, contactDocuments, nil)
				if err != nil {
					logger.Error("Error storing WhatsApp contacts", "error", err)
				}
			}

			conversationDocuments := []memory.TextDocument{}
			for i, conversation := range v.Data.Conversations {
				if conversation.ID == nil {
					logger.Warn("Skipping conversation with nil ID", "conversation_index", i)
					continue
				}

				chatID := *conversation.ID

				processedConversationMessages := 0

				for _, messageInfo := range conversation.Messages {
					userReceipts := messageInfo.GetMessage().UserReceipt
					contacts := []string{}

					for _, userReceipt := range userReceipts {
						if userReceipt.UserJID == nil {
							continue
						}

						userJID := *userReceipt.UserJID

						contact, ok := findContactByJID(userJID)
						if ok {
							contacts = append(contacts, contact.Name)
						} else {
							contacts = append(contacts, normalizeJID(userJID))
						}
					}

					message := messageInfo.GetMessage()
					if message == nil || message.Key == nil || message.Key.FromMe == nil {
						continue
					}

					var content string
					if msg := message.Message.GetConversation(); msg != "" {
						content = msg
					} else {
						continue
					}

					fromName := ""
					toName := chatID

					if *message.Key.FromMe {
						fromName = "me"
						if len(contacts) > 0 {
							toName = contacts[0]
						}
					} else if len(contacts) > 0 {
						fromName = contacts[0]
						toName = "me"
					}

					document, err := dataprocessing_whatsapp.ProcessHistoricalMessage(
						ctx,
						memoryStorage,
						content,
						fromName,
						toName,
						*message.MessageTimestamp,
					)
					if err != nil {
						logger.Error("Error processing historical WhatsApp message", "error", err)
					} else {
						conversationDocuments = append(conversationDocuments, document)
					}

					processedItems++
					processedConversationMessages++

					if processedItems%20 == 0 || processedItems == totalItems {
						UpdateSyncStatus(SyncStatus{
							IsCompleted:    false,
							IsSyncing:      true,
							ProcessedItems: processedItems,
							TotalItems:     totalItems,
							StatusMessage:  formatDetailedProgressMessage(totalContacts, processedItems-totalContacts, totalMessages),
						})
						err := PublishSyncStatus(nc, logger)
						if err != nil {
							logger.Error("Error publishing sync status", "error", err)
						}
					}
				}
			}

			if len(conversationDocuments) > 0 {
				err = memoryStorage.Store(ctx, conversationDocuments, nil)
				if err != nil {
					logger.Error("Error storing WhatsApp conversation documents", "error", err)
				} else {
					logger.Info("WhatsApp conversation documents storage completed successfully", "count", len(conversationDocuments))
				}
			}
			UpdateSyncStatus(SyncStatus{
				IsCompleted:    true,
				IsSyncing:      false,
				ProcessedItems: processedItems,
				TotalItems:     totalItems,
				StatusMessage:  "WhatsApp history sync completed",
			})
			err = PublishSyncStatus(nc, logger)
			if err != nil {
				logger.Error("Error publishing sync status", "error", err)
			}

		case *events.Message:

			message := v.Message.GetConversation()
			if message == "" {
				if v.Message.GetImageMessage() != nil {
					message = "[IMAGE]"
				} else if v.Message.GetVideoMessage() != nil {
					message = "[VIDEO]"
				} else if v.Message.GetAudioMessage() != nil {
					message = "[AUDIO]"
				} else if v.Message.GetDocumentMessage() != nil {
					message = "[DOCUMENT]"
				} else if v.Message.GetStickerMessage() != nil {
					message = "[STICKER]"
				}
			}

			if message == "" {
				logger.Info("Received a message with empty content")
				return
			}

			if v.Info.Sender.User != "" && v.Info.PushName != "" {
				senderJID := v.Info.Sender.String()
				addContact(senderJID, v.Info.PushName)
			}

			fromName := v.Info.PushName
			if fromName == "" {
				contact, found := findContactByJID(v.Info.Sender.String())
				if found {
					fromName = contact.Name
				} else {
					fromName = v.Info.Sender.User
				}
			}

			toName := v.Info.Chat.User

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()

			document, err := dataprocessing_whatsapp.ProcessNewMessage(ctx, memoryStorage, message, fromName, toName)
			if err != nil {
				logger.Error("Error processing WhatsApp message", "error", err)
			} else {
				logger.Info("WhatsApp message stored successfully")
			}

			err = memoryStorage.Store(ctx, []memory.TextDocument{document}, nil)
			if err != nil {
				logger.Error("Error storing WhatsApp message", "error", err)
			}

		default:
			logger.Info("Received unhandled event", "event", evt)
		}
	}
}

func BootstrapWhatsAppClient(memoryStorage memory.Storage, logger *log.Logger, nc *nats.Conn, dbPath string, config *config.Config) *whatsmeow.Client {
	dbLog := &WhatsmeowLoggerAdapter{Logger: logger, Module: "Database"}

	dbDir := filepath.Dir(dbPath)
	if dbDir != "." {
		if err := os.MkdirAll(dbDir, 0o755); err != nil {
			logger.Error("Failed to create WhatsApp database directory", "error", err)
			panic(err)
		}
	}

	dbFilePath := filepath.Join(dbDir, "whatsapp_store.db")
	container, err := sqlstore.New("sqlite3", "file:"+dbFilePath+"?_foreign_keys=on", dbLog)
	if err != nil {
		logger.Info("Failed to create WhatsApp database:", "dbFilePath", dbFilePath)
		logger.Error("Failed to create WhatsApp database:", "error", err, "dbFilePath", dbFilePath)
		panic(err)
	}
	deviceStore, err := container.GetFirstDevice()
	if err != nil {
		panic(err)
	}
	clientLog := &WhatsmeowLoggerAdapter{Logger: logger, Module: "Client"}
	client := whatsmeow.NewClient(deviceStore, clientLog)
	client.AddEventHandler(EventHandler(memoryStorage, logger, nc, config))

	logger.Info("Waiting for WhatsApp connection signal...")
	connectChan := GetConnectChannel()
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
				qrEvent := QRCodeEvent{
					Event: evt.Event,
					Code:  evt.Code,
				}
				SetLatestQREvent(qrEvent)
				whatsappQRChan := GetQRChannel()
				select {
				case whatsappQRChan <- qrEvent:
				default:
					logger.Warn("Warning: QR channel buffer full, dropping event")
				}
				logger.Info("Received new WhatsApp QR code", "qr_code", evt.Code)
			case "success":
				qrEvent := QRCodeEvent{
					Event: "success",
					Code:  "",
				}
				SetLatestQREvent(qrEvent)
				GetQRChannel() <- qrEvent
				logger.Info("WhatsApp connection successful")

				StartSync()
				UpdateSyncStatus(SyncStatus{
					IsSyncing:      true,
					IsCompleted:    false,
					ProcessedItems: 0,
					TotalItems:     0,
					StatusMessage:  "Waiting for history sync to begin",
				})
				err = PublishSyncStatus(nc, logger)
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
			qrEvent := QRCodeEvent{
				Event: "success",
				Code:  "",
			}
			SetLatestQREvent(qrEvent)
			GetQRChannel() <- qrEvent
			logger.Info("Already logged in, reusing session")

			StartSync()
			UpdateSyncStatus(SyncStatus{
				IsSyncing:      true,
				IsCompleted:    false,
				ProcessedItems: 0,
				TotalItems:     0,
				StatusMessage:  "Waiting for history sync to begin",
			})
			err = PublishSyncStatus(nc, logger)
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

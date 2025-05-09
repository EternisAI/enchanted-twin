package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.mau.fi/whatsmeow/types/events"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	dataprocessing_whatsapp "github.com/EternisAI/enchanted-twin/pkg/dataprocessing/whatsapp"
	"github.com/charmbracelet/log"
	"github.com/nats-io/nats.go"
	"github.com/samber/lo"
)

type QRCodeEvent struct {
	Event string
	Code  string
}

type SyncStatus struct {
	IsActive          bool
	Progress          float64
	TotalItems        int
	ProcessedItems    int
	StartTime         time.Time
	LastUpdateTime    time.Time
	StatusMessage     string
	EstimatedTimeLeft string
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

func TriggerConnect() {
	GetConnectChannel() <- struct{}{}
}

// GetSyncStatus returns a copy of the current sync status
func GetSyncStatus() SyncStatus {
	syncStatusLock.RLock()
	defer syncStatusLock.RUnlock()
	return syncStatus
}

// UpdateSyncStatus updates the sync status and calculates progress
func UpdateSyncStatus(isActive bool, processed, total int, statusMessage string) {
	syncStatusLock.Lock()
	defer syncStatusLock.Unlock()

	syncStatus.IsActive = isActive
	syncStatus.ProcessedItems = processed
	syncStatus.TotalItems = total
	syncStatus.StatusMessage = statusMessage
	syncStatus.LastUpdateTime = time.Now()

	if total > 0 {
		syncStatus.Progress = float64(processed) / float64(total) * 100

		// Calculate estimated time left if we have enough data
		if processed > 0 && syncStatus.StartTime.Unix() > 0 {
			elapsedTime := time.Since(syncStatus.StartTime)
			itemsPerSecond := float64(processed) / elapsedTime.Seconds()
			if itemsPerSecond > 0 {
				remainingItems := total - processed
				remainingSeconds := float64(remainingItems) / itemsPerSecond
				remainingDuration := time.Duration(remainingSeconds) * time.Second

				if remainingDuration > 1*time.Hour {
					syncStatus.EstimatedTimeLeft = fmt.Sprintf("~%.1f hours", remainingDuration.Hours())
				} else if remainingDuration > 1*time.Minute {
					syncStatus.EstimatedTimeLeft = fmt.Sprintf("~%.1f minutes", remainingDuration.Minutes())
				} else {
					syncStatus.EstimatedTimeLeft = fmt.Sprintf("~%.0f seconds", remainingDuration.Seconds())
				}
			}
		}
	}

	// If we're just starting the sync
	if isActive && processed == 0 && total > 0 && syncStatus.StartTime.IsZero() {
		syncStatus.StartTime = time.Now()
	}

	// If we're finishing the sync
	if processed >= total && total > 0 {
		syncStatus.IsActive = false
		syncStatus.Progress = 100
		syncStatus.EstimatedTimeLeft = "Complete"
	}
}

// StartSync initializes a new sync process
func StartSync() {
	syncStatusLock.Lock()
	defer syncStatusLock.Unlock()

	syncStatus = SyncStatus{
		IsActive:          true,
		Progress:          0,
		ProcessedItems:    0,
		TotalItems:        0,
		StartTime:         time.Now(),
		LastUpdateTime:    time.Now(),
		StatusMessage:     "Preparing to sync WhatsApp history...",
		EstimatedTimeLeft: "Calculating...",
	}
}

// PublishSyncStatus publishes the current sync status to NATS
func PublishSyncStatus(nc *nats.Conn, logger *log.Logger) error {
	status := GetSyncStatus()

	type syncStatusPublish struct {
		IsSyncing         bool    `json:"IsSyncing"`
		IsActive          bool    `json:"isActive"`
		Progress          float64 `json:"progress"`
		TotalItems        int     `json:"totalItems"`
		ProcessedItems    int     `json:"processedItems"`
		StatusMessage     string  `json:"statusMessage"`
		EstimatedTimeLeft string  `json:"estimatedTimeLeft"`
	}

	publishData := syncStatusPublish{
		IsSyncing:         true,
		IsActive:          status.IsActive,
		Progress:          status.Progress,
		TotalItems:        status.TotalItems,
		ProcessedItems:    status.ProcessedItems,
		StatusMessage:     status.StatusMessage,
		EstimatedTimeLeft: status.EstimatedTimeLeft,
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
		"active", status.IsActive,
		"progress", status.Progress,
		"processed", status.ProcessedItems,
		"total", status.TotalItems)

	return nil
}

// IsSyncComplete returns true if the sync process has completed successfully
func IsSyncComplete() bool {
	status := GetSyncStatus()
	return !status.IsActive && status.TotalItems > 0 && status.ProcessedItems >= status.TotalItems
}

// IsSyncInProgress returns true if the sync is currently active
func IsSyncInProgress() bool {
	return GetSyncStatus().IsActive
}

// GetSyncProgress returns the current sync progress as a percentage (0-100)
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
		fmt.Printf("Added contact to persistent store: %s - %s\n", jid, name)
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

func EventHandler(memoryStorage memory.Storage, logger *log.Logger, nc *nats.Conn) func(interface{}) {
	return func(evt interface{}) {
		switch v := evt.(type) {

		case *events.HistorySync:
			StartSync()
			logger.Info("Received WhatsApp history sync", "contacts", len(v.Data.Pushnames))

			// Count total items to process for progress tracking
			totalContacts := len(v.Data.Pushnames)
			totalMessages := 0
			for _, conversation := range v.Data.Conversations {
				totalMessages += len(conversation.Messages)
			}
			totalItems := totalContacts + totalMessages

			UpdateSyncStatus(true, 0, totalItems, fmt.Sprintf("Starting sync of %d contacts and %d messages", totalContacts, totalMessages))
			PublishSyncStatus(nc, logger)

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			processedItems := 0

			for _, pushname := range v.Data.Pushnames {
				if pushname.ID != nil && pushname.Pushname != nil {
					err := dataprocessing_whatsapp.ProcessNewContact(ctx, memoryStorage, *pushname.ID, *pushname.Pushname)
					if err != nil {
						logger.Error("Error processing WhatsApp contact", "error", err)
					} else {
						logger.Info("WhatsApp contact stored successfully", "pushname", *pushname.Pushname)
					}

					addContact(*pushname.ID, *pushname.Pushname)
				}

				processedItems++
				if processedItems%10 == 0 || processedItems == len(v.Data.Pushnames) {
					UpdateSyncStatus(true, processedItems, totalItems, fmt.Sprintf("Processed %d/%d contacts", processedItems, totalContacts))
					PublishSyncStatus(nc, logger)
				}
			}

			for _, conversation := range v.Data.Conversations {
				if conversation.ID == nil {
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

					err := dataprocessing_whatsapp.ProcessHistoricalMessage(
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
						logger.Info("Historical WhatsApp message stored successfully")
					}

					processedItems++
					processedConversationMessages++

					if processedItems%20 == 0 || processedItems == totalItems {
						UpdateSyncStatus(true, processedItems, totalItems,
							fmt.Sprintf("Processed %d/%d contacts and %d/%d messages",
								totalContacts, totalContacts,
								processedItems-totalContacts, totalMessages))
						PublishSyncStatus(nc, logger)
					}
				}

				logger.Info("Finished processing conversation",
					"chat_id", chatID,
					"messages", processedConversationMessages)
			}

			// Mark sync as complete
			UpdateSyncStatus(false, totalItems, totalItems, "WhatsApp history sync completed")
			PublishSyncStatus(nc, logger)
			logger.Info("WhatsApp history sync completed", "total_processed", processedItems)

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

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			err := dataprocessing_whatsapp.ProcessNewMessage(ctx, memoryStorage, message, fromName, toName)
			if err != nil {
				logger.Error("Error processing WhatsApp message", "error", err)
			} else {
				logger.Info("WhatsApp message stored successfully")
			}

		default:
			logger.Info("Received unhandled event", "event", evt)
		}
	}
}

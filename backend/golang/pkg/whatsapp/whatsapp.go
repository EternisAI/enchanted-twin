// Owner: slim@eternis.ai
package whatsapp

import (
	"context"
	"database/sql"
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
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/samber/lo"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waHistorySync"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/config"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	whatsappdb "github.com/EternisAI/enchanted-twin/pkg/db/sqlc/whatsapp"
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

type GlobalState struct {
	qrChan          chan QRCodeEvent
	qrChanOnce      sync.Once
	latestQREvent   *QRCodeEvent
	connectChan     chan struct{}
	connectChanOnce sync.Once
	allContacts     []WhatsappContact
	syncStatus      SyncStatus
	mu              sync.RWMutex
}

var globalState = &GlobalState{}

func GetQRChannel() chan QRCodeEvent {
	globalState.mu.Lock()
	defer globalState.mu.Unlock()

	globalState.qrChanOnce.Do(func() {
		globalState.qrChan = make(chan QRCodeEvent, 100)
	})
	return globalState.qrChan
}

func GetLatestQREvent() *QRCodeEvent {
	globalState.mu.RLock()
	defer globalState.mu.RUnlock()
	return globalState.latestQREvent
}

func SetLatestQREvent(evt QRCodeEvent) {
	globalState.mu.Lock()
	defer globalState.mu.Unlock()
	globalState.latestQREvent = &evt
}

func GetConnectChannel() chan struct{} {
	globalState.mu.Lock()
	defer globalState.mu.Unlock()

	globalState.connectChanOnce.Do(func() {
		globalState.connectChan = make(chan struct{}, 1)
	})
	return globalState.connectChan
}

func GetSyncStatus() SyncStatus {
	globalState.mu.RLock()
	defer globalState.mu.RUnlock()
	return globalState.syncStatus
}

func UpdateSyncStatus(newStatus SyncStatus) {
	globalState.mu.Lock()
	defer globalState.mu.Unlock()

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

	globalState.syncStatus = newStatus
}

func StartSync() {
	globalState.mu.Lock()
	defer globalState.mu.Unlock()

	globalState.syncStatus = SyncStatus{
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
	globalState.mu.Lock()
	defer globalState.mu.Unlock()

	exists := lo.ContainsBy(globalState.allContacts, func(c WhatsappContact) bool {
		return c.Jid == jid
	})

	if !exists {
		globalState.allContacts = append(globalState.allContacts, WhatsappContact{
			Jid:  jid,
			Name: name,
		})
	}
}

func findContactByJID(jid string) (WhatsappContact, bool) {
	globalState.mu.RLock()
	defer globalState.mu.RUnlock()

	contact, found := lo.Find(globalState.allContacts, func(c WhatsappContact) bool {
		return c.Jid == jid
	})

	if found {
		return contact, true
	}

	normJID := normalizeJID(jid)
	return lo.Find(globalState.allContacts, func(c WhatsappContact) bool {
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

const (
	DefaultMaxMessagesInBuffer      = 100
	DefaultMaxTimeBetweenMessages   = 48 * time.Hour
	ConversationConfidenceThreshold = 0.8
)

func getConversationID(v *events.Message) string {
	if v.Info.Chat.User != "" {
		return normalizeJID(v.Info.Chat.User)
	}
	return v.Info.Chat.String()
}

func extractMessageContent(message *events.Message) (string, string) {
	var content, messageType string

	if msg := message.Message.GetConversation(); msg != "" {
		content = msg
		messageType = "text"
	} else if message.Message.GetImageMessage() != nil {
		content = "[IMAGE]"
		messageType = "image"
	} else if message.Message.GetVideoMessage() != nil {
		content = "[VIDEO]"
		messageType = "video"
	} else if message.Message.GetAudioMessage() != nil {
		content = "[AUDIO]"
		messageType = "audio"
	} else if message.Message.GetDocumentMessage() != nil {
		content = "[DOCUMENT]"
		messageType = "document"
	} else if message.Message.GetStickerMessage() != nil {
		content = "[STICKER]"
		messageType = "sticker"
	}

	return content, messageType
}

func getSenderInfo(v *events.Message) (string, string) {
	senderJID := ""
	senderName := ""

	if v.Info.IsFromMe {
		senderJID = "me"
		senderName = "me"
	} else {
		if v.Info.Sender.User != "" {
			senderJID = v.Info.Sender.String()
			if v.Info.PushName != "" {
				senderName = v.Info.PushName
			} else {
				senderName = normalizeJID(senderJID)
			}
		} else {
			senderJID = "unknown"
			senderName = "unknown"
		}
	}

	return senderJID, senderName
}

func convertMessagesToConversationDocument(messages []whatsappdb.WhatsappMessage, conversationID string) *memory.ConversationDocument {
	if len(messages) == 0 {
		return nil
	}

	var conversationMessages []memory.ConversationMessage
	var people []string
	peopleMap := make(map[string]bool)

	for _, msg := range messages {
		conversationMessages = append(conversationMessages, memory.ConversationMessage{
			Speaker: msg.SenderName,
			Content: msg.Content,
			Time:    msg.SentAt,
		})

		if !peopleMap[msg.SenderName] {
			people = append(people, msg.SenderName)
			peopleMap[msg.SenderName] = true
		}
	}

	conversationDoc := memory.ConversationDocument{
		FieldID:      fmt.Sprintf("whatsapp-chat-%s", conversationID),
		FieldSource:  "whatsapp",
		People:       people,
		User:         "me",
		Conversation: conversationMessages,
		FieldTags:    []string{"conversation", "chat"},
		FieldMetadata: map[string]string{
			"chat_id": conversationID,
			"type":    "conversation",
		},
	}
	return &conversationDoc
}

func UpdateWhatsappMemory(ctx context.Context, database *db.DB, memoryStorage memory.Storage, conversationID string, logger *log.Logger) error {
	// Begin transaction to ensure atomicity
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil {
			logger.Error("Failed to rollback transaction", "error", err)
		}
	}()

	txQueries := database.WhatsappQueries.WithTx(tx)

	messages, err := txQueries.GetWhatsappMessagesByConversation(ctx, conversationID)
	if err != nil {
		return fmt.Errorf("failed to get messages for conversation %s: %w", conversationID, err)
	}

	if len(messages) == 0 {
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit empty transaction: %w", err)
		}
		return nil
	}

	conversationDoc := convertMessagesToConversationDocument(messages, conversationID)
	if conversationDoc == nil {
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit empty transaction: %w", err)
		}
		return nil
	}

	documents := []memory.Document{conversationDoc}

	logger.Info("Storing WhatsApp conversation in memory",
		"conversation_id", conversationID,
		"messages_count", len(messages),
		"people_count", len(conversationDoc.People))

	err = memoryStorage.Store(ctx, documents, func(processed, total int) {
		logger.Debug("WhatsApp conversation storage progress",
			"conversation_id", conversationID,
			"processed", processed,
			"total", total)
	})
	if err != nil {
		return fmt.Errorf("failed to store conversation in memory: %w", err)
	}

	err = txQueries.DeleteWhatsappMessagesByConversation(ctx, conversationID)
	if err != nil {
		return fmt.Errorf("failed to delete buffered messages: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	logger.Info("Successfully flushed WhatsApp conversation to memory",
		"conversation_id", conversationID,
		"messages_count", len(messages))

	return nil
}

func shouldFlushToMemory(ctx context.Context, database *db.DB, conversationID string, newMessageTime time.Time, newMessage string, newSender string, analyzer *ConversationAnalyzer, logger *log.Logger) (bool, error) {
	messageCount, err := database.WhatsappQueries.GetWhatsappMessageCount(ctx, conversationID)
	if err != nil {
		return false, fmt.Errorf("failed to get message count: %w", err)
	}

	if messageCount >= DefaultMaxMessagesInBuffer {
		logger.Debug("Flushing to memory due to message count",
			"conversation_id", conversationID,
			"count", messageCount)
		return true, nil
	}

	if messageCount > 0 && analyzer != nil {
		recentMessages, err := database.WhatsappQueries.GetWhatsappMessagesByConversation(ctx, conversationID)
		if err != nil {
			logger.Error("Failed to get recent messages for AI assessment", "error", err)
		} else {
			if len(recentMessages) > 10 {
				recentMessages = recentMessages[len(recentMessages)-10:]
			}

			assessment, err := analyzer.AssessConversationBoundary(ctx, recentMessages, newMessage, newSender)
			if err != nil {
				logger.Error("Failed to get AI conversation assessment", "error", err)
			} else {
				logger.Debug("AI conversation assessment",
					"conversation_id", conversationID,
					"is_new_conversation", assessment.IsNewConversation,
					"confidence", assessment.Confidence,
					"reasoning", assessment.Reasoning)

				if assessment.IsNewConversation && assessment.Confidence > ConversationConfidenceThreshold {
					logger.Debug("Flushing to memory due to AI assessment of new conversation",
						"conversation_id", conversationID,
						"confidence", assessment.Confidence,
						"reasoning", assessment.Reasoning)
					return true, nil
				}

				if !assessment.IsNewConversation && assessment.Confidence > ConversationConfidenceThreshold {
					logger.Debug("Not flushing - AI assessment indicates continuing conversation",
						"conversation_id", conversationID,
						"confidence", assessment.Confidence)
					return false, nil
				}
			}
		}
	}

	if messageCount > 0 {
		latestMessage, err := database.WhatsappQueries.GetLatestWhatsappMessage(ctx, conversationID)
		if err != nil && err != sql.ErrNoRows {
			return false, fmt.Errorf("failed to get latest message: %w", err)
		}

		if err == nil {
			timeDiff := newMessageTime.Sub(latestMessage.SentAt)
			if timeDiff > DefaultMaxTimeBetweenMessages {
				logger.Debug("Flushing to memory due to time gap (fallback)",
					"conversation_id", conversationID,
					"time_diff", timeDiff.String())
				return true, nil
			}
		}
	}

	return false, nil
}

func ProcessNewConversationMessage(conversation *waHistorySync.Conversation, logger *log.Logger) *memory.ConversationDocument {
	if conversation.ID == nil {
		logger.Warn("Skipping conversation with nil ID")
		return nil
	}

	chatID := *conversation.ID
	var conversationMessages []memory.ConversationMessage
	var people []string
	peopleMap := make(map[string]bool)

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
		} else if message.Message.GetImageMessage() != nil {
			content = "[IMAGE]"
		} else if message.Message.GetVideoMessage() != nil {
			content = "[VIDEO]"
		} else if message.Message.GetAudioMessage() != nil {
			content = "[AUDIO]"
		} else if message.Message.GetDocumentMessage() != nil {
			content = "[DOCUMENT]"
		} else if message.Message.GetStickerMessage() != nil {
			content = "[STICKER]"
		} else {
			continue
		}

		fromName := ""
		if *message.Key.FromMe {
			fromName = "me"
		} else {
			if message.Key.Participant != nil && *message.Key.Participant != "" {
				participantJID := *message.Key.Participant
				if contact, found := findContactByJID(participantJID); found {
					fromName = contact.Name
				} else {
					fromName = normalizeJID(participantJID)
				}
			} else if message.Key.RemoteJID != nil && *message.Key.RemoteJID != "" {
				remoteJID := *message.Key.RemoteJID
				if contact, found := findContactByJID(remoteJID); found {
					fromName = contact.Name
				} else {
					fromName = normalizeJID(remoteJID)
				}
			} else {
				fromName = "unknown"
			}
		}

		if fromName != "" {
			timestamp := time.Unix(int64(*message.MessageTimestamp), 0)
			conversationMessages = append(conversationMessages, memory.ConversationMessage{
				Speaker: fromName,
				Content: content,
				Time:    timestamp,
			})

			if !peopleMap[fromName] {
				people = append(people, fromName)
				peopleMap[fromName] = true
			}

			for _, contact := range contacts {
				if !peopleMap[contact] {
					people = append(people, contact)
					peopleMap[contact] = true
				}
			}
		}
	}

	if len(conversationMessages) > 0 {
		conversationDoc := memory.ConversationDocument{
			FieldID:      fmt.Sprintf("whatsapp-chat-%s", chatID),
			FieldSource:  "whatsapp",
			People:       people,
			User:         "me",
			Conversation: conversationMessages,
			FieldTags:    []string{"conversation", "chat"},
			FieldMetadata: map[string]string{
				"chat_id": chatID,
				"type":    "conversation",
			},
		}
		return &conversationDoc
	}
	return nil
}

func EventHandler(memoryStorage memory.Storage, database *db.DB, logger *log.Logger, nc *nats.Conn, config *config.Config, aiService *ai.Service) func(interface{}) {
	var analyzer *ConversationAnalyzer
	if aiService != nil {
		analyzer = NewConversationAnalyzer(logger, aiService, config.CompletionsModel)
	}

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

			var contactDocuments []memory.Document
			for _, pushname := range v.Data.Pushnames {
				if pushname.ID != nil && pushname.Pushname != nil {
					timestamp := time.Now()
					document := &memory.TextDocument{
						FieldSource:    "whatsapp",
						FieldContent:   fmt.Sprintf("WhatsApp Contact name: %s. Contact ID: %s.", *pushname.Pushname, *pushname.ID),
						FieldTimestamp: &timestamp,
						FieldTags:      []string{"contact"},
						FieldMetadata: map[string]string{
							"contact_id": *pushname.ID,
							"name":       *pushname.Pushname,
							"type":       "contact",
						},
					}
					contactDocuments = append(contactDocuments, document)

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
				UpdateSyncStatus(SyncStatus{
					IsCompleted:    false,
					IsSyncing:      true,
					ProcessedItems: processedItems,
					TotalItems:     totalItems,
					StatusMessage:  "Storing WhatsApp contacts in memory...",
				})
				err := PublishSyncStatus(nc, logger)
				if err != nil {
					logger.Error("Error publishing sync status", "error", err)
				}

				logger.Info("Storing WhatsApp contacts through evolvingmemory fact extraction", "count", len(contactDocuments))
				err = memoryStorage.Store(ctx, contactDocuments, func(processed, total int) {
					logger.Info("WhatsApp contacts storage progress", "processed", processed, "total", total)

					UpdateSyncStatus(SyncStatus{
						IsCompleted:    false,
						IsSyncing:      true,
						ProcessedItems: processedItems,
						TotalItems:     totalItems,
						StatusMessage:  fmt.Sprintf("Storing WhatsApp contacts in memory (%d/%d)...", processed, total),
					})
					err = PublishSyncStatus(nc, logger)
					if err != nil {
						logger.Error("Error publishing sync status", "error", err)
					}
				})
				if err != nil {
					logger.Error("Error storing WhatsApp contacts", "error", err)
				} else {
					logger.Info("WhatsApp contacts stored successfully through evolvingmemory", "count", len(contactDocuments))

					testFilter := &memory.Filter{
						Source: func() *string { s := "whatsapp"; return &s }(),
						Tags: &memory.TagsFilter{
							All: []string{"whatsapp", "contact"},
						},
						Limit: func() *int { l := 10; return &l }(),
					}

					time.Sleep(2 * time.Second)

					result, queryErr := memoryStorage.Query(ctx, "WhatsApp contacts", testFilter)
					if queryErr != nil {
						logger.Error("Debug query for WhatsApp contacts failed", "error", queryErr)
					} else {
						logger.Info("Debug query for WhatsApp contacts successful",
							"facts_count", len(result.Facts))

						if len(result.Facts) > 0 {
							logger.Info("Sample WhatsApp contact fact", "content", result.Facts[0].Content, "source", result.Facts[0].Source)
						}
					}
				}
			}

			var documents []memory.Document

			for _, conversation := range v.Data.Conversations {
				conversationDoc := ProcessNewConversationMessage(conversation, logger)
				if conversationDoc != nil {
					documents = append(documents, conversationDoc)
				}

				processedItems++

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

			if len(documents) > 0 {
				UpdateSyncStatus(SyncStatus{
					IsCompleted:    false,
					IsSyncing:      true,
					ProcessedItems: processedItems,
					TotalItems:     totalItems,
					StatusMessage:  "Storing WhatsApp conversations in memory...",
				})
				err := PublishSyncStatus(nc, logger)
				if err != nil {
					logger.Error("Error publishing sync status", "error", err)
				}

				logger.Info("Storing WhatsApp conversations through evolvingmemory fact extraction", "count", len(documents))

				if len(documents) > 0 {
					if sampleConv, ok := documents[0].(*memory.ConversationDocument); ok {
						logger.Info("Sample conversation before storage",
							"id", sampleConv.FieldID,
							"source", sampleConv.FieldSource,
							"user", sampleConv.User,
							"people_count", len(sampleConv.People),
							"messages_count", len(sampleConv.Conversation),
							"content_preview", func() string {
								content := sampleConv.Content()
								if len(content) > 200 {
									return content[:200] + "..."
								}
								return content
							}())
					}
				}

				err = memoryStorage.Store(ctx, documents, func(processed, total int) {
					logger.Info("WhatsApp conversations storage progress", "processed", processed, "total", total)

					UpdateSyncStatus(SyncStatus{
						IsCompleted:    false,
						IsSyncing:      true,
						ProcessedItems: processedItems,
						TotalItems:     totalItems,
						StatusMessage:  fmt.Sprintf("Storing WhatsApp conversations in memory (%d/%d)...", processed, total),
					})
					err = PublishSyncStatus(nc, logger)
					if err != nil {
						logger.Error("Error publishing sync status", "error", err)
					}
				})
				if err != nil {
					logger.Error("Error storing WhatsApp conversation documents", "error", err)
				} else {
					logger.Info("WhatsApp conversation documents storage completed successfully through evolvingmemory", "count", len(documents))

					testFilter := &memory.Filter{
						Source: func() *string { s := "whatsapp"; return &s }(),
						Tags: &memory.TagsFilter{
							All: []string{"whatsapp", "conversation"},
						},
						Limit: func() *int { l := 10; return &l }(),
					}

					time.Sleep(2 * time.Second)

					result, queryErr := memoryStorage.Query(ctx, "WhatsApp conversations", testFilter)
					if queryErr != nil {
						logger.Error("Debug query for WhatsApp conversations failed", "error", queryErr)
					} else {
						logger.Info("Debug query for WhatsApp conversations successful",
							"facts_count", len(result.Facts))

						if len(result.Facts) > 0 {
							logger.Info("Sample WhatsApp conversation fact", "content", result.Facts[0].Content, "source", result.Facts[0].Source)
						}
					}
				}
			}

			UpdateSyncStatus(SyncStatus{
				IsCompleted:    true,
				IsSyncing:      false,
				ProcessedItems: processedItems,
				TotalItems:     totalItems,
				StatusMessage:  "WhatsApp history sync and memory storage completed",
			})
			err = PublishSyncStatus(nc, logger)
			if err != nil {
				logger.Error("Error publishing sync status", "error", err)
			}

		case *events.Message:
			logger.Info("Received WhatsApp message", "message", v)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			content, messageType := extractMessageContent(v)
			if content == "" {
				logger.Debug("Received a message with empty content")
				return
			}

			conversationID := getConversationID(v)
			senderJID, senderName := getSenderInfo(v)
			messageTimestamp := v.Info.Timestamp

			if v.Info.Sender.User != "" && v.Info.PushName != "" {
				addContact(senderJID, v.Info.PushName)
			}

			shouldFlush, err := shouldFlushToMemory(ctx, database, conversationID, messageTimestamp, content, senderName, analyzer, logger)
			if err != nil {
				logger.Error("Error checking flush condition", "error", err, "conversation_id", conversationID)
			} else if shouldFlush {
				err = UpdateWhatsappMemory(ctx, database, memoryStorage, conversationID, logger)
				if err != nil {
					logger.Error("Error flushing messages to memory", "error", err, "conversation_id", conversationID)
				}
			}

			messageID := uuid.New().String()
			err = database.WhatsappQueries.InsertWhatsappMessage(ctx, whatsappdb.InsertWhatsappMessageParams{
				ID:             messageID,
				ConversationID: conversationID,
				SenderJid:      senderJID,
				SenderName:     senderName,
				Content:        content,
				MessageType:    messageType,
				SentAt:         messageTimestamp,
				FromMe:         v.Info.IsFromMe,
			})
			if err != nil {
				logger.Error("Error storing WhatsApp message", "error", err,
					"conversation_id", conversationID,
					"sender", senderName)
			} else {
				logger.Debug("Stored WhatsApp message in buffer",
					"conversation_id", conversationID,
					"sender", senderName,
					"type", messageType)
			}

		default:
			logger.Info("Received unhandled event", "event", evt)
		}
	}
}

type ConnectionManager struct {
	client          *whatsmeow.Client
	logger          *log.Logger
	nc              *nats.Conn
	isConnected     bool
	isConnecting    bool
	isReconnecting  bool
	reconnectMux    sync.Mutex
	ctx             context.Context
	cancel          context.CancelFunc
	maxRetries      int
	baseDelay       time.Duration
	reconnectCancel context.CancelFunc
	reconnectMux2   sync.Mutex
}

type ConnectionManagerConfig struct {
	MaxMessagesInBuffer    int
	MaxTimeBetweenMessages time.Duration
	client                 *whatsmeow.Client
	logger                 *log.Logger
	nc                     *nats.Conn
}

func NewConnectionManager(config ConnectionManagerConfig) *ConnectionManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &ConnectionManager{
		client:     config.client,
		logger:     config.logger,
		nc:         config.nc,
		ctx:        ctx,
		cancel:     cancel,
		maxRetries: 10,
		baseDelay:  time.Second * 2,
	}
}

func (cm *ConnectionManager) Connect() error {
	cm.reconnectMux.Lock()
	defer cm.reconnectMux.Unlock()

	if cm.isConnecting {
		cm.logger.Debug("Connection already in progress")
		return nil
	}

	cm.isConnecting = true
	defer func() { cm.isConnecting = false }()

	cm.client.AddEventHandler(cm.handleConnectionEvents)

	return cm.connectWithRetry()
}

func (cm *ConnectionManager) connectWithRetry() error {
	var lastErr error

	for attempt := 0; attempt <= cm.maxRetries; attempt++ {
		select {
		case <-cm.ctx.Done():
			return cm.ctx.Err()
		default:
		}

		if attempt > 0 {
			delay := cm.calculateBackoff(attempt)
			cm.logger.Info("Retrying WhatsApp connection",
				"attempt", attempt,
				"delay", delay.String(),
				"last_error", lastErr)

			select {
			case <-cm.ctx.Done():
				return cm.ctx.Err()
			case <-time.After(delay):
			}
		}

		err := cm.attemptConnection()
		if err == nil {
			cm.isConnected = true
			cm.logger.Info("WhatsApp connection established successfully")

			go cm.monitorConnection()
			return nil
		}

		lastErr = err
		cm.logger.Error("WhatsApp connection attempt failed",
			"attempt", attempt+1,
			"error", err)
	}

	return fmt.Errorf("failed to connect after %d attempts, last error: %w", cm.maxRetries, lastErr)
}

func (cm *ConnectionManager) attemptConnection() error {
	ctx, cancel := context.WithTimeout(cm.ctx, 30*time.Second)
	defer cancel()

	errChan := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				cm.logger.Error("Recovered from panic during connection", "panic", r)
				errChan <- fmt.Errorf("connection panic: %v", r)
			}
		}()

		err := cm.client.Connect()
		errChan <- err
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf("connection timeout: %w", ctx.Err())
	case err := <-errChan:
		return err
	}
}

func (cm *ConnectionManager) calculateBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}

	backoff := cm.baseDelay * time.Duration(1<<uint(attempt-1))

	if backoff > 5*time.Minute {
		backoff = 5 * time.Minute
	}

	jitter := time.Duration(float64(backoff) * 0.25 * (2*0.5 - 1))
	return backoff + jitter
}

func (cm *ConnectionManager) handleConnectionEvents(evt interface{}) {
	switch evt.(type) {
	case *events.Disconnected:
		cm.logger.Warn("WhatsApp client disconnected")
		cm.isConnected = false

		// Don't auto-reconnect if we're shutting down
		select {
		case <-cm.ctx.Done():
			return
		default:
		}

		// Prevent multiple concurrent reconnection attempts
		cm.reconnectMux2.Lock()
		if cm.isReconnecting {
			cm.reconnectMux2.Unlock()
			return
		}
		cm.isReconnecting = true
		cm.reconnectMux2.Unlock()

		// Cancel any existing reconnection attempt
		if cm.reconnectCancel != nil {
			cm.reconnectCancel()
		}

		// Create new context for reconnection
		reconnectCtx, cancel := context.WithCancel(cm.ctx)
		cm.reconnectCancel = cancel

		// Attempt reconnection
		go func() {
			defer func() {
				cm.reconnectMux2.Lock()
				cm.isReconnecting = false
				cm.reconnectMux2.Unlock()
			}()

			cm.logger.Info("Attempting to reconnect WhatsApp client")
			if err := cm.connectWithRetryContext(reconnectCtx); err != nil {
				select {
				case <-reconnectCtx.Done():
					cm.logger.Info("Reconnection canceled")
				default:
					cm.logger.Error("Failed to reconnect WhatsApp client", "error", err)
				}
			}
		}()

	case *events.ConnectFailure:
		cm.logger.Error("WhatsApp connection failure")
		cm.isConnected = false
	}
}

func (cm *ConnectionManager) connectWithRetryContext(ctx context.Context) error {
	var lastErr error

	for attempt := 0; attempt <= cm.maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if attempt > 0 {
			delay := cm.calculateBackoff(attempt)
			cm.logger.Info("Retrying WhatsApp connection",
				"attempt", attempt,
				"delay", delay.String(),
				"last_error", lastErr)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		err := cm.attemptConnectionContext(ctx)
		if err == nil {
			cm.isConnected = true
			cm.logger.Info("WhatsApp connection established successfully")
			return nil
		}

		lastErr = err
		cm.logger.Error("WhatsApp connection attempt failed",
			"attempt", attempt+1,
			"error", err)
	}

	return fmt.Errorf("failed to connect after %d attempts, last error: %w", cm.maxRetries, lastErr)
}

func (cm *ConnectionManager) attemptConnectionContext(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	errChan := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				cm.logger.Error("Recovered from panic during connection", "panic", r)
				errChan <- fmt.Errorf("connection panic: %v", r)
			}
		}()

		err := cm.client.Connect()
		errChan <- err
	}()

	select {
	case <-timeoutCtx.Done():
		return fmt.Errorf("connection timeout: %w", timeoutCtx.Err())
	case err := <-errChan:
		return err
	}
}

func (cm *ConnectionManager) monitorConnection() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-cm.ctx.Done():
			// Cancel any ongoing reconnection attempts
			if cm.reconnectCancel != nil {
				cm.reconnectCancel()
			}
			return
		case <-ticker.C:
			if !cm.client.IsConnected() && cm.isConnected {
				cm.logger.Warn("WhatsApp connection lost, attempting reconnection")
				cm.isConnected = false

				// Prevent multiple concurrent reconnection attempts
				cm.reconnectMux2.Lock()
				if cm.isReconnecting {
					cm.reconnectMux2.Unlock()
					continue
				}
				cm.isReconnecting = true
				cm.reconnectMux2.Unlock()

				// Cancel any existing reconnection attempt
				if cm.reconnectCancel != nil {
					cm.reconnectCancel()
				}

				// Create new context for reconnection
				reconnectCtx, cancel := context.WithCancel(cm.ctx)
				cm.reconnectCancel = cancel

				go func() {
					defer func() {
						cm.reconnectMux2.Lock()
						cm.isReconnecting = false
						cm.reconnectMux2.Unlock()
					}()

					if err := cm.connectWithRetryContext(reconnectCtx); err != nil {
						select {
						case <-reconnectCtx.Done():
							cm.logger.Info("Reconnection canceled during monitoring")
						default:
							cm.logger.Error("Failed to reconnect during monitoring", "error", err)
						}
					}
				}()
			}
		}
	}
}

func (cm *ConnectionManager) Disconnect() {
	cm.cancel()

	// Cancel any ongoing reconnection attempts
	if cm.reconnectCancel != nil {
		cm.reconnectCancel()
	}

	cm.reconnectMux.Lock()
	defer cm.reconnectMux.Unlock()

	if cm.client.IsConnected() {
		cm.client.Disconnect()
	}
	cm.isConnected = false
}

func BootstrapWhatsAppClient(memoryStorage memory.Storage, database *db.DB, logger *log.Logger, nc *nats.Conn, dbPath string, config *config.Config, aiService *ai.Service) *whatsmeow.Client {
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
	client.AddEventHandler(EventHandler(memoryStorage, database, logger, nc, config, aiService))

	connManager := NewConnectionManager(ConnectionManagerConfig{
		client:                 client,
		logger:                 logger,
		nc:                     nc,
		MaxMessagesInBuffer:    DefaultMaxMessagesInBuffer,
		MaxTimeBetweenMessages: DefaultMaxTimeBetweenMessages,
	})

	logger.Info("Waiting for WhatsApp connection signal...")
	connectChan := GetConnectChannel()
	<-connectChan
	logger.Info("Received signal to start WhatsApp connection")

	if client.Store.ID == nil {
		qrChan, _ := client.GetQRChannel(context.Background())

		go func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("Recovered from panic during QR connection", "panic", r)
				}
			}()

			err := connManager.Connect()
			if err != nil {
				logger.Error("Error establishing WhatsApp connection", "error", err)
			}
		}()

		// Handle QR code events
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
		// Existing session - connect directly
		err = connManager.Connect()
		if err != nil {
			logger.Error("Error connecting to WhatsApp with existing session", "error", err)
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

	// Set up graceful shutdown
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		logger.Info("Received shutdown signal, disconnecting WhatsApp client")
		connManager.Disconnect()
	}()

	return client
}

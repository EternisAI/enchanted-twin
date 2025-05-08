package whatsapp

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.mau.fi/whatsmeow/types/events"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	dataprocessing_whatsapp "github.com/EternisAI/enchanted-twin/pkg/dataprocessing/whatsapp"
	"github.com/charmbracelet/log"
	"github.com/samber/lo"
)

type QRCodeEvent struct {
	Event string
	Code  string
}

var (
	QRChan     chan QRCodeEvent
	QRChanOnce sync.Once

	latestQREvent     *QRCodeEvent
	latestQREventLock sync.RWMutex

	ConnectChan     chan struct{}
	ConnectChanOnce sync.Once

	// Store contacts at package level to persist between events
	allContacts     []WhatsappContact
	allContactsLock sync.RWMutex
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

// normalizeJID extracts the phone number part from a JID to make matching more reliable
func normalizeJID(jid string) string {
	// Remove any suffix like @s.whatsapp.net
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

func EventHandler(memoryStorage memory.Storage, logger *log.Logger) func(interface{}) {
	return func(evt interface{}) {
		switch v := evt.(type) {

		case *events.HistorySync:

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

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
			}

			for _, conversation := range v.Data.Conversations {
				if conversation.ID == nil {
					continue
				}

				chatID := *conversation.ID

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
				}
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

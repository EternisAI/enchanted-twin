package whatsapp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.mau.fi/whatsmeow/types/events"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	dataprocessing_whatsapp "github.com/EternisAI/enchanted-twin/pkg/dataprocessing/whatsapp"
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

func EventHandler(memoryStorage memory.Storage) func(interface{}) {
	return func(evt interface{}) {
		switch v := evt.(type) {

		case *events.HistorySync:
			fmt.Println("History sync event received")

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			for _, pushname := range v.Data.Pushnames {
				fmt.Println("pushname", *pushname.ID)
				fmt.Println("pushname", *pushname.Pushname)
				if pushname.ID != nil && pushname.Pushname != nil {
					err := dataprocessing_whatsapp.ProcessNewContact(ctx, memoryStorage, *pushname.ID, *pushname.Pushname)
					if err != nil {
						fmt.Println("Error processing WhatsApp contact:", err)
					} else {
						fmt.Println("WhatsApp contact stored successfully:", *pushname.Pushname)
					}
				}
			}

			for _, conversation := range v.Data.Conversations {
				if conversation.ID == nil {
					continue
				}

				chatID := *conversation.ID

				for _, messageInfo := range conversation.Messages {
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
					} else {
						fromName = chatID
					}

					fmt.Println("message", content)
					fmt.Println("fromName", fromName)
					fmt.Println("toName", toName)
					fmt.Println("timestamp", *message.MessageTimestamp)
					err := dataprocessing_whatsapp.ProcessHistoricalMessage(
						ctx,
						memoryStorage,
						content,
						fromName,
						toName,
						*message.MessageTimestamp,
					)
					if err != nil {
						fmt.Println("Error processing historical WhatsApp message:", err)
					} else {
						fmt.Println("Historical WhatsApp message stored successfully")
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
				fmt.Println("Received a message with empty content")
				return
			}

			fmt.Println("Received a message:", message)

			fromName := v.Info.PushName
			if fromName == "" {
				fromName = v.Info.Sender.User
			}

			toName := v.Info.Chat.User

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			err := dataprocessing_whatsapp.ProcessNewMessage(ctx, memoryStorage, message, fromName, toName)
			if err != nil {
				fmt.Println("Error processing WhatsApp message:", err)
			} else {
				fmt.Println("WhatsApp message stored successfully")
			}
		}
	}
}

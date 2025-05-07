package whatsapp

import (
	"sync"
)

// QRCodeEvent represents a QR code event from WhatsApp
type QRCodeEvent struct {
	Event string // "code" or other status
	Code  string // The QR code data for scanning
}

// Global QR channel for WhatsApp authentication
var (
	QRChan     chan QRCodeEvent
	QRChanOnce sync.Once

	// For storing latest event
	latestQREvent     *QRCodeEvent
	latestQREventLock sync.RWMutex

	// Connect channel
	ConnectChan     chan struct{}
	ConnectChanOnce sync.Once
)

// GetQRChannel returns the singleton QR channel instance
func GetQRChannel() chan QRCodeEvent {
	QRChanOnce.Do(func() {
		QRChan = make(chan QRCodeEvent, 100)
	})
	return QRChan
}

// GetLatestQREvent returns the latest QR code event
func GetLatestQREvent() *QRCodeEvent {
	latestQREventLock.RLock()
	defer latestQREventLock.RUnlock()
	return latestQREvent
}

// SetLatestQREvent updates the latest QR code event
func SetLatestQREvent(evt QRCodeEvent) {
	latestQREventLock.Lock()
	defer latestQREventLock.Unlock()
	latestQREvent = &evt
}

// GetConnectChannel returns the channel for connection signals
func GetConnectChannel() chan struct{} {
	ConnectChanOnce.Do(func() {
		ConnectChan = make(chan struct{}, 1)
	})
	return ConnectChan
}

// TriggerConnect sends a signal to start connection
func TriggerConnect() {
	GetConnectChannel() <- struct{}{}
}

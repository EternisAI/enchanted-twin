// Owner: august@eternis.ai
package bootstrap

import (
	"errors"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/log"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

const (
	NatsServerURL = "nats://127.0.0.1:4222"
)

func StartEmbeddedNATSServer(logger *log.Logger) (*server.Server, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil, errors.New("unable to get user cache directory")
	}
	storeDir := filepath.Join(cacheDir, "enchanted-twin", "nats")

	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		return nil, errors.New("unable to create NATS store directory")
	}
	logger.Debug("Using NATS store directory", "path", storeDir)

	opts := &server.Options{
		Port:      4222,
		Host:      "127.0.0.1",
		JetStream: true,
		StoreDir:  storeDir,
	}

	s, err := server.NewServer(opts)
	if err != nil {
		return nil, err
	}

	go s.Start()

	if !s.ReadyForConnections(5 * time.Second) {
		return nil, errors.New("NATS server not ready in time")
	}

	addr := s.Addr()
	tcpAddr, ok := addr.(*net.TCPAddr)
	if !ok {
		return nil, errors.New("unexpected address type")
	}

	logger.Info("Started NATS server", "port", tcpAddr.Port)
	return s, nil
}

func NewNatsClient() (*nats.Conn, error) {
	opts := []nats.Option{
		nats.ReconnectWait(2 * time.Second),
		nats.MaxReconnects(10),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				slog.Error("NATS disconnected", "error", err)
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			slog.Info("NATS reconnected", "url", nc.ConnectedUrl())
		}),
		nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error) {
			slog.Error("NATS error", "error", err)
		}),
	}
	return nats.Connect(NatsServerURL, opts...)
}

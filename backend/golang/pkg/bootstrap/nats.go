package bootstrap

import (
	"errors"
	"log/slog"

	"github.com/charmbracelet/log"

	"net"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

const (
	NatsServerURL = "nats://127.0.0.1:4222"
)

func StartEmbeddedNATSServer(logger *log.Logger) (*server.Server, error) {
	opts := &server.Options{
		Port:      4222,
		Host:      "127.0.0.1",
		JetStream: true,
		StoreDir:  "./nats",
	}

	s, err := server.NewServer(opts)
	if err != nil {
		return nil, err
	}

	go s.Start()

	if !s.ReadyForConnections(30 * time.Second) {
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

package bootstrap

import (
	"errors"
	"log"
	"net"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

const (
	NatsServerURL = "nats://127.0.0.1:4222"
)

func StartEmbeddedNATSServer() (*server.Server, error) {
	opts := &server.Options{}

	s, err := server.NewServer(opts)
	if err != nil {
		return nil, err
	}

	go s.Start()

	if !s.ReadyForConnections(10 * time.Second) {
		return nil, errors.New("NATS server not ready in time")
	}

	addr := s.Addr()
	tcpAddr, ok := addr.(*net.TCPAddr)
	if !ok {
		return nil, errors.New("unexpected address type")
	}

	log.Printf("Started NATS server on port: %d", tcpAddr.Port)
	return s, nil
}

func NewNatsClient() (*nats.Conn, error) {
	return nats.Connect(NatsServerURL)
}

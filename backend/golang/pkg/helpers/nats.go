package helpers

import (
	"encoding/json"

	"github.com/nats-io/nats.go"
)

func NatsPublish(nc *nats.Conn, subject string, payload any) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return nc.Publish(subject, payloadJSON)
}

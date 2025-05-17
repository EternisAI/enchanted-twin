package notifications

import (
	"context"

	"github.com/nats-io/nats.go"

	"github.com/EternisAI/enchanted-twin/graph/model"
	"github.com/EternisAI/enchanted-twin/pkg/helpers"
)

type Service struct {
	nc *nats.Conn
}

const AppNotificationsSubject = "notifications.app"

func NewService(nc *nats.Conn) *Service {
	return &Service{
		nc: nc,
	}
}

func (s *Service) SendNotification(ctx context.Context, notification *model.AppNotification) error {
	return helpers.NatsPublish(s.nc, AppNotificationsSubject, notification)
}

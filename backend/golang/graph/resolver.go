package graph

import (
	"log/slog"

	"go.temporal.io/sdk/client"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	Logger         *slog.Logger
	TemporalClient client.Client
}

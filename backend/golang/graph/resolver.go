package graph

import (
	"github.com/charmbracelet/log"

	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/workflows"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/pkg/twinchat"

	"github.com/nats-io/nats.go"
	"go.temporal.io/sdk/client"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	Logger                 *log.Logger
	TemporalClient         client.Client
	TwinChatService        twinchat.Service
	Nc                     *nats.Conn
	Store                  *db.Store
	AiService              *ai.Service
	DataProcessingWorkflow *workflows.DataProcessingWorkflows
}

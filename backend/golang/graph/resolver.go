package graph

import (
	"github.com/charmbracelet/log"
	"github.com/nats-io/nats.go"
	"go.temporal.io/sdk/client"

	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/workflows"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/pkg/holon"
	"github.com/EternisAI/enchanted-twin/pkg/mcpserver"
	"github.com/EternisAI/enchanted-twin/pkg/telegram"
	"github.com/EternisAI/enchanted-twin/pkg/twinchat"
	"github.com/EternisAI/enchanted-twin/pkg/whatsapp"
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
	MCPService             mcpserver.MCPService
	DataProcessingWorkflow *workflows.DataProcessingWorkflows
	TelegramService        *telegram.TelegramService
	HolonService           *holon.Service
	WhatsAppService        *whatsapp.Service
}

func (r *Resolver) GetWhatsAppQRCode() *string {
	if r.WhatsAppService != nil {
		return r.WhatsAppService.GetCurrentQRCode()
	}
	return nil
}

func (r *Resolver) GetWhatsAppConnected() bool {
	if r.WhatsAppService != nil {
		return r.WhatsAppService.IsConnected()
	}
	return false
}

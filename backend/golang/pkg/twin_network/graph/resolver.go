package graph

import (
	"github.com/charmbracelet/log"

	twinnetwork "github.com/EternisAI/enchanted-twin/pkg/twin_network"
)

// Resolver serves as dependency injection for your app, add any dependencies you
// require here.
type Resolver struct {
	Store  *twinnetwork.MessageStore
	Logger *log.Logger
}

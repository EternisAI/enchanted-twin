package graph

import (
	twinnetwork "github.com/EternisAI/enchanted-twin/pkg/twin_network"
	"github.com/charmbracelet/log"
)

// Resolver serves as dependency injection for your app, add any dependencies you
// require here.
type Resolver struct {
	Store  *twinnetwork.MessageStore
	Logger *log.Logger
}

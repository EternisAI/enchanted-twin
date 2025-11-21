package bootstrap

import (
	"github.com/EternisAI/enchanted-twin/pkg/logging"
)

// ComponentRegistry is deprecated. Use logging.ComponentRegistry instead.
type ComponentRegistry = logging.ComponentRegistry

// ComponentType is deprecated. Use logging.ComponentType instead.
type ComponentType = logging.ComponentType

// ComponentInfo is deprecated. Use logging.ComponentInfo instead.
type ComponentInfo = logging.ComponentInfo

// NewComponentRegistry is deprecated. Use logging.NewComponentRegistry instead.
func NewComponentRegistry() *ComponentRegistry {
	return logging.NewComponentRegistry()
}

// Re-export component type constants for backward compatibility.
const (
	ComponentTypeService     = logging.ComponentTypeService
	ComponentTypeManager     = logging.ComponentTypeManager
	ComponentTypeHandler     = logging.ComponentTypeHandler
	ComponentTypeResolver    = logging.ComponentTypeResolver
	ComponentTypeRepository  = logging.ComponentTypeRepository
	ComponentTypeWorker      = logging.ComponentTypeWorker
	ComponentTypeClient      = logging.ComponentTypeClient
	ComponentTypeServer      = logging.ComponentTypeServer
	ComponentTypeMiddleware  = logging.ComponentTypeMiddleware
	ComponentTypeUtility     = logging.ComponentTypeUtility
	ComponentTypeAI          = logging.ComponentTypeAI
	ComponentTypeAnonymizer  = logging.ComponentTypeAnonymizer
	ComponentTypeEmbedding   = logging.ComponentTypeEmbedding
	ComponentTypeCompletions = logging.ComponentTypeCompletions
	ComponentTypeProcessor   = logging.ComponentTypeProcessor
	ComponentTypeWorkflow    = logging.ComponentTypeWorkflow
	ComponentTypeIntegration = logging.ComponentTypeIntegration
	ComponentTypeParser      = logging.ComponentTypeParser
	ComponentTypeTelegram    = logging.ComponentTypeTelegram
	ComponentTypeWhatsApp    = logging.ComponentTypeWhatsApp
	ComponentTypeSlack       = logging.ComponentTypeSlack
	ComponentTypeGmail       = logging.ComponentTypeGmail
	ComponentTypeMCP         = logging.ComponentTypeMCP
	ComponentTypeDatabase    = logging.ComponentTypeDatabase
	ComponentTypeNATS        = logging.ComponentTypeNATS
	ComponentTypeTemporal    = logging.ComponentTypeTemporal
	ComponentTypeDirectory   = logging.ComponentTypeDirectory
	ComponentTypeIdentity    = logging.ComponentTypeIdentity
	ComponentTypeAuth        = logging.ComponentTypeAuth
	ComponentTypeOAuth       = logging.ComponentTypeOAuth
	ComponentTypeChat        = logging.ComponentTypeChat
	ComponentTypeMemory      = logging.ComponentTypeMemory
	ComponentTypeTwinChat    = logging.ComponentTypeTwinChat
	ComponentTypeTTS         = logging.ComponentTypeTTS
)

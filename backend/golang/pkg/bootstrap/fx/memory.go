package fx

import (
	"go.uber.org/fx"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/memory/evolvingmemory/storage"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/bootstrap"
	"github.com/EternisAI/enchanted-twin/pkg/config"
)

// MemoryModule provides memory-related services.
var MemoryModule = fx.Module("memory",
	fx.Provide(
		ProvideEvolvingMemory,
	),
)

// EvolvingMemoryResult provides evolving memory storage.
type EvolvingMemoryResult struct {
	fx.Out
	Memory          evolvingmemory.MemoryStorage
	MemoryInterface memory.Storage // Also provide the interface for backward compatibility
}

// ProvideEvolvingMemory creates evolving memory with all dependencies.
func ProvideEvolvingMemory(
	loggerFactory *bootstrap.LoggerFactory,
	envs *config.Config,
	completionsService *ai.Service,
	embeddingWrapper *storage.EmbeddingWrapper,
	storageInterface storage.Interface,
) (EvolvingMemoryResult, error) {
	logger := loggerFactory.ForMemory("evolving.memory")
	logger.Info("Creating evolving memory", "backend", envs.MemoryBackend)

	mem, err := evolvingmemory.New(evolvingmemory.Dependencies{
		Logger:             logger,
		Storage:            storageInterface,
		CompletionsService: completionsService,
		CompletionsModel:   envs.CompletionsModel,
		EmbeddingsWrapper:  embeddingWrapper,
	})
	if err != nil {
		logger.Error("Failed to create evolving memory", "error", err)
		return EvolvingMemoryResult{}, err
	}

	logger.Info("Evolving memory created successfully", "backend", envs.MemoryBackend)
	return EvolvingMemoryResult{
		Memory:          mem,
		MemoryInterface: mem, // MemoryStorage implements memory.Storage interface
	}, nil
}

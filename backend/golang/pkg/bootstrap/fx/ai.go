package fx

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
	"go.uber.org/fx"

	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/config"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/pkg/localmodel/jinaaiembedding"
	"github.com/EternisAI/enchanted-twin/pkg/localmodel/ollama"
	"github.com/EternisAI/enchanted-twin/pkg/microscheduler"
)

// AIModule provides all AI-related services.
var AIModule = fx.Module("ai",
	fx.Provide(
		ProvideFirebaseTokenGetter,
		ProvideAICompletionsService,
		ProvideAIEmbeddingsService,
		ProvideAnonymizerManager,
		ProvidePrivateCompletionsService,
	),
    fx.Invoke(LogAnonymizerStartupBanner),
    fx.Invoke(ActivateAnonymizerManager),
    fx.Invoke(ActivatePrivateCompletionsService),
)

// FirebaseTokenGetter provides token retrieval function.
type FirebaseTokenGetter func() (string, error)

// ProvideFirebaseTokenGetter creates Firebase token getter function.
func ProvideFirebaseTokenGetter(store *db.Store) FirebaseTokenGetter {
	return func() (string, error) {
		firebaseToken, err := store.GetOAuthTokens(context.Background(), "firebase")
		if err != nil {
			return "", err
		}
		if firebaseToken == nil {
			return "", fmt.Errorf("firebase token not found")
		}
		return firebaseToken.AccessToken, nil
	}
}

// AICompletionsServiceResult provides AI completions service.
type AICompletionsServiceResult struct {
	fx.Out
	CompletionsService *ai.Service
}

// ProvideAICompletionsService creates AI completions service based on configuration.
func ProvideAICompletionsService(
	logger *log.Logger,
	envs *config.Config,
	getFirebaseToken FirebaseTokenGetter,
) AICompletionsServiceResult {
	var aiCompletionsService *ai.Service

	if envs.ProxyTeeURL != "" {
		logger.Info("Using proxy tee url", "url", envs.ProxyTeeURL)
		aiCompletionsService = ai.NewOpenAIServiceProxy(logger, getFirebaseToken, envs.ProxyTeeURL, envs.CompletionsAPIURL)
	} else {
		aiCompletionsService = ai.NewOpenAIService(logger, envs.CompletionsAPIKey, envs.CompletionsAPIURL)
	}

	return AICompletionsServiceResult{CompletionsService: aiCompletionsService}
}

// AIEmbeddingsServiceResult provides AI embeddings service.
type AIEmbeddingsServiceResult struct {
	fx.Out
	EmbeddingsService ai.Embedding
}

// ProvideAIEmbeddingsService creates AI embeddings service (local or remote).
func ProvideAIEmbeddingsService(
	logger *log.Logger,
	envs *config.Config,
	getFirebaseToken FirebaseTokenGetter,
) (AIEmbeddingsServiceResult, error) {
	var aiEmbeddingsService ai.Embedding

	if envs.UseLocalEmbedding == "true" {
		logger.Info("Using local embedding model")
		sharedLibPath := filepath.Join(envs.AppDataPath, "shared", "lib")
		localEmbeddingModel, err := jinaaiembedding.NewEmbedding(envs.AppDataPath, sharedLibPath)
		if err != nil {
			logger.Error("Failed to create local embedding model", "error", err)
			return AIEmbeddingsServiceResult{}, err
		}
		aiEmbeddingsService = localEmbeddingModel
	} else {
		var openAIEmbeddingsService *ai.Service
		if envs.ProxyTeeURL != "" {
			logger.Info("Using proxy tee url for embeddings", "url", envs.ProxyTeeURL)
			openAIEmbeddingsService = ai.NewOpenAIServiceProxy(logger, getFirebaseToken, envs.ProxyTeeURL, envs.EmbeddingsAPIURL)
		} else {
			openAIEmbeddingsService = ai.NewOpenAIService(logger, envs.EmbeddingsAPIKey, envs.EmbeddingsAPIURL)
		}
		aiEmbeddingsService = openAIEmbeddingsService
	}

	return AIEmbeddingsServiceResult{EmbeddingsService: aiEmbeddingsService}, nil
}

// AnonymizerManagerResult provides anonymizer manager.
type AnonymizerManagerResult struct {
	fx.Out
	AnonymizerManager *ai.AnonymizerManager `optional:"true"`
	LocalAnonymizer   *ollama.OllamaClient  `optional:"true"`
}

// ProvideAnonymizerManager creates anonymizer manager based on configuration.
func ProvideAnonymizerManager(
	lc fx.Lifecycle,
	logger *log.Logger,
	envs *config.Config,
	store *db.Store,
	completionsService *ai.Service,
) AnonymizerManagerResult {
	// High-visibility banner for anonymizer configuration
	logger.Info("==================== ANONYMIZER ====================")
	if val, ok := os.LookupEnv("ANONYMIZER_TYPE"); ok {
		logger.Info("Environment variable", "ANONYMIZER_TYPE", val)
	} else {
		logger.Info("Environment variable", "ANONYMIZER_TYPE", "(unset)")
	}
	logger.Info("Effective anonymizer type", "type", envs.AnonymizerType)
	logger.Info("=====================================================")

	var anonymizerManager *ai.AnonymizerManager
	var localAnonymizer *ollama.OllamaClient

	switch envs.AnonymizerType {
	case "local":
		logger.Info("Using local anonymizer model")
		localAnonymizer = ollama.NewOllamaClient("http://localhost:11435", "qwen3-0.6b-q4_k_m", logger)
		if err := localAnonymizer.Ping(context.Background()); err != nil {
			logger.Error("Local anonymizer health check failed", "error", err)
		} else {
			logger.Info("Local anonymizer is reachable")
		}
		logger.Info("Local anonymizer model initialized successfully")
		anonymizerManager = ai.NewLocalAnonymizerManager(localAnonymizer, store.DB().DB, logger)

	case "mock":
		logger.Info("Using mock anonymizer for development/testing")
		anonymizerManager = ai.NewLLMAnonymizerManager(completionsService, envs.CompletionsModel, store.DB().DB, logger)

	case "llm":
		logger.Info("Using LLM anonymizer manager")
		anonymizerManager = ai.NewLLMAnonymizerManager(completionsService, envs.CompletionsModel, store.DB().DB, logger)

	case "no-op":
		fallthrough
	default:
		logger.Info("Anonymizer disabled (no-op mode)")
		anonymizerManager = nil
	}

	// Register cleanup hooks
	if localAnonymizer != nil {
		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				logger.Info("Closing local anonymizer")
				if err := localAnonymizer.Close(); err != nil {
					logger.Error("Error closing local anonymizer", "error", err)
					return err
				}
				return nil
			},
		})
	}

	if anonymizerManager != nil {
		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				logger.Info("Shutting down anonymizer manager")
				anonymizerManager.Shutdown()
				return nil
			},
		})
	}

	return AnonymizerManagerResult{
		AnonymizerManager: anonymizerManager,
		LocalAnonymizer:   localAnonymizer,
	}
}

// LogAnonymizerStartupBanner prints a clear, early banner with anonymizer config.
func LogAnonymizerStartupBanner(logger *log.Logger, envs *config.Config) {
    logger.Info("==================== ANONYMIZER ====================")
    if val, ok := os.LookupEnv("ANONYMIZER_TYPE"); ok {
        logger.Info("Environment variable", "ANONYMIZER_TYPE", val)
    } else {
        logger.Info("Environment variable", "ANONYMIZER_TYPE", "(unset)")
    }
    logger.Info("Effective anonymizer type", "type", envs.AnonymizerType)
    logger.Info("=====================================================")
}

// PrivateCompletionsServiceResult provides private completions service.
type PrivateCompletionsServiceResult struct {
	fx.Out
	PrivateCompletionsService *ai.PrivateCompletionsService `optional:"true"`
}

// PrivateCompletionsServiceParams holds parameters for private completions service.
type PrivateCompletionsServiceParams struct {
	fx.In
	Lifecycle          fx.Lifecycle
	Logger             *log.Logger
	CompletionsService *ai.Service
	AnonymizerManager  *ai.AnonymizerManager `optional:"true"`
}

// ProvidePrivateCompletionsService creates private completions service if anonymizer is enabled.
func ProvidePrivateCompletionsService(params PrivateCompletionsServiceParams) (PrivateCompletionsServiceResult, error) {
	// Create private completions service if anonymizer is enabled
	var privateCompletionsService *ai.PrivateCompletionsService

	if params.AnonymizerManager != nil {
		var err error
		privateCompletionsService, err = ai.NewPrivateCompletionsService(ai.PrivateCompletionsConfig{
			CompletionsService: params.CompletionsService,
			AnonymizerManager:  params.AnonymizerManager,
			ExecutorWorkers:    1,
			Logger:             params.Logger,
		})
		if err != nil {
			params.Logger.Error("Failed to create private completions service", "error", err)
			return PrivateCompletionsServiceResult{}, err
		}

		// Enable private completions in the AI service
		params.CompletionsService.EnablePrivateCompletions(privateCompletionsService, microscheduler.UI)
		params.Logger.Info("Private completions service enabled")

		// Register cleanup hook
		params.Lifecycle.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				params.Logger.Info("Shutting down private completions service")
				privateCompletionsService.Shutdown()
				return nil
			},
		})
	} else {
		params.Logger.Info("Private completions service disabled (no anonymizer)")
	}

	return PrivateCompletionsServiceResult{PrivateCompletionsService: privateCompletionsService}, nil
}

// ActivateAnonymizerManager forces construction of the anonymizer manager and logs its status.
func ActivateAnonymizerManager(logger *log.Logger, manager *ai.AnonymizerManager) {
    if manager != nil {
        logger.Info("Anonymizer manager initialized and active")
    } else {
        logger.Info("Anonymizer manager not initialized (no-op mode)")
    }
}

// ActivatePrivateCompletionsService forces construction of the private completions service and logs its status.
func ActivatePrivateCompletionsService(logger *log.Logger, service *ai.PrivateCompletionsService) {
    if service != nil {
        logger.Info("Private completions service enabled (activation log)")
    } else {
        logger.Info("Private completions service disabled (activation log)")
    }
}

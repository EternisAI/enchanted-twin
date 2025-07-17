package ai

import (
	"database/sql"
	"sync"
	"time"

	"github.com/charmbracelet/log"
)

type AnonymizerType int

const (
	NoOpAnonymizerType AnonymizerType = iota
	MockAnonymizerType
	LocalAnonymizerType
	PersistentAnonymizerType
	LLMAnonymizerType
)

type AnonymizerConfig struct {
	Type    AnonymizerType
	Enabled bool
	Delay   time.Duration
	Logger  *log.Logger

	// For PersistentAnonymizer
	Database *sql.DB

	// For LocalAnonymizer
	LlamaAnonymizer LlamaAnonymizerInterface

	// For MockAnonymizer
	PredefinedReplacements map[string]string

	// For LLMAnonymizer
	AIService CompletionsService
	Model     string
}

type AnonymizerManager struct {
	instance   Anonymizer
	once       sync.Once
	config     AnonymizerConfig
	shutdown   func()
	shutdownMu sync.Mutex
	isShutdown bool
}

func NewAnonymizerManager(config AnonymizerConfig) *AnonymizerManager {
	return &AnonymizerManager{
		config: config,
	}
}

func (m *AnonymizerManager) GetAnonymizer() Anonymizer {
	m.once.Do(func() {
		m.instance, m.shutdown = m.createAnonymizer()
	})
	return m.instance
}

func (m *AnonymizerManager) Shutdown() {
	m.shutdownMu.Lock()
	defer m.shutdownMu.Unlock()

	if m.isShutdown {
		return
	}

	if m.shutdown != nil {
		m.shutdown()
	}

	m.isShutdown = true
}

func (m *AnonymizerManager) createAnonymizer() (Anonymizer, func()) {
	logger := m.config.Logger
	if logger == nil {
		logger = log.New(nil)
	}

	switch m.config.Type {
	case MockAnonymizerType:
		if !m.config.Enabled {
			logger.Info("MockAnonymizer disabled, using no-op anonymizer")
			return NewNoOpAnonymizer(logger), nil
		}

		replacements := m.config.PredefinedReplacements
		if replacements == nil {
			replacements = getDefaultReplacements()
		}

		mockAnonymizer := NewMockAnonymizer(m.config.Delay, replacements, logger)

		logger.Info("MockAnonymizer created", "delay", m.config.Delay)

		return mockAnonymizer, func() {
			mockAnonymizer.Shutdown()
		}

	case PersistentAnonymizerType:
		if m.config.Database == nil {
			logger.Error("PersistentAnonymizer requires database, falling back to no-op")
			return NewNoOpAnonymizer(logger), nil
		}

		persistentAnonymizer := NewPersistentAnonymizer(m.config.Database, logger)
		logger.Info("PersistentAnonymizer created")

		return persistentAnonymizer, func() {
			persistentAnonymizer.Shutdown()
		}

	case LocalAnonymizerType:
		if m.config.LlamaAnonymizer == nil {
			logger.Error("LocalAnonymizer requires LlamaAnonymizer, falling back to no-op")
			return NewNoOpAnonymizer(logger), nil
		}
		if m.config.Database == nil {
			logger.Error("LocalAnonymizer requires database, falling back to no-op")
			return NewNoOpAnonymizer(logger), nil
		}

		localAnonymizer := NewLocalAnonymizer(m.config.LlamaAnonymizer, m.config.Database, logger)
		logger.Info("LocalAnonymizer created")

		return localAnonymizer, func() {
			localAnonymizer.Shutdown()
		}

	case LLMAnonymizerType:
		if m.config.AIService == nil {
			logger.Error("LLMAnonymizer requires AIService, falling back to no-op")
			return NewNoOpAnonymizer(logger), nil
		}
		if m.config.Database == nil {
			logger.Error("LLMAnonymizer requires database, falling back to no-op")
			return NewNoOpAnonymizer(logger), nil
		}

		model := m.config.Model
		if model == "" {
			model = "openai/gpt-4o-mini"
		}

		llmAnonymizer := NewLLMAnonymizer(m.config.AIService, model, m.config.Database, logger)
		logger.Info("LLMAnonymizer created", "model", model)

		return llmAnonymizer, func() {
			llmAnonymizer.Shutdown()
		}

	case NoOpAnonymizerType:
		fallthrough
	default:
		logger.Info("NoOpAnonymizer created")
		return NewNoOpAnonymizer(logger), nil
	}
}

func getDefaultReplacements() map[string]string {
	return map[string]string{
		// Full names (processed first due to length sorting)
		"John Smith":    "PERSON_001",
		"Jane Doe":      "PERSON_002",
		"Alice Johnson": "PERSON_003",

		// Common names
		"John":    "PERSON_001",
		"Jane":    "PERSON_002",
		"Alice":   "PERSON_003",
		"Bob":     "PERSON_004",
		"Charlie": "PERSON_005",
		"David":   "PERSON_006",
		"Emma":    "PERSON_007",
		"Frank":   "PERSON_008",

		// Company names
		"OpenAI":    "COMPANY_001",
		"Microsoft": "COMPANY_002",
		"Google":    "COMPANY_003",
		"Apple":     "COMPANY_004",
		"Tesla":     "COMPANY_005",
		"Amazon":    "COMPANY_006",

		// Locations
		"New York":      "LOCATION_001",
		"London":        "LOCATION_002",
		"Tokyo":         "LOCATION_003",
		"Paris":         "LOCATION_004",
		"Berlin":        "LOCATION_005",
		"San Francisco": "LOCATION_006",
	}
}

// Factory functions for common configurations

func NewMockAnonymizerManager(delay time.Duration, enabled bool, logger *log.Logger) *AnonymizerManager {
	return NewAnonymizerManager(AnonymizerConfig{
		Type:    MockAnonymizerType,
		Enabled: enabled,
		Delay:   delay,
		Logger:  logger,
	})
}

func NewPersistentAnonymizerManager(db *sql.DB, logger *log.Logger) *AnonymizerManager {
	return NewAnonymizerManager(AnonymizerConfig{
		Type:     PersistentAnonymizerType,
		Database: db,
		Logger:   logger,
	})
}

func NewLocalAnonymizerManager(llama LlamaAnonymizerInterface, db *sql.DB, logger *log.Logger) *AnonymizerManager {
	return NewAnonymizerManager(AnonymizerConfig{
		Type:            LocalAnonymizerType,
		LlamaAnonymizer: llama,
		Database:        db,
		Logger:          logger,
	})
}

func NewLLMAnonymizerManager(aiService CompletionsService, model string, db *sql.DB, logger *log.Logger) *AnonymizerManager {
	return NewAnonymizerManager(AnonymizerConfig{
		Type:      LLMAnonymizerType,
		AIService: aiService,
		Model:     model,
		Database:  db,
		Logger:    logger,
	})
}

func NewNoOpAnonymizerManager(logger *log.Logger) *AnonymizerManager {
	return NewAnonymizerManager(AnonymizerConfig{
		Type:   NoOpAnonymizerType,
		Logger: logger,
	})
}

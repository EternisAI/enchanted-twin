// Owner: slimane@eternis.ai

// Package engagement provides services for managing automated friend interactions
// and user engagement activities. It orchestrates AI-driven conversations, memory
// sharing, and proactive messaging through Temporal workflows to maintain
// meaningful connections with users.

package engagement

import (
	"time"

	"github.com/charmbracelet/log"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/EternisAI/enchanted-twin/pkg/agent/memory"
	"github.com/EternisAI/enchanted-twin/pkg/agent/tools"
	"github.com/EternisAI/enchanted-twin/pkg/ai"
	"github.com/EternisAI/enchanted-twin/pkg/db"
	"github.com/EternisAI/enchanted-twin/pkg/helpers"
	"github.com/EternisAI/enchanted-twin/pkg/identity"
	"github.com/EternisAI/enchanted-twin/pkg/twinchat"
)

const (
	MinWaitSeconds = 60 * 60 * 0.5
	MaxWaitSeconds = 60 * 60 * 1
)

type ActivityType string

const (
	ActivityTypePokeMessage   ActivityType = "poke_message"
	ActivityTypeMemoryPicture ActivityType = "memory_picture"
	ActivityTypeQuestion      ActivityType = "question"
)

type FriendService struct {
	aiService       *ai.Service
	logger          *log.Logger
	identityService *identity.IdentityService
	twinchatService *twinchat.Service
	memoryService   memory.Storage
	toolRegistry    tools.ToolRegistry
	store           *db.Store
}

type FriendServiceConfig struct {
	AiService       *ai.Service
	Logger          *log.Logger
	IdentityService *identity.IdentityService
	TwinchatService *twinchat.Service
	MemoryService   memory.Storage
	ToolRegistry    tools.ToolRegistry
	Store           *db.Store
}

func NewFriendService(config FriendServiceConfig) *FriendService {
	return &FriendService{
		aiService:       config.AiService,
		logger:          config.Logger,
		identityService: config.IdentityService,
		twinchatService: config.TwinchatService,
		memoryService:   config.MemoryService,
		toolRegistry:    config.ToolRegistry,
		store:           config.Store,
	}
}

func (s *FriendService) RegisterWorkflowsAndActivities(worker *worker.Worker, temporalClient client.Client) {
	(*worker).RegisterWorkflow(s.FriendWorkflow)
	(*worker).RegisterActivity(s.FetchMemory)
	(*worker).RegisterActivity(s.FetchRandomMemory)
	(*worker).RegisterActivity(s.FetchIdentity)
	(*worker).RegisterActivity(s.GeneratePokeMessage)
	(*worker).RegisterActivity(s.SendPokeMessage)
	(*worker).RegisterActivity(s.GenerateMemoryPicture)
	(*worker).RegisterActivity(s.SendMemoryPicture)
	(*worker).RegisterActivity(s.TrackUserResponse)
	(*worker).RegisterActivity(s.GenerateRandomWait)
	(*worker).RegisterActivity(s.SelectRandomActivity)
	(*worker).RegisterActivity(s.StoreSentMessage)
	(*worker).RegisterActivity(s.CheckForSimilarFriendMessages)
	(*worker).RegisterActivity(s.GetRandomQuestion)
	(*worker).RegisterActivity(s.SendQuestion)

	err := helpers.CreateScheduleIfNotExists(s.logger, temporalClient, "friend-workflow", 1*time.Hour, s.FriendWorkflow, []any{&FriendWorkflowInput{}})
	if err != nil {
		s.logger.Error("Failed to create friend workflow schedule", "error", err)
	}
}

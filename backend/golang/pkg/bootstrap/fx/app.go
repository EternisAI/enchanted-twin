package fx

import (
	"go.uber.org/fx"
)

// AppModule combines all modules for the complete application.
var AppModule = fx.Options(
	// Infrastructure layer - foundational services
	InfrastructureModule,

	// AI layer - AI services and embeddings
	AIModule,

	// Database layer - storage backends and memory
	DatabaseModule,
	MemoryModule,

	// Tools layer - tool registry and core tools
	ToolsModule,

	// Temporal layer - workflow orchestration
	TemporalModule,

	// Services layer - application services
	ServicesModule,

	// Server layer - HTTP GraphQL server
	ServerModule,
)

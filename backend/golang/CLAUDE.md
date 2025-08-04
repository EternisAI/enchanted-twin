# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

### Building and Running
- `make run` - Start the server (runs `cmd/server/main.go`)
- `make build` - Build the binary to `bin/enchanted-twin`
- `make release` - Build cross-platform binaries for Darwin (amd64/arm64) and Linux (amd64)
- `go run cmd/telegram_chat_server/main.go` or `make run-telegram-chat` - Run Telegram chat server

### Code Quality
- `make lint` - Format and lint code using golangci-lint with auto-fix
- `make test` - Run unit tests (short mode)
- `make test-integration` - Run integration tests for data processing (90min timeout)
- `make deadcode` - Check for transitively dead functions

### Code Generation
- `make gqlgen` - Generate GraphQL code from schema
- `make sqlc-generate` - Generate SQL code from queries and schema

### Database Management
- `make fresh-db` - Remove all Weaviate and SQLite data for a fresh start

### Installation
- `make install` - Install required tools (gqlgen, mockery, golangci-lint)

## Architecture Overview

This is a Go backend for an AI agent system called "Enchanted Twin" that processes various data sources and provides intelligent memory and interaction capabilities.

### Core Components

**Agent System (`pkg/agent/`)**
- Memory management with evolving memory engine (`pkg/agent/memory/evolvingmemory/`)
- Tool registry and execution system
- Notification handling
- Temporal workflow integration for scheduling

**Data Processing (`pkg/dataprocessing/`)**
- Multi-source data ingestion: WhatsApp, Telegram, Gmail, Slack, X/Twitter, ChatGPT exports
- Integration tests with sample data in `testdata/`
- Memory conversation processing pipeline

**AI Integration (`pkg/ai/`)**
- OpenAI API client (custom fork: `github.com/EternisAI/openai-go`)
- Message handling and streaming
- Utility functions for AI interactions

**Storage Layer**
- **Memory Backend** (configurable via `MEMORY_BACKEND`):
  - **PostgreSQL + pgvector** (default): Embedded PostgreSQL with vector search capabilities
  - **Weaviate**: Embedded vector database server (starts on `WEAVIATE_PORT`, default 51414)
- **SQLite**: Relational data with multiple schemas (config, holons, whatsapp)
- **SQLC**: Type-safe SQL code generation from queries

**Communication Services**
- **GraphQL**: API layer using gqlgen
- **Temporal**: Workflow orchestration for agent tasks
- **NATS**: Message broker for internal communication
- **MCP (Model Context Protocol)**: Server implementations for various integrations

### Key Directories

- `cmd/` - Entry points (main server, memory processor test, telegram chat, tee-api)
- `pkg/` - Core packages organized by domain
- `graph/` - GraphQL schema and resolvers
- `pkg/db/` - Database layer with migrations, queries, and generated code

### Technology Stack

- **Go 1.24.2** with extensive dependency list
- **Temporal** for workflow orchestration
- **Weaviate** for vector storage
- **SQLite** for relational data
- **GraphQL** via gqlgen
- **NATS** for messaging
- **Testcontainers** for integration testing

### Development Patterns

- Repository pattern per feature (avoid monolithic interfaces)
- Temporal workflows for async processing
- Type-safe database queries via SQLC
- Integration testing with real data samples
- Embedded services (Weaviate) for simplified deployment

### Testing

- Unit tests: `make test`
- Integration tests: `TEST_SOURCE=misc TEST_INPUT_PATH=testdata/misc make test-integration`
- Sample data available in `pkg/dataprocessing/integration/testdata/`

### Environment Variables

- `MEMORY_BACKEND` - Memory storage backend: `postgresql` (default) or `weaviate`
- `POSTGRES_PORT` - Port for embedded PostgreSQL server (default: 5432)
- `WEAVIATE_PORT` - Port for embedded Weaviate server (default: 51414)
- `DEBUG_CONFIG_PRINT` - Enable environment variable logging during config load (default: false)
- Various OAuth and API credentials for external services

#### Configuration Debugging

The application supports detailed logging of environment variables during configuration loading for debugging purposes:

- **Enable**: Set `DEBUG_CONFIG_PRINT=true` to enable environment variable logging
- **Security**: Sensitive values (API keys, tokens, passwords) are automatically masked in logs
- **Format**: Logs show `ENV: VARIABLE_NAME = value` for set variables and `ENV: VARIABLE_NAME = default_value (default)` for defaults
- **Purpose**: Useful for troubleshooting configuration issues in development and deployment

Example:
```bash
# Enable config debugging
export DEBUG_CONFIG_PRINT=true
make run

# Output will include:
# ENV: COMPLETIONS_API_URL = https://api.openai.com/v1
# ENV: COMPLETIONS_API_KEY = sk-p***masked***xyz7 
# ENV: GRAPHQL_PORT = 44999 (default)
```

### Code Style Notes

- Do not add comments for function definitions
- Code should be self-sufficient unless non-trivial
- Use existing repository patterns when adding new features
- Always run `make build` to verify code compiles
- Use `make lint` before committing changes
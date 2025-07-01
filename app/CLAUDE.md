# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

### Core Development

```bash
pnpm dev                    # Start development with hot reload
pnpm build                  # Full build with typecheck
pnpm typecheck              # Run TypeScript checks for both Node and web
pnpm lint                   # Run ESLint with cache
pnpm format                 # Format code with Prettier
```

### GraphQL Code Generation

```bash
pnpm codegen               # Generate GraphQL types from operations.gql
```

### Platform-Specific Builds

```bash
pnpm build:mac             # Build and publish macOS app
pnpm build-local:mac       # Build macOS app locally (no publish)
pnpm build:win             # Build Windows app
pnpm build:linux           # Build Linux app
```

### Testing & Development

Before committing changes, always run `pnpm typecheck` and `pnpm lint` to ensure code quality.

## Architecture Overview

This is a sophisticated Electron application with multi-language runtime coordination:

### Technology Stack

- **Frontend**: React 19, TypeScript, TanStack Router, Framer Motion, Tailwind CSS 4
- **Desktop**: Electron with Vite build system
- **Backend**: Go server with GraphQL API and SQLite database
- **Voice/AI**: Python LiveKit agents with conda-like environment management
- **Screen Capture**: Screenpipe integration for context awareness
- **UI Components**: shadcn/ui with Radix UI primitives

### Multi-Process Architecture

**Main Process** (`src/main/`):

- Orchestrates Go backend server (localhost:44999)
- Manages Python LiveKit voice agents with UV package manager
- Handles Screenpipe screen recording integration
- Provides comprehensive IPC API for system integration

**Renderer Process** (`src/renderer/src/`):

- File-based routing with hash history for Electron compatibility
- Apollo GraphQL client with HTTP/WebSocket split links
- Zustand stores for local state management
- Framer Motion animations with layout-aware transitions

### Key Service Integrations

- **Go Backend**: GraphQL API server with real-time subscriptions
- **LiveKit Agents**: Python-based voice processing with bidirectional IPC
- **Screenpipe**: Cross-platform screen recording for AI context
- **MCP Servers**: OAuth-based external AI service connections

### State Management Strategy

1. **Apollo Client**: Server state, caching, and real-time GraphQL subscriptions
2. **Zustand Stores**: UI state (sidebar, omnibar, onboarding, voice preferences)
3. **Electron Store**: Cross-process configuration persistence

## Code Conventions

Follow the established patterns from `.cursor/rules/common.mdc`:

### React/TypeScript

- Minimal comments - code should be self-documenting
- Use existing shadcn/ui components before creating new ones
- Prefer Tailwind CSS classes with semantic CSS variables
- Include both light and dark mode classes for colors
- Use Framer Motion for animations (avoid setTimeout)
- Use Lucide icons consistently

### Component Organization

- **Components**: Organized by feature area (chat/, holon/, settings/, etc.)
- **Routes**: File-based routing matching TanStack Router conventions
- **Hooks**: Custom hooks in dedicated hooks directory
- **Stores**: Zustand stores in lib/stores with localStorage persistence

### IPC Patterns

All IPC communication follows typed interfaces:

```typescript
// Service control patterns
;'service:action' | 'service:get-state' | 'service:get-status'

// Real-time event streaming
window.api.onServiceEvent((data) => handleUpdate(data))
```

## Development Workflow

### Adding New Features

1. Check existing components and patterns in the codebase
2. Use GraphQL codegen for any new backend operations
3. Follow the established state management patterns
4. Ensure proper error handling for service integrations
5. Test cross-platform compatibility (services run on Windows/macOS/Linux)

### Working with Services

- **Go Server**: Modify backend schema and run codegen
- **LiveKit Agents**: Python changes require understanding UV package management
- **Screenpipe**: Platform-specific binary management
- **IPC Communication**: Update both main and renderer type definitions

### UI Development

- The app has a custom title bar with `.titlebar` class for window dragging
- Responsive design with animated sidebar and omnibar
- Settings pages use different layout from main chat interface
- Global providers wrap the entire app (Theme, TTS, Apollo, GoLogs)

## Project Structure Highlights

```
src/main/                  # Electron main process
├── index.ts              # App entry point with service coordination
├── goServer.ts           # Go backend management
├── livekitManager.ts     # Python agent orchestration
├── pythonManager.ts      # UV package manager integration
├── screenpipe.ts         # Screen recording service
└── ipcHandlers.ts        # Comprehensive IPC API

src/renderer/src/         # React application
├── main.tsx             # App root with providers
├── routes/              # File-based routing
├── components/          # Feature-organized components
├── hooks/               # Custom React hooks
├── lib/stores/          # Zustand state management
└── graphql/             # Apollo client and codegen
```

## Troubleshooting

### Common Issues

- **Build Failures**: Run `pnpm typecheck` first to identify TypeScript issues
- **Service Startup**: Check logs in main process for Go server or Python agent failures
- **GraphQL Errors**: Ensure Go backend is running and `pnpm codegen` is up to date
- **IPC Issues**: Verify type definitions match between main and renderer processes

### Development Dependencies

- Uses `pnpm` package manager with workspace optimizations
- Electron builds require platform-specific native dependencies
- Python agents require UV for package management
- Go server requires Go toolchain for backend development

# AGENTS

The codebase consists of Electron app and go backend.

### Electron

Electron handles the following responsibilities

- UI
- Downloading and run Python models like Kokoro
- Spawn go backend binary and communicate via GraphQL API
- Spawn Screenpipe binary
- Communicate with WebRTC server in Go

### Go

- Handle database (SQLLite and Weaviate)
- Execute and schedule agent tasks
- Hadle chat with twin
- Handle MCP servers
- Handle OAuth (Google, X, Slack)
- Handle Telegram
- Handle GraphQL resolvers

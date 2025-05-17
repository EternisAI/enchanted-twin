# AGENTS

The codebase consists of Electron app and go backend.

## Electron

Electron handles the following responsibilities

- UI
- Downloading and run Python models like Kokoro
- Spawn go backend binary and communicate via GraphQL API
- Spawn Screenpipe binary
- Communicate with WebRTC server in Go

## Go

- Handle database (SQLLite and Weaviate)
- Execute and schedule agent tasks
- Hadle chat with twin
- Handle MCP servers
- Handle OAuth (Google, X, Slack)
- Handle Telegram
- Handle GraphQL resolvers

Requirements for Go

### Package guide

- Each package should have a clear purpose or responsibility.
  - Each package should have a "main" file named the same as the package itself.
  - Example: the "main" file of package `agi` is `pkg/agi/agi.go`.
  - Document the package itself in the "main" file.
  - Each package has an owner responsible for maintaining the package and ensuring adherence to this guide.
  - Every PR should add all owners of packages which are modified.
- Each package should have a clean and well-defined API.
  - The API consists of all exported (uppercase) identifiers.
  - All exported identifiers should be in the "main" file.
  - All exported identifiers must be documented.
  - Unexported identifiers do not need to be documented.
  - [The bigger the interface, the weaker the abstraction](https://go-proverbs.github.io/).
- The main functionality of a package should have a suitable level of testing.
- Don't use subpackages, except for internal packages.

Here is the suggested tree structure for the package `agi`.

```
pkg/
└── agi/
    ├── internal/
    │   ├── model/
    │   │   ├── model.go
    │   │   └── model_test.go
    │   └── tokenizer/
    │       ├── tokenizer.go
    │       └── tokenizer_test.go
    ├── preprocess.go
    ├── agi.go       # <==== MAIN FILE - PUBLIC API - DOCUMENTED
    └── agi_test.go
```

### Keep code easy to follow

- Write code in a direct style, making it easy to understand by reading and following function calls.
- Use Go channels when appropriate, but don't overuse them. Channels should be used for communication between Goroutines when it simplifies the code.
- Use Goroutines for concurrency, but avoid creating Goroutines that persist beyond the function's execution whenever possible. Bugs in background or long-running tasks are difficult to diagnose and debug.
  - Instead, centralize the creation of all Goroutines that must perform background tasks (timers, imports, etc.) in the `main` function. This approach makes it easier to monitor and control these tasks.

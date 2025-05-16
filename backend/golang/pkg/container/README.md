# Podman Package

This package provides Go interfaces for managing Podman containers in the Enchanted Twin application.

## Features

- Verify if Podman is installed
- Check if a Podman machine exists and is running
- Pull Docker images
- Run containers with customizable configurations

## Usage

### Basic Usage

```go
import (
    "context"
    "github.com/EternisAI/enchanted-twin/pkg/podman"
)

func main() {
    // Create a context
    ctx := context.Background()
    
    // Create a new Podman manager
    manager := podman.NewManager()
    
    // Check if Podman is installed
    installed, err := manager.IsInstalled(ctx)
    if err != nil {
        // Handle error
    }
    
    // Check if a Podman machine is running
    running, err := manager.IsMachineRunning(ctx)
    if err != nil {
        // Handle error
    }
    
    // Pull an image
    err = manager.PullImage(ctx, "docker.io/library/alpine:latest")
    if err != nil {
        // Handle error
    }
    
    // Run a container
    containerConfig := podman.ContainerConfig{
        ImageURL: "docker.io/library/alpine:latest",
        Name: "my-alpine-container",
        Command: []string{"echo", "Hello from Podman!"},
        Ports: map[string]string{
            "8080": "80",
        },
        Environment: map[string]string{
            "ENV_VAR": "value",
        },
        PullIfNeeded: true,
    }
    
    containerID, err := manager.RunContainer(ctx, containerConfig)
    if err != nil {
        // Handle error
    }
}
```

### Using the Kokoro Manager

For specialized Kokoro container management:

```go
import (
    "context"
    "github.com/EternisAI/enchanted-twin/pkg/podman"
)

func main() {
    // Create a context
    ctx := context.Background()
    
    // Create a Kokoro manager
    kokoroManager := podman.NewKokoroManager()
    
    // Verify Podman installation
    installed, _ := kokoroManager.VerifyPodmanInstalled(ctx)
    
    // Pull the Kokoro image
    err := kokoroManager.PullKokoroImage(ctx)
    if err != nil {
        // Handle error
    }
    
    // Run the Kokoro container on port 8080
    containerID, err := kokoroManager.RunKokoroContainer(ctx, "8080")
    if err != nil {
        // Handle error
    }
}
```

## CLI Usage

A command-line tool is available in `cmd/podman/main.go` that demonstrates usage of this package:

```bash
# Check Podman status
go run cmd/podman/main.go -action status

# Pull the Kokoro image
go run cmd/podman/main.go -action pull

# Run the Kokoro container
go run cmd/podman/main.go -action run -port 8080
```

## Integration with Electron

This package is designed to be used from the Go backend of the Enchanted Twin application. The Electron frontend communicates with this backend via API calls. 
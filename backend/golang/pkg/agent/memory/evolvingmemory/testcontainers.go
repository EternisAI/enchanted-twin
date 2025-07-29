package evolvingmemory

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/log"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestContainerSuite manages test containers for embedding and completion services.
type TestContainerSuite struct {
	logger              *log.Logger
	jinaContainer       testcontainers.Container
	onnxContainer       testcontainers.Container
	completionContainer testcontainers.Container
	JinaEndpoint        string
	ONNXEndpoint        string
	CompletionEndpoint  string
}

// SetupTestContainers initializes all required containers for testing.
func SetupTestContainers(ctx context.Context, logger *log.Logger) (*TestContainerSuite, error) {
	suite := &TestContainerSuite{
		logger: logger,
	}

	// Setup JinaAI embeddings container
	if err := suite.setupJinaContainer(ctx); err != nil {
		logger.Warn("Failed to setup Jina container", "error", err)
		return nil, fmt.Errorf("jina container setup failed: %w", err)
	}

	// Setup ONNX Runtime container
	if err := suite.setupONNXContainer(ctx); err != nil {
		logger.Warn("Failed to setup ONNX container", "error", err)
		_ = suite.Cleanup(ctx) // Clean up Jina container if ONNX fails
		return nil, fmt.Errorf("onnx container setup failed: %w", err)
	}

	// Skip completion container setup for now to focus on embeddings
	suite.CompletionEndpoint = "http://localhost:8002" // Placeholder
	logger.Info("Skipping completion container setup - focusing on embeddings")

	logger.Info("Test containers ready",
		"jina_endpoint", suite.JinaEndpoint,
		"onnx_endpoint", suite.ONNXEndpoint,
		"completion_endpoint", suite.CompletionEndpoint,
	)

	return suite, nil
}

// setupJinaContainer creates and starts the JinaAI embeddings container.
func (tc *TestContainerSuite) setupJinaContainer(ctx context.Context) error {
	tc.logger.Info("Setting up JinaAI embeddings container")

	// Use official JinaAI embedding server with the actual model
	jinaImage := getEnvOrDefault("TESTCONTAINER_JINA_IMAGE", "python:3.11-slim")
	startupTimeout := getTimeoutFromEnv("CONTAINER_STARTUP_TIMEOUT", 5*time.Minute)

	req := testcontainers.ContainerRequest{
		Image:        jinaImage,
		ExposedPorts: []string{"8000/tcp"},
		WaitingFor: wait.ForAll(
			wait.ForLog("Uvicorn running").WithStartupTimeout(startupTimeout),
			wait.ForHTTP("/health").WithPort("8000/tcp").WithStartupTimeout(startupTimeout),
		),
		Env: map[string]string{
			"PYTHONUNBUFFERED": "1",
		},
		Cmd: []string{
			"bash", "-c", `
# Install required packages
pip install --no-cache-dir fastapi uvicorn sentence-transformers torch numpy

# Create the embedding server
cat > embedding_server.py << 'EOF'
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from sentence_transformers import SentenceTransformer
from typing import List, Union
import uvicorn
import numpy as np

app = FastAPI()

# Load JinaAI embedding model
print("Loading JinaAI embedding model...")
model = SentenceTransformer('jinaai/jina-embeddings-v2-base-en')
print(f"Model loaded. Embedding dimension: {model.get_sentence_embedding_dimension()}")

class EmbeddingRequest(BaseModel):
    input: Union[str, List[str]]
    model: str = "jina-embeddings-v2-base-en"
    task: str = "retrieval.query"

class EmbeddingData(BaseModel):
    embedding: List[float]
    index: int

class EmbeddingResponse(BaseModel):
    data: List[EmbeddingData]
    model: str
    usage: dict

@app.get("/health")
async def health():
    return {"status": "healthy"}

@app.post("/v1/embeddings")
async def create_embeddings(request: EmbeddingRequest):
    try:
        # Handle both string and list inputs
        texts = request.input if isinstance(request.input, list) else [request.input]
        
        # Generate embeddings using JinaAI model
        embeddings = model.encode(texts).tolist()
        
        # Format response to match OpenAI API
        data = [
            EmbeddingData(embedding=emb, index=i) 
            for i, emb in enumerate(embeddings)
        ]
        
        return EmbeddingResponse(
            data=data,
            model=request.model,
            usage={
                "prompt_tokens": sum(len(text.split()) for text in texts),
                "total_tokens": sum(len(text.split()) for text in texts)
            }
        )
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=8000)
EOF

echo "Starting JinaAI embedding server..."
python embedding_server.py
`,
		},
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return fmt.Errorf("failed to start jina container: %w", err)
	}

	tc.jinaContainer = container

	// Get the mapped port and construct endpoint
	host, err := container.Host(ctx)
	if err != nil {
		return fmt.Errorf("failed to get jina container host: %w", err)
	}

	port, err := container.MappedPort(ctx, "8000")
	if err != nil {
		return fmt.Errorf("failed to get jina container port: %w", err)
	}

	tc.JinaEndpoint = fmt.Sprintf("http://%s:%s", host, port.Port())
	tc.logger.Info("JinaAI container ready", "endpoint", tc.JinaEndpoint)

	return nil
}

// setupONNXContainer creates and starts the ONNX Runtime container.
func (tc *TestContainerSuite) setupONNXContainer(ctx context.Context) error {
	tc.logger.Info("Setting up ONNX Runtime container")

	// Get configuration from environment - use a simple mock ONNX server
	onnxImage := getEnvOrDefault("TESTCONTAINER_ONNX_IMAGE", "python:3.11-slim")
	startupTimeout := getTimeoutFromEnv("CONTAINER_STARTUP_TIMEOUT", 2*time.Minute)

	req := testcontainers.ContainerRequest{
		Image:        onnxImage,
		ExposedPorts: []string{"8001/tcp"},
		WaitingFor: wait.ForAll(
			wait.ForLog("Starting mock ONNX server").WithStartupTimeout(startupTimeout),
			wait.ForHTTP("/v2/health/ready").WithPort("8001/tcp").WithStartupTimeout(startupTimeout),
		),
		Env: map[string]string{
			"PYTHONUNBUFFERED": "1",
		},
		Cmd: []string{
			"python", "-c", `
import http.server
import socketserver
import json

class MockONNXHandler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == '/v2/health/ready':
            self.send_response(200)
            self.send_header('Content-type', 'application/json')
            self.end_headers()
            self.wfile.write(json.dumps({"status": "ready"}).encode())
        elif self.path == '/v2/models':
            self.send_response(200)
            self.send_header('Content-type', 'application/json')
            self.end_headers()
            models = {
                "models": [
                    {"name": "jina-embeddings-v2-base-en", "versions": ["1"], "platform": "onnx"},
                    {"name": "mock-embedding-model", "versions": ["1"], "platform": "onnx"}
                ]
            }
            self.wfile.write(json.dumps(models).encode())
        else:
            self.send_response(404)
            self.end_headers()

print("Starting mock ONNX server on port 8001...")
with socketserver.TCPServer(("", 8001), MockONNXHandler) as httpd:
    httpd.serve_forever()
`,
		},
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return fmt.Errorf("failed to start onnx container: %w", err)
	}

	tc.onnxContainer = container

	// Get the mapped port and construct endpoint
	host, err := container.Host(ctx)
	if err != nil {
		return fmt.Errorf("failed to get onnx container host: %w", err)
	}

	port, err := container.MappedPort(ctx, "8001")
	if err != nil {
		return fmt.Errorf("failed to get onnx container port: %w", err)
	}

	tc.ONNXEndpoint = fmt.Sprintf("http://%s:%s", host, port.Port())
	tc.logger.Info("ONNX Runtime container ready", "endpoint", tc.ONNXEndpoint)

	return nil
}

// Cleanup terminates all containers.
func (tc *TestContainerSuite) Cleanup(ctx context.Context) error {
	var errs []error

	if tc.jinaContainer != nil {
		tc.logger.Info("Terminating JinaAI container")
		if err := tc.jinaContainer.Terminate(ctx); err != nil {
			tc.logger.Error("Failed to terminate jina container", "error", err)
			errs = append(errs, fmt.Errorf("jina cleanup failed: %w", err))
		}
	}

	if tc.onnxContainer != nil {
		tc.logger.Info("Terminating ONNX container")
		if err := tc.onnxContainer.Terminate(ctx); err != nil {
			tc.logger.Error("Failed to terminate onnx container", "error", err)
			errs = append(errs, fmt.Errorf("onnx cleanup failed: %w", err))
		}
	}

	if tc.completionContainer != nil {
		tc.logger.Info("Terminating completion container")
		if err := tc.completionContainer.Terminate(ctx); err != nil {
			tc.logger.Error("Failed to terminate completion container", "error", err)
			errs = append(errs, fmt.Errorf("completion cleanup failed: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %v", errs)
	}

	tc.logger.Info("All test containers cleaned up successfully")
	return nil
}

// IsHealthy checks if all containers are healthy and responsive.
func (tc *TestContainerSuite) IsHealthy(ctx context.Context) error {
	if tc.jinaContainer == nil || tc.onnxContainer == nil {
		return fmt.Errorf("required containers not initialized")
	}

	// Check if containers are still running
	jinaState, err := tc.jinaContainer.State(ctx)
	if err != nil {
		return fmt.Errorf("failed to get jina container state: %w", err)
	}
	if !jinaState.Running {
		return fmt.Errorf("jina container is not running")
	}

	onnxState, err := tc.onnxContainer.State(ctx)
	if err != nil {
		return fmt.Errorf("failed to get onnx container state: %w", err)
	}
	if !onnxState.Running {
		return fmt.Errorf("onnx container is not running")
	}

	completionState, err := tc.completionContainer.State(ctx)
	if err != nil {
		return fmt.Errorf("failed to get completion container state: %w", err)
	}
	if !completionState.Running {
		return fmt.Errorf("completion container is not running")
	}

	tc.logger.Debug("All containers are healthy")
	return nil
}

// getEnvOrDefault returns environment variable value or default.
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getTimeoutFromEnv parses timeout from environment variable.
func getTimeoutFromEnv(key string, defaultTimeout time.Duration) time.Duration {
	if timeoutStr := os.Getenv(key); timeoutStr != "" {
		if timeout, err := time.ParseDuration(timeoutStr); err == nil {
			return timeout
		}
	}
	return defaultTimeout
}

// shouldUseTestContainers determines if testcontainers should be used.
func shouldUseTestContainers() bool {
	return getEnvOrDefault("USE_TESTCONTAINERS", "true") == "true"
}

// isDockerAvailable checks if Docker is available for testcontainers.
func isDockerAvailable(ctx context.Context) bool {
	provider, err := testcontainers.NewDockerProvider()
	if err != nil {
		return false
	}
	defer func() { _ = provider.Close() }()

	// Use the correct API to check Docker availability
	_, err = provider.CreateContainer(ctx, testcontainers.ContainerRequest{
		Image:      "hello-world:latest",
		SkipReaper: true,
	})

	// If we can create a container request, Docker is available
	return err == nil
}

package coreml

import (
	"github.com/openai/openai-go"
)

type OperationType string

const (
	OperationCompletion OperationType = "completion"
	OperationEmbedding  OperationType = "embedding"
)

type Request struct {
	Operation OperationType `json:"operation"`
	Model     string        `json:"model,omitempty"`
	Data      interface{}   `json:"data"`
}

type CompletionRequest struct {
	Messages    []openai.ChatCompletionMessageParamUnion `json:"messages"`
	Tools       []openai.ChatCompletionToolParam         `json:"tools,omitempty"`
	Temperature *float64                                 `json:"temperature,omitempty"`
	MaxTokens   *int                                     `json:"max_tokens,omitempty"`
}

type EmbeddingRequest struct {
	Inputs []string `json:"inputs"`
}

type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type CompletionResponse struct {
	Message openai.ChatCompletionMessage `json:"message"`
}

type EmbeddingResponse struct {
	Embeddings [][]float64 `json:"embeddings"`
}

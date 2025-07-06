package coreml

import (
	"encoding/json"
	"fmt"

	"github.com/openai/openai-go"
)

type MockSuccessfulProcess struct {
	*MockBinaryProcess
}

func NewMockSuccessfulProcess() *MockSuccessfulProcess {
	return &MockSuccessfulProcess{
		MockBinaryProcess: NewMockBinaryProcess([]string{}),
	}
}

func (m *MockSuccessfulProcess) ReadLine() (string, error) {
	if !m.running {
		return "", fmt.Errorf("process not running")
	}

	if len(m.writeData) == 0 {
		return "", fmt.Errorf("no request written")
	}

	lastRequest := m.writeData[len(m.writeData)-1]

	var req Request
	if err := json.Unmarshal(lastRequest[:len(lastRequest)-1], &req); err != nil {
		return "", fmt.Errorf("failed to unmarshal request: %w", err)
	}

	var resp Response
	switch req.Operation {
	case OperationCompletion:
		resp = Response{
			Success: true,
			Data: CompletionResponse{
				Message: openai.ChatCompletionMessage{
					Content: "Mock completion response",
					Role:    "assistant",
				},
			},
		}
	case OperationEmbedding:
		embeddingReq := req.Data.(map[string]interface{})
		inputs := embeddingReq["inputs"].([]interface{})
		embeddings := make([][]float64, len(inputs))
		for i := range inputs {
			embeddings[i] = []float64{0.1, 0.2, 0.3, 0.4, 0.5}
		}
		resp = Response{
			Success: true,
			Data: EmbeddingResponse{
				Embeddings: embeddings,
			},
		}
	default:
		resp = Response{
			Success: false,
			Error:   "unknown operation",
		}
	}

	respJSON, err := json.Marshal(resp)
	if err != nil {
		return "", fmt.Errorf("failed to marshal response: %w", err)
	}

	return string(respJSON), nil
}

type MockFailingProcess struct {
	*MockBinaryProcess
	errorMessage string
}

func NewMockFailingProcess(errorMessage string) *MockFailingProcess {
	return &MockFailingProcess{
		MockBinaryProcess: NewMockBinaryProcess([]string{}),
		errorMessage:      errorMessage,
	}
}

func (m *MockFailingProcess) ReadLine() (string, error) {
	if !m.running {
		return "", fmt.Errorf("process not running")
	}

	resp := Response{
		Success: false,
		Error:   m.errorMessage,
	}

	respJSON, err := json.Marshal(resp)
	if err != nil {
		return "", fmt.Errorf("failed to marshal response: %w", err)
	}

	return string(respJSON), nil
}

type MockUnresponsiveProcess struct {
	*MockBinaryProcess
}

func NewMockUnresponsiveProcess() *MockUnresponsiveProcess {
	return &MockUnresponsiveProcess{
		MockBinaryProcess: NewMockBinaryProcess([]string{}),
	}
}

func (m *MockUnresponsiveProcess) ReadLine() (string, error) {
	return "", fmt.Errorf("process is unresponsive")
}

func (m *MockUnresponsiveProcess) IsRunning() bool {
	return false
}
